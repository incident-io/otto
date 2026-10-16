package otto

import (
	"testing"
)

// TestCompletionValue checks a script's completion value follows ES2015's
// UpdateEmpty rules rather than ES5's.
func TestCompletionValue(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		for _, tc := range []struct {
			src  string
			want interface{}
		}{
			{`1; {}`, 1},
			{`1; ;`, 1},
			{`1; var a = 2;`, 1},
			{`1; function f() {}`, 1},
			{`1; if (true) {}`, "undefined"},
			{`eval("1; if (false) 2")`, "undefined"},
			{`1; while (false) {}`, "undefined"},
			{`1; for (var k in null) {}`, "undefined"},
			{`for (var k in {a: 1}) { 8 }`, 8},
			{`for (var i = 0; i < 2; i++) { if (i) break; 7 }`, "undefined"},
			{`do { 4; continue; } while (false)`, 4},
			{`x: for (var i of [1, 2]) { for (;;) { 9; continue x; } }`, 9},
			{`B: { 1; break B; }`, 1},
			{`L: if (true) { 3; break L; }`, 3},
			{`1; switch (1) {}`, "undefined"},
			{`1; switch (1) { case 1: }`, "undefined"},
			{`switch (2) { case 1: 1; case 2: 2; break; case 3: 3 }`, 2},
			{`1; try {} finally { 2 }`, "undefined"},
			{`try { 5 } catch (e) {}`, 5},
			{`try { throw 1 } catch (e) {}`, "undefined"},
			{`1; with ({}) {}`, "undefined"},
			{`(function () { 1; })()`, "undefined"},
		} {
			test(tc.src, tc.want)
		}
	})
}
