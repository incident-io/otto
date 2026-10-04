package otto

import (
	"io"
	"regexp"
	"sync/atomic"
	"unicode/utf8"
)

// regExpDirectWork bounds the matching work, in estimated instructions times
// input bytes, of a search run as a single uninterruptible call into package
// regexp. Larger searches read their input through a regExpReader instead.
const regExpDirectWork = 1 << 20

// regExpCheckWork is roughly how much matching work a regExpReader allows
// between interrupt checks.
const regExpCheckWork = 1 << 14

// regExpProgram is a compiled regular expression and an estimate of its size.
type regExpProgram struct {
	re *regexp.Regexp

	// after is re preceded by (?s:.), compiled on first use. Searching from
	// the start of the rune before an offset with it finds matches of re at
	// or after that offset, with the rune before as context for ^, \b and \B.
	// Clones of a runtime share programs, so it is set atomically.
	after atomic.Pointer[regexp.Regexp]

	size int64
}

// regExpFind returns the submatch indices in s of the leftmost match of p
// that starts at or after byte offset pos, or nil if there is none.
func (rt *runtime) regExpFind(p *regExpProgram, s string, pos int) []int {
	if pos < 0 || pos > len(s) {
		return nil
	}
	re, start := p.re, pos
	if pos > 0 {
		_, w := utf8.DecodeLastRuneInString(s[:pos])
		// If pos is inside a rune there is no rune to use as context.
		if _, fw := utf8.DecodeRuneInString(s[pos-w:]); fw == w {
			if after := rt.regExpAfter(p); after != nil {
				re, start = after, pos-w
			}
		}
	}

	var m []int
	if int64(len(s)-start+1)*max(p.size, 1) <= regExpDirectWork {
		m = re.FindStringSubmatchIndex(s[start:])
	} else {
		m = re.FindReaderSubmatchIndex(&regExpReader{rt: rt, s: s, pos: start, cost: max(p.size, 1)})
	}
	if m == nil {
		return nil
	}
	for i := range m {
		if m[i] >= 0 {
			m[i] += start
		}
	}
	if re != p.re {
		_, w := utf8.DecodeRuneInString(s[m[0]:])
		m[0] += w
	}
	return m
}

// regExpAfter returns p.after, compiling it if need be, or nil if it does not
// compile.
func (rt *runtime) regExpAfter(p *regExpProgram) *regexp.Regexp {
	if after := p.after.Load(); after != nil {
		return after
	}
	rt.allocate(p.size * 40)
	after, err := regexp.Compile(`(?s:.)(?:` + p.re.String() + `)`)
	if err != nil {
		return nil
	}
	p.after.CompareAndSwap(nil, after)
	return p.after.Load()
}

// regExpFindAll returns the submatch indices of successive non-overlapping
// matches of p in s, as regexp's FindAllStringSubmatchIndex does, stopping
// after n matches if n >= 0. Each match is charged to the allocation limit.
func (rt *runtime) regExpFindAll(p *regExpProgram, s string, n int) [][]int {
	if n < 0 {
		n = len(s) + 1
	}
	var all [][]int
	for pos, prevEnd := 0, -1; len(all) < n && pos <= len(s); {
		rt.checkInterrupt()
		m := rt.regExpFind(p, s, pos)
		if m == nil {
			break
		}
		accept := true
		if m[1] == pos {
			// An empty match directly after the previous match is skipped.
			if m[0] == prevEnd {
				accept = false
			}
			if _, w := utf8.DecodeRuneInString(s[pos:]); w > 0 {
				pos += w
			} else {
				pos = len(s) + 1
			}
		} else {
			pos = m[1]
		}
		prevEnd = m[1]
		if accept {
			rt.allocateElement(int64(len(all)), int64(len(m))*8+allocValueCost)
			all = append(all, m)
		}
	}
	return all
}

// regExpReader reads a string from pos for package regexp, checking for
// interrupts as it goes. cost is the matching work charged per rune.
type regExpReader struct {
	rt   *runtime
	s    string
	pos  int
	cost int64
	work int64
}

func (r *regExpReader) ReadRune() (rune, int, error) {
	r.work += r.cost
	if r.work >= regExpCheckWork {
		r.work = 0
		r.rt.checkInterrupt()
	}
	if r.pos >= len(r.s) {
		return 0, 0, io.EOF
	}
	c, w := utf8.DecodeRuneInString(r.s[r.pos:])
	r.pos += w
	return c, w, nil
}
