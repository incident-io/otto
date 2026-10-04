package otto

import (
	"testing"
)

func TestObjectEntries(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// [key, value] pairs in Object.keys order.
		test(`
            var o = { a: 1, b: 2, c: 3 };
            JSON.stringify(Object.entries(o));
        `, `[["a",1],["b",2],["c",3]]`)

		// Only own enumerable string-keyed properties (not inherited).
		test(`
            var proto = { inherited: 99 };
            var o = Object.create(proto);
            o.own = 1;
            JSON.stringify(Object.entries(o));
        `, `[["own",1]]`)

		// Non-enumerable own properties are excluded.
		test(`
            var o = {};
            Object.defineProperty(o, "hidden", { value: 1, enumerable: false });
            o.shown = 2;
            JSON.stringify(Object.entries(o));
        `, `[["shown",2]]`)

		test(`Object.entries.length`, 1)
	})
}

func TestObjectIs(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// NaN is the same as NaN under SameValue (unlike ===).
		test(`Object.is(NaN, NaN)`, true)
		test(`NaN === NaN`, false)

		// +0 and -0 differ under SameValue (unlike ===).
		test(`Object.is(0, -0)`, false)
		test(`Object.is(-0, -0)`, true)
		test(`0 === -0`, true)

		test(`Object.is(1, 1)`, true)
		test(`Object.is("foo", "foo")`, true)
		test(`Object.is("foo", "bar")`, false)
		test(`Object.is(null, null)`, true)
		test(`Object.is(undefined, undefined)`, true)

		test(`var o = {}; Object.is(o, o)`, true)
		test(`Object.is({}, {})`, false)

		test(`Object.is.length`, 2)
	})
}

func TestObjectFromEntries(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            var o = Object.fromEntries([['a', 1], ['b', 2]]);
            [o.a, o.b].join(",");
        `, "1,2")

		// Later duplicate keys win.
		test(`
            var o = Object.fromEntries([['x', 1], ['x', 9]]);
            o.x;
        `, 9)

		// Round-trips with Object.entries.
		test(`
            var src = { a: 1, b: 2 };
            JSON.stringify(Object.fromEntries(Object.entries(src)));
        `, `{"a":1,"b":2}`)

		test(`Object.fromEntries.length`, 1)
	})
}

func TestObjectSetPrototypeOf(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Changing the prototype changes inheritance.
		test(`
            var proto = { greet: function() { return "hi"; } };
            var o = {};
            var result = Object.setPrototypeOf(o, proto);
            [o.greet(), result === o].join(",");
        `, "hi,true")

		// Setting prototype to null removes inheritance.
		test(`
            var o = { foo: 1 };
            Object.setPrototypeOf(o, null);
            Object.getPrototypeOf(o);
        `, "null")

		test(`Object.setPrototypeOf.length`, 2)
	})
}

func TestObjectGetOwnPropertyDescriptors(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            var o = { a: 1 };
            var d = Object.getOwnPropertyDescriptors(o);
            [ d.a.value, d.a.writable, d.a.enumerable, d.a.configurable ].join(",");
        `, "1,true,true,true")

		// Non-enumerable own properties are still included.
		test(`
            var o = {};
            Object.defineProperty(o, "hidden", { value: 7, enumerable: false });
            var d = Object.getOwnPropertyDescriptors(o);
            [ d.hidden.value, d.hidden.enumerable ].join(",");
        `, "7,false")

		test(`Object.getOwnPropertyDescriptors.length`, 1)
	})
}

func TestNumberIsInteger(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`Number.isInteger(5)`, true)
		test(`Number.isInteger(5.0)`, true)
		test(`Number.isInteger(-100)`, true)
		test(`Number.isInteger(5.5)`, false)
		test(`Number.isInteger(NaN)`, false)
		test(`Number.isInteger(Infinity)`, false)
		test(`Number.isInteger(-Infinity)`, false)

		// No coercion: non-numbers are always false.
		test(`Number.isInteger("5")`, false)
		test(`Number.isInteger(true)`, false)
		test(`Number.isInteger(null)`, false)
		test(`Number.isInteger(undefined)`, false)
		test(`Number.isInteger()`, false)

		test(`Number.isInteger.length`, 1)
	})
}

func TestNumberIsFinite(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`Number.isFinite(5)`, true)
		test(`Number.isFinite(0)`, true)
		test(`Number.isFinite(NaN)`, false)
		test(`Number.isFinite(Infinity)`, false)
		test(`Number.isFinite(-Infinity)`, false)

		// No coercion (unlike global isFinite).
		test(`Number.isFinite("5")`, false)
		test(`isFinite("5")`, true)
		test(`Number.isFinite(null)`, false)
		test(`isFinite(null)`, true)
		test(`Number.isFinite(undefined)`, false)
		test(`Number.isFinite()`, false)

		test(`Number.isFinite.length`, 1)
	})
}

func TestNumberIsSafeInteger(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`Number.isSafeInteger(5)`, true)
		test(`Number.isSafeInteger(Math.pow(2, 53) - 1)`, true)
		test(`Number.isSafeInteger(Math.pow(2, 53))`, false)
		test(`Number.isSafeInteger(-(Math.pow(2, 53) - 1))`, true)
		test(`Number.isSafeInteger(5.5)`, false)
		test(`Number.isSafeInteger(NaN)`, false)
		test(`Number.isSafeInteger(Infinity)`, false)

		// No coercion.
		test(`Number.isSafeInteger("5")`, false)
		test(`Number.isSafeInteger(null)`, false)
		test(`Number.isSafeInteger()`, false)

		test(`Number.isSafeInteger.length`, 1)
	})
}

func TestNumberParseIntFloat(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Behavioural parity with the global parseInt/parseFloat. They share the
		// same underlying builtin but are distinct function objects, so identity
		// (===) does not hold.
		test(`Number.parseInt("42") === parseInt("42")`, true)
		test(`Number.parseFloat("3.14") === parseFloat("3.14")`, true)

		test(`Number.parseInt("42")`, 42)
		test(`Number.parseInt("0xff", 16)`, 255)
		test(`Number.parseInt("10", 2)`, 2)
		test(`Number.parseInt("  -7  ")`, -7)
		test(`isNaN(Number.parseInt("xyz"))`, true)

		test(`Number.parseFloat("3.14")`, 3.14)
		test(`Number.parseFloat("  2.5e3")`, 2500)
		test(`isNaN(Number.parseFloat("abc"))`, true)

		test(`Number.parseInt.length`, 2)
		test(`Number.parseFloat.length`, 1)
	})
}
