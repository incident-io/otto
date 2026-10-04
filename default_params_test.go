package otto

import (
	"testing"
)

func TestDefaultParameters(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// A default is used when the argument is omitted.
		test(`
            function f(a, b = 2) { return a + b; }
            f(1);
        `, 3)

		// An explicitly passed value overrides the default.
		test(`
            function f(a, b = 2) { return a + b; }
            f(1, 10);
        `, 11)

		// Passing undefined triggers the default; other falsy values do not.
		test(`
            function f(a, b = 2) { return a + b; }
            [f(1, undefined), f(1, 0)].join(",");
        `, "3,1")

		// Defaults may reference earlier parameters.
		test(`
            function f(a = 1, b = a + 1) { return a + "/" + b; }
            f();
        `, "1/2")

		test(`
            function f(a = 1, b = a * 2) { return b; }
            f(5);
        `, 10)

		// A default in the middle of the list.
		test(`
            function f(a, b = 2, c = 3) { return a + b + c; }
            f(1, undefined, 10);
        `, 13)

		// Method shorthand with a default.
		test(`
            var o = { m(x = 7) { return x; } };
            o.m();
        `, 7)
	})
}
