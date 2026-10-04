package otto

import (
	"testing"
)

func TestArrayES6Find(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`[1, 2, 3, 4].find(function(x) { return x > 2; })`, 3)
		test(`[1, 2, 3, 4].find(function(x) { return x > 10; })`, "undefined")
		test(`[1, 2, 3, 4].findIndex(function(x) { return x > 2; })`, 2)
		test(`[1, 2, 3, 4].findIndex(function(x) { return x > 10; })`, -1)

		// find passes (element, index, array)
		test(`
			var seen = [];
			[10, 20, 30].find(function(v, i, a) { seen.push(i + ":" + v + ":" + a.length); return false; });
			seen.join(",");
		`, "0:10:3,1:20:3,2:30:3")

		// thisArg
		test(`
			[1, 2, 3].find(function(x) { return x === this.target; }, { target: 2 });
		`, 2)

		// findLast / findLastIndex iterate from the end
		test(`[1, 2, 3, 4].findLast(function(x) { return x < 3; })`, 2)
		test(`[1, 2, 3, 4].findLastIndex(function(x) { return x < 3; })`, 1)
		test(`[1, 2, 3, 4].findLast(function(x) { return x > 10; })`, "undefined")
		test(`[1, 2, 3, 4].findLastIndex(function(x) { return x > 10; })`, -1)

		// holes are visited as undefined (find does not skip)
		test(`
			var count = 0;
			[ , , ].find(function() { count++; return false; });
			count;
		`, 2)

		test(`raise: [].find(undefined)`, "TypeError: Array.find \"undefined\" is not callable")
		test(`raise: [].findLastIndex(123)`, "TypeError: Array.findLastIndex \"123\" is not callable")
	})
}

func TestArrayES6Includes(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`[1, 2, 3].includes(2)`, true)
		test(`[1, 2, 3].includes(4)`, false)

		// SameValueZero: NaN is found
		test(`[1, NaN, 3].includes(NaN)`, true)
		test(`[1, 2, 3].indexOf(NaN)`, -1)

		// +0 and -0 are equal
		test(`[0].includes(-0)`, true)
		test(`[-0].includes(0)`, true)

		// fromIndex
		test(`[1, 2, 3, 2].includes(2, 2)`, true)
		test(`[1, 2, 3].includes(1, 1)`, false)

		// negative fromIndex
		test(`[1, 2, 3].includes(3, -1)`, true)
		test(`[1, 2, 3].includes(1, -1)`, false)
		test(`[1, 2, 3].includes(1, -100)`, true)

		// fromIndex Infinity
		test(`[1, 2, 3].includes(2, Infinity)`, false)
		test(`[1, 2, 3].includes(2, -Infinity)`, true)

		test(`[].includes(1)`, false)
	})
}

func TestArrayES6Fill(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`[1, 2, 3, 4].fill(0).toString()`, "0,0,0,0")
		test(`[1, 2, 3, 4].fill(5, 1).toString()`, "1,5,5,5")
		test(`[1, 2, 3, 4].fill(5, 1, 3).toString()`, "1,5,5,4")

		// negative start/end
		test(`[1, 2, 3, 4].fill(5, -2).toString()`, "1,2,5,5")
		test(`[1, 2, 3, 4].fill(5, -3, -1).toString()`, "1,5,5,4")

		// returns the same array
		test(`var a = [1, 2, 3]; a.fill(0) === a;`, true)
	})
}

func TestArrayES6CopyWithin(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`[1, 2, 3, 4, 5].copyWithin(0, 3).toString()`, "4,5,3,4,5")
		test(`[1, 2, 3, 4, 5].copyWithin(1, 3).toString()`, "1,4,5,4,5")
		test(`[1, 2, 3, 4, 5].copyWithin(0, 3, 4).toString()`, "4,2,3,4,5")
		test(`[1, 2, 3, 4, 5].copyWithin(-2, -3, -1).toString()`, "1,2,3,3,4")

		// overlapping ranges copied correctly (forward target after source)
		test(`[1, 2, 3, 4, 5].copyWithin(2, 0).toString()`, "1,2,1,2,3")

		test(`var a = [1, 2, 3]; a.copyWithin(0, 1) === a;`, true)
	})
}

func TestArrayES6Flat(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`[1, [2, 3], [4, [5]]].flat().toString()`, "1,2,3,4,5")
		test(`[1, [2, [3, [4]]]].flat().toString()`, "1,2,3,4")
		test(`[1, [2, [3, [4]]]].flat(2).toString()`, "1,2,3,4")
		test(`[1, [2, [3, [4]]]].flat(Infinity).toString()`, "1,2,3,4")
		test(`[1, [2, [3, [4]]]].flat(0).toString()`, "1,2,3,4")

		// .length default is 0
		test(`[].flat.length`, 0)

		// holes are skipped
		test(`
			var a = [1, , 3, [4, , 6]];
			a.flat().length;
		`, 4)

		// flatMap maps then flattens one level
		test(`[1, 2, 3].flatMap(function(x) { return [x, x * 2]; }).toString()`, "1,2,2,4,3,6")
		test(`[1, 2, 3].flatMap(function(x) { return x; }).toString()`, "1,2,3")
		// only flattens one level
		test(`[1, 2].flatMap(function(x) { return [[x]]; }).length`, 2)

		test(`raise: [].flatMap(undefined)`, "TypeError: Array.flatMap \"undefined\" is not callable")
	})
}

func TestArrayES6At(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`[1, 2, 3].at(0)`, 1)
		test(`[1, 2, 3].at(2)`, 3)
		test(`[1, 2, 3].at(-1)`, 3)
		test(`[1, 2, 3].at(-3)`, 1)
		test(`[1, 2, 3].at(3)`, "undefined")
		test(`[1, 2, 3].at(-4)`, "undefined")
		// fractional index truncates
		test(`[1, 2, 3].at(1.9)`, 2)
	})
}

func TestArrayES6From(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// from a string
		test(`Array.from("abc").toString()`, "a,b,c")
		test(`Array.from("abc").length`, 3)

		// from an array-like object
		test(`Array.from({ length: 2, 0: "a", 1: "b" }).toString()`, "a,b")
		test(`Array.from({ length: 3 }).length`, 3)

		// from an array
		test(`Array.from([1, 2, 3]).toString()`, "1,2,3")

		// mapFn(value, index)
		test(`Array.from([1, 2, 3], function(x) { return x * 2; }).toString()`, "2,4,6")
		test(`Array.from({ length: 3 }, function(_, i) { return i; }).toString()`, "0,1,2")
		test(`Array.from("ab", function(c, i) { return c + i; }).toString()`, "a0,b1")

		// .length is 1
		test(`Array.from.length`, 1)

		test(`raise: Array.from(undefined)`, "TypeError: Array.from requires an array-like object - not null or undefined")
		test(`raise: Array.from([], 123)`, "TypeError: Array.from when provided a map function must be callable")
	})
}

func TestArrayES6Of(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`Array.of(1, 2, 3).toString()`, "1,2,3")
		test(`Array.of(7).length`, 1)
		test(`Array.of(7)[0]`, 7)
		test(`Array.of().length`, 0)
		// .length is 0
		test(`Array.of.length`, 0)
	})
}
