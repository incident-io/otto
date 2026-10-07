package otto

import (
	"math"
	"regexp/syntax"
	"strings"

	"github.com/robertkrimen/otto/parser"
)

// maxNativeDepth bounds the Go recursion of the evaluator, independently of
// SetStackDepthLimit. Every nested expression, statement and function call
// counts towards it, so that deeply nested source, long prototype chains or
// unbounded JavaScript recursion throw a RangeError instead of exhausting the
// Go stack, which would crash the process.
const maxNativeDepth = 10000

// resourceLimitPanic is raised when a VM exceeds a hard resource limit such as
// SetAllocationLimit. Like an interrupt, it cannot be caught by JavaScript
// try/catch; catchPanic turns it into the returned error.
type resourceLimitPanic struct {
	rt  *runtime
	err ottoError
}

// enterNative records one level of Go recursion on behalf of the evaluator.
// Callers must pair it with leaveNative on the non-panicking path; on a panic,
// the depth is restored by whichever of tryCatchEvaluate or catchPanic
// recovers it.
func (rt *runtime) enterNative() {
	rt.nativeDepth++
	if rt.nativeDepth > maxNativeDepth {
		rt.nativeDepth--
		panic(rt.panicRangeError("Maximum call stack size exceeded"))
	}
}

func (rt *runtime) leaveNative() {
	rt.nativeDepth--
}

// Approximate costs, in bytes, used to account for allocations made on behalf
// of JavaScript code.
const (
	allocObjectCost   = 192
	allocPropertyCost = 64
	allocValueCost    = 24
)

// allocate charges bytes against the VM's allocation limit, if any. It is
// called before memory proportional to JavaScript-controlled sizes is
// allocated.
func (rt *runtime) allocate(bytes int64) {
	if rt.allocLimit <= 0 {
		return
	}
	if bytes < 0 || bytes > math.MaxInt64-rt.allocated {
		rt.allocated = math.MaxInt64
	} else {
		rt.allocated += bytes
	}
	if rt.allocated > rt.allocLimit {
		rt.unwinding = true
		panic(&resourceLimitPanic{rt: rt, err: newError(rt, "RangeError", 0, "Allocation limit exceeded")})
	}
}

// allocateN charges count items of size bytes each.
func (rt *runtime) allocateN(count, size int64) {
	if rt.allocLimit <= 0 {
		return
	}
	if count < 0 || (size > 0 && count > math.MaxInt64/size) {
		rt.allocate(-1)
		return
	}
	rt.allocate(count * size)
}

// maxDenseBytes bounds any single dense allocation, such as a Go slice or
// argument list, whose size is controlled by JavaScript, whether or not an
// allocation limit is set.
const maxDenseBytes = 1 << 28

// allocateDense validates and charges a dense allocation of count items of
// size bytes each, throwing a RangeError if count is negative or the
// allocation would exceed maxDenseBytes.
func (rt *runtime) allocateDense(count, size int64) {
	if count < 0 || count > maxDenseBytes/max(size, 1) {
		panic(rt.panicRangeError("Invalid array length"))
	}
	rt.allocateN(count, size)
}

// allocateElement validates and charges the growth of a dense allocation,
// such as an argument list, to index+1 items of size bytes each, throwing a
// RangeError once it would exceed maxDenseBytes.
func (rt *runtime) allocateElement(index, size int64) {
	if index >= maxDenseBytes/max(size, 1) {
		panic(rt.panicRangeError("Invalid array length"))
	}
	rt.allocate(size)
}

// allocateString checks a string of length bytes against the string length
// limit and charges it against the allocation limit.
func (rt *runtime) allocateString(length int) {
	rt.checkStringLength(length)
	rt.allocate(int64(length))
}

// interruptEvery is how many iterations of a cheap native loop may run between
// checks for Interrupt.
const interruptEvery = 1024

// pollInterrupt calls checkInterrupt every interruptEvery calls, for loops whose
// iterations are too cheap to check on every iteration.
func (rt *runtime) pollInterrupt(i int) {
	if i%interruptEvery == 0 {
		rt.checkInterrupt()
	}
}

// parserOptions returns the options with which to parse source on behalf of
// this runtime.
func (rt *runtime) parserOptions() []parser.Option {
	if rt.otto == nil || rt.otto.Interrupt == nil {
		return nil
	}
	return []parser.Option{parser.WithInterrupt(rt.checkInterrupt)}
}

// mapString is strings.Map, but polls for interrupts and enforces the string
// and allocation limits as the result grows.
func (rt *runtime) mapString(s string, mapping func(rune) rune) string {
	rt.allocateString(len(s))
	var b strings.Builder
	b.Grow(len(s))
	n := 0
	for _, r := range s {
		n++
		if n%interruptEvery == 0 {
			rt.checkInterrupt()
			rt.checkStringLength(b.Len())
		}
		r = mapping(r)
		if r >= 0 {
			b.WriteRune(r)
		}
	}
	if extra := b.Len() - len(s); extra > 0 {
		rt.checkStringLength(b.Len())
		rt.allocate(int64(extra))
	}
	return b.String()
}

// maxRegExpSize bounds the estimated number of instructions in a compiled
// regular expression, which bounds both the memory used to compile it and the
// work done per character when matching it.
const maxRegExpSize = 1 << 16

// maxRegExpPrograms is how many compiled patterns a runtime keeps for reuse.
const maxRegExpPrograms = 64

// checkRegExpSize throws a SyntaxError if pattern, in Go syntax, would
// compile to more than maxRegExpSize instructions, and otherwise returns the
// estimated number of instructions and whether the pattern is context-free
// (see regExpContextFree).
func (rt *runtime) checkRegExpSize(pattern string) (int64, bool) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		// Let Compile report the error.
		return 0, false
	}
	size := regExpSize(re)
	if size > maxRegExpSize {
		panic(rt.panicSyntaxError("Invalid regular expression: regular expression too large"))
	}
	rt.allocate(size * 40)
	return size, regExpContextFree(re)
}

// regExpContextFree reports whether re has no assertions that look at the
// text before the position they are tested at.
func regExpContextFree(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpBeginLine, syntax.OpBeginText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return false
	}
	for _, sub := range re.Sub {
		if !regExpContextFree(sub) {
			return false
		}
	}
	return true
}

// regExpSize estimates the number of instructions re compiles to, saturating
// at maxRegExpSize+1.
func regExpSize(re *syntax.Regexp) int64 {
	var size int64 = 1
	for _, sub := range re.Sub {
		size += regExpSize(sub)
	}
	if re.Op == syntax.OpRepeat {
		n := int64(re.Max)
		if n < 0 {
			n = int64(re.Min) + 1
		}
		size *= max(n, 1)
	}
	size += int64(len(re.Rune))
	return min(size, maxRegExpSize+1)
}
