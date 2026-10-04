package otto

import (
	"testing"
)

func TestObjectParameterDestructuring(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            function f({a, b}) { return a + b; }
            f({a: 1, b: 2});
        `, 3)

		// Default within an object pattern parameter.
		test(`
            function f({a, b = 10}) { return a + b; }
            f({a: 5});
        `, 15)

		// Nested object pattern.
		test(`
            function f({a: {b}}) { return b; }
            f({a: {b: 42}});
        `, 42)

		// A defaulted object-pattern parameter.
		test(`
            function f({a = 1, b = 2} = {}) { return a + "/" + b; }
            f();
        `, "1/2")

		// Mixed plain and pattern parameters.
		test(`
            function f(x, {y, z}) { return x + y + z; }
            f(1, {y: 2, z: 3});
        `, 6)
	})
}

func TestArrayParameterDestructuring(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            function f([x, y]) { return x * y; }
            f([3, 4]);
        `, 12)

		// Rest within an array pattern parameter.
		test(`
            function f([a, ...rest]) { return a + "/" + rest.join(","); }
            f([1, 2, 3]);
        `, "1/2,3")
	})
}

func TestArrowParameterDestructuring(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`var g = ({x, y}) => x + y; g({x: 7, y: 8});`, 15)
		test(`var h = ([a, b]) => a + "-" + b; h(["p", "q"]);`, "p-q")

		// Cover grammar: defaulted object pattern in an arrow parameter.
		test(`var k = ({n = 3} = {}) => n; [k(), k({n: 9})].join(",");`, "3,9")

		// A destructuring callback, the common real-world use.
		test(`
            [{a: 1, b: 2}, {a: 3, b: 4}].map(({a, b}) => a + b).join(",");
        `, "3,7")
	})
}

// TestArrowNestedPatternDefaults covers arrow parameter lists whose patterns
// nest a defaulted array or object pattern, e.g. ([{x} = {}]) => ... The inner
// "{x} = {}" parses as a destructuring assignment, so its left side is already
// a pattern when the cover grammar reinterprets the outer literal.
func TestArrowNestedPatternDefaults(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Defaulted object pattern nested inside an array pattern parameter.
		test(`var f = ([{ x, y } = { x: 1, y: 2 }]) => x + "," + y; f([]);`, "1,2")
		test(`var f = ([{ x, y } = { x: 1, y: 2 }]) => x + "," + y; f([{x: 7, y: 8}]);`, "7,8")

		// Defaulted array pattern nested inside an array pattern parameter.
		test(`var f = ([[a, b] = [4, 5]]) => a + "," + b; f([]);`, "4,5")

		// Defaulted array pattern nested inside a defaulted object pattern.
		test(`var f = ({ w: [a, b] = [4, 5] } = { w: [7] }) => a + "," + b; f();`, "7,undefined")

		// A rest element binding a nested pattern (no default) is still valid.
		test(`var f = ([a, ...[b]]) => a + "," + b; f([1, 2]);`, "1,2")
	})
}

// TestArrowInvalidPatternRest checks that genuinely invalid arrow parameter
// lists are still rejected: a rest element may not carry a default initializer.
func TestArrowInvalidPatternRest(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`raise:
            eval("([...[x] = []]) => x;");
        `, "SyntaxError: (anonymous): Line 1:2 malformed arrow function parameter list (and 1 more errors)")

		test(`raise:
            eval("([...x = 1]) => x;");
        `, "SyntaxError: (anonymous): Line 1:2 malformed arrow function parameter list (and 1 more errors)")

		// A rest element must be the final element: nothing may follow it.
		test(`raise:
            eval("([...x, y]) => x;");
        `, "SyntaxError: (anonymous): Line 1:2 malformed arrow function parameter list (and 1 more errors)")
	})
}
