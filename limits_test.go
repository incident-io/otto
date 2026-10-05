package otto

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// runWithDeadline runs script with an Interrupt that fires after timeout, and
// fails the test if the VM does not return within a second of the interrupt.
// It returns the error from Run, or errHalted if the interrupt stopped it.
func runWithDeadline(t *testing.T, vm *Otto, script string, timeout time.Duration) error {
	t.Helper()
	halt := errors.New("halt")
	vm.Interrupt = make(chan func(), 1)
	done := make(chan error, 1)
	go func() {
		defer func() {
			if caught := recover(); caught != nil {
				if caught == halt { //nolint:errorlint
					done <- errHalted
					return
				}
				panic(caught)
			}
		}()
		_, err := vm.Run(script)
		done <- err
	}()
	timer := time.AfterFunc(timeout, func() { vm.Interrupt <- func() { panic(halt) } })
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout + time.Second):
		t.Fatalf("script ran more than a second past its deadline: %.100s", script)
		return nil
	}
}

var errHalted = errors.New("halted")

func TestTemplateNesting(t *testing.T) {
	t.Parallel()
	vm := New()
	v, err := vm.Run("`a${`b${`c${1}`}`}`")
	require.NoError(t, err)
	require.Equal(t, "abc1", v.String())

	nest := func(n int) string { return strings.Repeat("`${", n) + "1" + strings.Repeat("}`", n) }
	v, err = vm.Run(nest(32))
	require.NoError(t, err)
	require.Equal(t, "1", v.String())
	_, err = vm.Run(nest(33))
	require.ErrorContains(t, err, "Maximum nesting depth exceeded")

	start := time.Now()
	n := 100000
	_, err = vm.Run(nest(n))
	require.ErrorContains(t, err, "Maximum nesting depth exceeded")
	require.Less(t, len(err.Error()), 1000)
	require.Less(t, time.Since(start), 3*time.Second)
}

func TestParseInterrupt(t *testing.T) {
	t.Parallel()
	script := strings.Repeat("1,", 1<<22) + "1"
	run := map[string]func(vm *Otto){
		"run":     func(vm *Otto) { _, _ = vm.Run(script) },
		"compile": func(vm *Otto) { _, _ = vm.Compile("", script) },
	}
	for name, fn := range run {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vm := New()
			vm.Interrupt = make(chan func(), 1)
			vm.Interrupt <- func() { panic(errHalted) }
			require.PanicsWithValue(t, errHalted, func() { fn(vm) })
		})
	}
}

func TestGoSliceLength(t *testing.T) {
	for _, length := range []string{"-1", "NaN", "Infinity", "1.5", "4e9", "4294967295"} {
		t.Run(length, func(t *testing.T) {
			vm := New()
			require.NoError(t, vm.Set("s", []int{1, 2, 3}))
			_, err := vm.Run("s.length = " + length)
			require.EqualError(t, err, "RangeError: Invalid array length")
		})
	}

	vm := New()
	require.NoError(t, vm.Set("s", []int{1, 2, 3}))
	v, err := vm.Run("s.length = 1; s.length = 4; s.join()")
	require.NoError(t, err)
	require.Equal(t, "1,0,0,0", v.String())
}

func TestPrototypeCycle(t *testing.T) {
	vm := New()
	v, err := vm.Run(`
		var a = {}, b = Object.create(a);
		try { Object.setPrototypeOf(a, b); "no error" } catch (e) { e.name }
	`)
	require.NoError(t, err)
	require.Equal(t, "TypeError", v.String())
}

func TestParseFloatPrefix(t *testing.T) {
	tt(t, func() {
		test, _ := test()
		test(`String(parseFloat("1e999"))`, "Infinity")
		test(`String(parseFloat("-1e999"))`, "-Infinity")
		test(`parseFloat("1e-999")`, 0)
		test(`parseFloat("1.")`, 1)
		test(`parseFloat("-.5")`, -0.5)
		test(`parseFloat("5.e3")`, 5000)
		test(`parseFloat("  12abc")`, 12)
		test(`parseFloat("1e")`, 1)
		test(`parseFloat("1e+")`, 1)
		test(`String(parseFloat("Infinityx"))`, "Infinity")
		test(`String(parseFloat("-Infinity"))`, "-Infinity")
		test(`String(parseFloat(".e1"))`, "NaN")
		test(`String(parseFloat("abc"))`, "NaN")
		test(`String(parseFloat(""))`, "NaN")
	})
}

func TestStackLimit(t *testing.T) {
	scripts := map[string]string{
		"nested-source":    `eval("(".repeat(100000) + "1" + ")".repeat(100000))`,
		"nested-arrays":    `eval("[".repeat(100000) + "]".repeat(100000))`,
		"nested-functions": `eval("function f(){".repeat(100000) + "}".repeat(100000))`,
		"nested-regexp":    `new RegExp("(".repeat(100000) + ")".repeat(100000))`,
		"recursion":        `function f() { f() } f()`,
		"json-stringify":   `var a = []; for (var i = 0; i < 100000; i++) a = [a]; JSON.stringify(a)`,
		"json-parse":       `JSON.parse("[".repeat(100000) + "]".repeat(100000))`,
		"bind-chain":       `var f = function() {}; for (var i = 0; i < 20000; i++) f = f.bind(null); f()`,
		"to-string":        `var o = {}; o.toString = function() { return String(o) }; String(o)`,
	}
	for name, script := range scripts {
		t.Run(name, func(t *testing.T) {
			err := runWithDeadline(t, New(), script, 10*time.Second)
			require.Error(t, err)
			require.NotErrorIs(t, err, errHalted)
		})
	}
}

func TestStringLengthLimitCoverage(t *testing.T) {
	scripts := map[string]string{
		"template":     "var s = 'x'.repeat(1 << 20); `${s}${s}`",
		"encode-uri":   "encodeURIComponent('\\u0800'.repeat(1 << 18))",
		"upper-case":   "'x'.repeat(1 << 20).toUpperCase() + 'x'.repeat(1 << 20)",
		"error-string": "var s = 'x'.repeat(1 << 20); var e = new Error(s); e.name = s; String(e)",
		"pad":          "''.padStart(4 << 20, 'x')",
		"repeat":       "'x'.repeat(4 << 20)",
	}
	for name, script := range scripts {
		t.Run(name, func(t *testing.T) {
			vm := New()
			vm.SetStringLengthLimit(1 << 20)
			_, err := vm.Run(script)
			require.ErrorContains(t, err, "RangeError")
		})
	}
}

func TestAllocationLimit(t *testing.T) {
	scripts := map[string]string{
		"objects": `var a = []; for (;;) a.push({x: 1})`,
		"strings": `var a = []; for (;;) a.push("x".repeat(1000) + a.length)`,
		"split":   `var a = []; for (;;) a.push("xxxxxxxx".repeat(1000).split(""))`,
		"json":    `var a = []; var s = JSON.stringify(new Array(1000).fill(1)); for (;;) a.push(JSON.parse(s))`,
		"catch":   `var a = []; for (;;) { try { a.push({}) } catch (e) {} }`,
	}
	for name, script := range scripts {
		t.Run(name, func(t *testing.T) {
			vm := New()
			vm.SetAllocationLimit(64 << 20)
			err := runWithDeadline(t, vm, script, 20*time.Second)
			require.EqualError(t, err, "RangeError: Allocation limit exceeded")
		})
	}

	vm := New()
	vm.SetAllocationLimit(64 << 20)
	v, err := vm.Run(`var a = []; for (var i = 0; i < 1000; i++) a.push({x: i}); a.length`)
	require.NoError(t, err)
	require.Equal(t, "1000", v.String())
}

func TestNativeInterrupt(t *testing.T) {
	t.Parallel()
	scripts := map[string]string{
		"parse-float":          `var s = "9".repeat(1 << 22); for (;;) parseFloat(s + "x")`,
		"split":                `var s = "x".repeat(1 << 24); for (;;) s.split("")`,
		"json-parse":           `var s = "[" + "1,".repeat(1 << 22) + "1]"; for (;;) JSON.parse(s)`,
		"regexp-backtrack":     `var s = "a".repeat(1 << 22); for (;;) /(a|aa)*(b|c|d)$/.test(s)`,
		"regexp-match-all":     `var s = "a".repeat(1 << 24); for (;;) s.match(/a/g)`,
		"regexp-replace":       `var s = "a".repeat(1 << 24); for (;;) s.replace(/a/g, "b")`,
		"regexp-quadratic":     `var s = "x".repeat(1 << 15); for (;;) s.replace(/x*y|x/g, "")`,
		"regexp-big-program":   `var s = "a".repeat(1 << 22); for (;;) /(?:a{1,800}b|a{1,800}c)$/.test(s)`,
		"regexp-search":        `var s = "a".repeat(1 << 22); for (;;) s.search(/(a|aa)*(b|c|d)$/)`,
		"regexp-split":         `var s = "a".repeat(1 << 22); for (;;) s.split(/(a|aa)*(b|c|d)/)`,
		"regexp-exec-offset":   `var s = "a".repeat(1 << 22), re = /(a|aa)*(b|c|d)$/g; for (;;) { re.lastIndex = 1; re.exec(s) }`,
		"apply":                `for (;;) Math.max.apply(null, {length: 4294967295})`,
		"encode-uri":           `var s = "\u0800".repeat(1 << 22); for (;;) encodeURIComponent(s)`,
		"escape":               `var s = "\u0800".repeat(1 << 22); for (;;) escape(s)`,
		"unescape":             `var s = "%u0800".repeat(1 << 22); for (;;) unescape(s)`,
		"date-parse":           `var s = "1".repeat(1 << 24); for (;;) Date.parse(s)`,
		"array-from-string":    `var s = "a".repeat(1 << 24); for (;;) Array.from(s)`,
		"string-index":         `var s = "\u0800".repeat(1 << 22); for (var n = 0;;) n += s[5].length`,
		"sort":                 `var a = []; for (var i = 0; i < 1e6; i++) a.push(1e6 - i); for (;;) a.slice().sort()`,
		"recursive-catch":      `function f() { try { f() } catch (e) { f() } } f()`,
		"eval-big-source":      `var s = "1+".repeat(1 << 22) + "1"; for (;;) eval(s)`,
		"eval-nested-template": "var s = '`${'.repeat(1 << 20) + '1' + '}`'.repeat(1 << 20); for (;;) eval(s)",
	}
	for name, script := range scripts {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			err := runWithDeadline(t, New(), script, 500*time.Millisecond)
			if err != nil && !errors.Is(err, errHalted) {
				// Hitting a hard limit is also fine, as long as it was quick.
				require.True(t, strings.Contains(err.Error(), "RangeError") || strings.Contains(err.Error(), "SyntaxError"), err.Error())
			}
			require.Less(t, time.Since(start), 1500*time.Millisecond)
		})
	}
}

func TestRegExpSizeLimit(t *testing.T) {
	vm := New()
	_, err := vm.Run(`new RegExp("(x{1000}){1000}")`)
	require.ErrorContains(t, err, "SyntaxError")
	_, err = vm.Run(`new RegExp("x".repeat(1 << 20))`)
	require.ErrorContains(t, err, "SyntaxError")
	v, err := vm.Run(`/^\w{1,100}@[a-z]{2,10}$/.test("someone@example")`)
	require.NoError(t, err)
	require.True(t, v.bool())
}

func TestRegExpSearchContext(t *testing.T) {
	tt(t, func() {
		test, _ := test()
		test(`"ab ab".replace(/\bb/g, "x")`, "ab ab")
		test(`"a\na".replace(/^a/gm, "x")`, "x\nx")
		test(`"aa".replace(/^a/g, "x")`, "xa")
		test(`"a b".match(/\B|\b/g).length`, 4)
		test(`var re = /^a/g; re.lastIndex = 1; re.exec("aa")`, "null")
		test(`var re = /\ba/g; re.lastIndex = 1; re.exec("aa a").index`, 3)
		test(`var re = /a/g; re.lastIndex = 2; re.exec("aéa").index`, 2)
		test(`"x".repeat(5000).replace(/x*y|x/g, "-").length`, 5000)
		test(`"\u00e9\u00e9".replace(/\b|/g, "-")`, "-é-é-")
	})
}

func TestJSONParse(t *testing.T) {
	tt(t, func() {
		test, _ := test()
		test(`JSON.stringify(JSON.parse(' {"a": [1, -2.5e3, true, false, null, "x\\u00e9\\ud83d\\ude00\\n"], "b": {}} '))`,
			`{"a":[1,-2500,true,false,null,"xé😀\n"],"b":{}}`)
		test(`JSON.parse('"\\ud800"').length`, 1)
		test(`JSON.parse('{"a":1,"a":2}').a`, 2)
		test(`JSON.parse("[1,2]", function(k, v) { return typeof v === "number" ? v * 2 : v }).join()`, "2,4")
	})

	for _, bad := range []string{``, `[1,]`, `{"a" 1}`, `01`, `1.`, `tru`, `"\x"`, `[1] x`, `"\ud800`} {
		vm := New()
		require.NoError(t, vm.Set("s", bad))
		_, err := vm.Run(`JSON.parse(s)`)
		require.ErrorContains(t, err, "SyntaxError", bad)
	}
}
