package otto

import (
	"testing"
)

func TestObjectShorthand(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Property shorthand: { x, y } is sugar for { x: x, y: y }.
		test(`
            var x = 1, y = 2;
            var o = { x, y };
            [o.x, o.y].join(",");
        `, "1,2")

		// Shorthand mixed with ordinary properties.
		test(`
            var a = 10;
            var o = { a, b: 20 };
            [o.a, o.b].join(",");
        `, "10,20")

		// Properties literally named "get" and "set".
		test(`
            var o = { get: 1, set: 2 };
            [o.get, o.set].join(",");
        `, "1,2")
	})
}

func TestObjectMethodShorthand(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            var o = { foo() { return 42; } };
            o.foo();
        `, 42)

		test(`
            var o = { add(a, b) { return a + b; } };
            o.add(3, 4);
        `, 7)

		// A method shorthand can use `this`.
		test(`
            var o = { value: 5, get() { return this.value; } };
            o.get();
        `, 5)
	})
}

func TestObjectComputedProperty(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            var k = "dyn";
            var o = { [k]: 9 };
            o.dyn;
        `, 9)

		test(`
            var o = { ["a" + "b"]: 5 };
            o.ab;
        `, 5)

		// Computed keys are evaluated in order.
		test(`
            var i = 0;
            var o = { [++i]: "a", [++i]: "b" };
            o[1] + o[2];
        `, "ab")

		// A computed method key.
		test(`
            var name = "run";
            var o = { [name]() { return "ok"; } };
            o.run();
        `, "ok")
	})
}

func TestObjectAccessorsStillWork(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Ensure getters/setters continue to parse alongside the new forms.
		test(`
            var store = 0;
            var o = {
                get x() { return store; },
                set x(v) { store = v; },
            };
            o.x = 11;
            o.x;
        `, 11)
	})
}
