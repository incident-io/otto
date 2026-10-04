package otto

import (
	"testing"
)

func TestArrayDestructuring(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`let [a, b] = [1, 2]; a + "," + b;`, "1,2")

		// Elisions (holes).
		test(`let [a, , c] = [1, 2, 3]; a + "," + c;`, "1,3")

		// Rest element.
		test(`let [a, ...rest] = [1, 2, 3, 4]; a + "/" + rest.join(",");`, "1/2,3,4")

		// Defaults.
		test(`let [a = 10, b = 20] = [1]; a + "," + b;`, "1,20")

		// Nested.
		test(`let [[a], [b]] = [[1], [2]]; a + "," + b;`, "1,2")

		// Destructuring a string.
		test(`let [a, b] = "hi"; a + b;`, "hi")

		// From a function result.
		test(`
            function f() { return [1, 2, 3]; }
            let [x, y, z] = f();
            x + y + z;
        `, 6)

		// var destructuring.
		test(`var [a, b] = [8, 9]; a + b;`, 17)
	})
}

func TestObjectDestructuring(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`let {x, y} = {x: 1, y: 2}; x + "," + y;`, "1,2")

		// Renaming.
		test(`let {a: p, b: q} = {a: 5, b: 6}; p + "," + q;`, "5,6")

		// Defaults.
		test(`let {x = 7} = {}; x;`, 7)
		test(`let {a: x = 7} = {}; x;`, 7)

		// Nested.
		test(`let {a: {b}} = {a: {b: 42}}; b;`, 42)
		test(`let {a: {b: {c}}} = {a: {b: {c: "deep"}}}; c;`, "deep")

		// Computed key.
		test(`let k = "dyn"; let {[k]: v} = {dyn: 5}; v;`, 5)

		// Object rest.
		test(`let {a, ...rest} = {a: 1, b: 2, c: 3}; a + "/" + rest.b + "/" + rest.c;`, "1/2/3")
	})
}

func TestDestructuringAssignment(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Assignment (not declaration) destructuring.
		test(`var a, b; [a, b] = [3, 4]; a + "," + b;`, "3,4")

		// Assigning into a member.
		test(`
            var o = {};
            ({x: o.k} = {x: 99});
            o.k;
        `, 99)

		// Swap idiom.
		test(`var a = 1, b = 2; [a, b] = [b, a]; a + "," + b;`, "2,1")

		// A nested array pattern with a default, as an assignment target.
		test(`var x, y; ({ w: [x, y] = [4, 5] } = { w: [7] }); x + "," + y;`, "7,undefined")
		test(`var x, y; ({ w: [x, y] = [4, 5] } = {}); x + "," + y;`, "4,5")

		// A nested object pattern with a default inside an array target.
		test(`var x, y; [{ x, y } = { x: 1, y: 2 }] = []; x + "," + y;`, "1,2")

		// A rest element must be the final element of an array pattern: an
		// element or further rest may not follow it.
		test(`raise:
            eval("[...x, y] = [1, 2, 3];");
        `, "SyntaxError: (anonymous): Line 1:1 invalid destructuring assignment target")
		test(`raise:
            eval("[...x, ...y] = [1, 2, 3];");
        `, "SyntaxError: (anonymous): Line 1:1 invalid destructuring assignment target")

		// A trailing rest element remains valid.
		test(`var a, b; [a, ...b] = [1, 2, 3]; a + ":" + b.join(",");`, "1:2,3")
	})
}

func TestDestructuringForOf(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            var out = [];
            for (const [k, v] of [["a", 1], ["b", 2]]) out.push(k + "=" + v);
            out.join(",");
        `, "a=1,b=2")

		test(`
            var s = 0;
            for (let {x} of [{x: 1}, {x: 2}, {x: 3}]) s += x;
            s;
        `, 6)
	})
}
