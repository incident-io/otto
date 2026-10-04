package otto

import (
	"testing"
)

func TestRestParameters(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// All arguments collected.
		test(`
            function f(...args) { return args.length; }
            f(1, 2, 3);
        `, 3)

		// The rest parameter is a real array.
		test(`
            function f(...args) { return Array.isArray(args); }
            f(1);
        `, true)

		test(`
            function f(...args) { return args.join("-"); }
            f("a", "b");
        `, "a-b")

		// Rest after fixed parameters.
		test(`
            function f(a, b, ...rest) { return rest.join(","); }
            f(1, 2, 3, 4, 5);
        `, "3,4,5")

		// No trailing arguments yields an empty array.
		test(`
            function f(a, ...rest) { return rest.length; }
            f(1);
        `, 0)

		// Rest combined with a default parameter.
		test(`
            function f(a = 5, ...rest) { return a + "," + rest.length; }
            f(undefined, 1, 2);
        `, "5,2")

		// Spread-free higher-order use.
		test(`
            function sum(...ns) { return ns.reduce(function(a, b) { return a + b; }, 0); }
            sum(1, 2, 3, 4);
        `, 10)
	})
}
