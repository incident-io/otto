package otto

import (
	"testing"
)

func TestArrowFunction(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Single identifier parameter, concise body.
		test(`
            var double = x => x * 2;
            double(21);
        `, 42)

		// Empty parameter list.
		test(`
            var answer = () => 42;
            answer();
        `, 42)

		// Multiple parameters.
		test(`
            var add = (a, b) => a + b;
            add(3, 4);
        `, 7)

		// Parenthesised single parameter.
		test(`
            var inc = (a) => a + 1;
            inc(10);
        `, 11)

		// Block body with an explicit return and local var.
		test(`
            var f = (a) => { var r = a + 1; return r; };
            f(10);
        `, 11)

		// A block body with no return yields undefined.
		test(`
            var f = () => { 1 + 1; };
            f();
        `, "undefined")

		// typeof an arrow function is "function".
		test(`typeof (() => 1)`, "function")

		// length reflects the parameter count.
		test(`((a, b, c) => 0).length`, 3)
	})
}

func TestArrowFunction_higherOrder(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            [1, 2, 3].map(x => x * x).join(",");
        `, "1,4,9")

		test(`
            [1, 2, 3, 4].filter(x => x % 2 === 0).join(",");
        `, "2,4")

		test(`
            [1, 2, 3, 4].reduce((acc, x) => acc + x, 0);
        `, 10)

		// Currying via nested arrows.
		test(`
            var add = a => b => a + b;
            add(2)(3);
        `, 5)
	})
}

func TestArrowFunction_lexicalThis(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// `this` inside an arrow is captured from the enclosing method,
		// not rebound by the callback invocation.
		test(`
            var obj = {
                value: 5,
                run: function() {
                    return [1].map(() => this.value)[0];
                },
            };
            obj.run();
        `, 5)

		// Several layers deep, `this` is still the method's receiver.
		test(`
            var obj = {
                values: [1, 2, 3],
                factor: 10,
                scaled: function() {
                    return this.values.map(v => v * this.factor).join(",");
                },
            };
            obj.scaled();
        `, "10,20,30")
	})
}

func TestArrowFunction_arguments(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// An arrow function does not bind its own `arguments`; it sees the
		// enclosing function's `arguments`.
		test(`
            function outer() {
                var inner = () => arguments[0];
                return inner();
            }
            outer("hello");
        `, "hello")
	})
}
