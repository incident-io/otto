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
