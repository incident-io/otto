package otto

import (
	"testing"
)

func TestSpreadCall(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            function f(a, b, c) { return a + b + c; }
            f(...[1, 2, 3]);
        `, 6)

		// Spread mixed with fixed arguments.
		test(`
            function f(a, b, c, d) { return [a, b, c, d].join(","); }
            f(1, ...[2, 3], 4);
        `, "1,2,3,4")

		// Spread into a built-in.
		test(`Math.max(...[4, 1, 9, 2]);`, 9)

		// Spread feeding a rest parameter.
		test(`
            function f(x, ...r) { return x + ":" + r.join(","); }
            f(...[1, 2, 3, 4]);
        `, "1:2,3,4")
	})
}

func TestSpreadArrayLiteral(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            var a = [1, 2];
            [0, ...a, 3].join(",");
        `, "0,1,2,3")

		test(`
            var a = [1, 2], b = [3, 4];
            [...a, ...b].length;
        `, 4)

		// Spreading a string iterates its characters.
		test(`[..."abc"].join("-");`, "a-b-c")

		// Spreading an array-like object.
		test(`
            var o = { 0: "x", 1: "y", length: 2 };
            [...o].join(",");
        `, "x,y")
	})
}
