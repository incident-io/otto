package otto

import (
	"testing"
)

func TestForOf(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Iterating an array yields its values, not its indices.
		test(`
            var s = 0;
            for (var x of [1, 2, 3]) s += x;
            s;
        `, 6)

		// for (let ...) of.
		test(`
            var s = 0;
            for (let x of [10, 20]) s += x;
            s;
        `, 30)

		// Iterating a string yields its characters.
		test(`
            var r = "";
            for (const c of "abc") r += c + ".";
            r;
        `, "a.b.c.")

		// const loop variable.
		test(`
            var sum = 0;
            for (const n of [1, 2, 3, 4]) sum += n;
            sum;
        `, 10)
	})
}

func TestForOfDestructuring(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Array pattern declared with const.
		test(`
            var out = [];
            for (const [a, b] of [[1, 2], [3, 4]]) out.push(a + b);
            out.join(",");
        `, "3,7")

		// Object pattern declared with let.
		test(`
            var out = [];
            for (let {x, y} of [{x: 1, y: 2}, {x: 10, y: 20}]) out.push(x + y);
            out.join(",");
        `, "3,30")

		// Assignment array pattern (no declaration keyword) over existing names.
		test(`
            var a, b, out = [];
            for ([a, b] of [[1, 2], [3, 4]]) out.push(a * b);
            out.join(",");
        `, "2,12")

		// Nested patterns with const.
		test(`
            var out = [];
            for (const [a, [b]] of [[1, [2]], [3, [4]]]) out.push(a + b);
            out.join(",");
        `, "3,7")

		// Default values inside the pattern.
		test(`
            var out = [];
            for (const [a, b = 9] of [[1], [2, 3]]) out.push(a + b);
            out.join(",");
        `, "10,5")

		// Each iteration of for (const [..] of) gets its own binding.
		test(`
            var fns = [];
            for (const [x] of [[1], [2], [3]]) fns.push(function () { return x; });
            [fns[0](), fns[1](), fns[2]()].join(",");
        `, "1,2,3")
	})
}

func TestForInDestructuring(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Object pattern declared with let, destructuring each key string.
		test(`
            var out = [];
            for (let {length: n} in {ab: 1, cde: 1}) out.push(n);
            out.sort().join(",");
        `, "2,3")

		// Assignment object pattern (no declaration keyword).
		test(`
            var n, out = [];
            for ({length: n} in {ab: 1, cde: 1}) out.push(n);
            out.sort().join(",");
        `, "2,3")

		// Assignment array pattern over an array's index keys.
		test(`
            var k, out = [];
            for ([k] in ["x", "y"]) out.push(k);
            out.sort().join(",");
        `, "0,1")
	})
}

func TestForOfControlFlow(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            var out = [];
            for (let x of [1, 2, 3, 4]) {
                if (x === 2) continue;
                if (x === 4) break;
                out.push(x);
            }
            out.join(",");
        `, "1,3")

		test(`
            var n = 0;
            outer: for (var x of [1, 2, 3]) {
                n++;
                continue outer;
            }
            n;
        `, 3)

		test(`
            var out = [];
            outer: for (const x of [1, 2, 3]) {
                for (const y of [1, 2, 3]) {
                    if (y === 2) continue outer;
                    if (x === 3) break outer;
                    out.push(x + ":" + y);
                }
            }
            out.join(",");
        `, "1:1,2:1")
	})
}

func TestForOfPerIteration(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Each iteration of for (let ... of) has its own binding.
		test(`
            var fns = [];
            for (let x of [1, 2, 3]) fns.push(function() { return x; });
            [fns[0](), fns[1](), fns[2]()].join(",");
        `, "1,2,3")
	})
}

func TestForOfLabelledControlFlow(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		for _, tc := range []struct{ src, want string }{
			{`var n = 0; L: for (const [a] of [[1], [2], [3]]) { n += a; continue L; } String(n);`, "6"},
			{`var x, out = []; L: for (x of [1, 2, 3]) { if (x === 2) break L; out.push(x); } out.join(",") + ":" + x;`, "1:2"},
			{`var out = ""; L: for (const c of "abc") { if (c === "b") continue L; out += c; } out;`, "ac"},
			{`var out = []; L: M: for (const x of [1, 2, 3]) { if (x === 1) continue L; if (x === 3) continue M; out.push(x); } out.join(",");`, "2"},
			{`var out = []; L: for (const x of [1, 2, 3, 4]) { switch (x) { case 1: continue L; case 3: break L; default: out.push(x); } } out.join(",");`, "2"},
			{`var f = 0, out = []; L: for (const x of [1, 2, 3]) { try { if (x === 2) continue L; out.push(x); } finally { f++; } } out.join(",") + ":" + f;`, "1,3:3"},
			{`var out = []; L: for (const x of [1, 2, 3]) { B: { if (x === 2) break B; out.push(x); } out.push("-"); } out.join("");`, "1--3-"},
			{`function g() { L: for (const x of [1, 2, 3]) { for (const y of [1, 2]) { if (x === 2) return x + ":" + y; } } return "none"; } g();`, "2:1"},
			{`var out = []; L: for (const x of [1, 2, 3]) { var w = 0; while (w++ < 3) { if (w === 2) continue L; out.push(x + "" + w); } } out.join(",");`, "11,21,31"},
			{`var out = []; L: for (const x of [1, 2, 3]) { for (var k in {a: 1, b: 2}) { if (x === 2) break L; out.push(x + k); } } out.join(",");`, "1a,1b"},
			{`var fs = []; L: for (let x of [1, 2, 3]) { fs.push(function () { return x; }); if (x < 3) continue L; } fs.map(function (f) { return f(); }).join(",");`, "1,2,3"},
			{`let x = "outer"; L: for (let x of [1, 2]) { if (x === 1) continue L; break L; } x;`, "outer"},
		} {
			test(tc.src, tc.want)
		}
	})
}
