package otto

import (
	"testing"
)

func TestString_includes(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`"abc".includes("b")`, true)
		test(`"abc".includes("d")`, false)
		test(`"abc".includes("")`, true)
		test(`"abcabc".includes("a", 1)`, true)
		test(`"abcabc".includes("a", 4)`, false)
		test(`"abc".includes("a", -5)`, true)
		test(`raise: "abc".includes(/b/)`, "TypeError: First argument to String.prototype.includes must not be a regular expression")
	})
}

func TestString_endsWith(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`"abc".endsWith("c")`, true)
		test(`"abc".endsWith("b")`, false)
		test(`"abc".endsWith("")`, true)
		test(`"abc".endsWith("b", 2)`, true)
		test(`"abc".endsWith("c", 2)`, false)
		test(`raise: "abc".endsWith(/c/)`, "TypeError: First argument to String.prototype.endsWith must not be a regular expression")
	})
}

func TestString_repeat(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`"ab".repeat(3)`, "ababab")
		test(`"ab".repeat(0)`, "")
		test(`"ab".repeat(1)`, "ab")
		test(`raise: "ab".repeat(-1)`, "RangeError: Invalid count value")
		test(`raise: "ab".repeat(Infinity)`, "RangeError: Invalid count value")
	})
}

func TestString_padStart(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`"5".padStart(3, "0")`, "005")
		test(`"abc".padStart(6)`, "   abc")
		test(`"abc".padStart(2)`, "abc")
		test(`"abc".padStart(10, "123")`, "1231231abc")
		test(`"abc".padStart(6, "")`, "abc")
	})
}

func TestString_padEnd(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`"5".padEnd(3, "0")`, "500")
		test(`"abc".padEnd(6)`, "abc   ")
		test(`"abc".padEnd(2)`, "abc")
		test(`"abc".padEnd(10, "123")`, "abc1231231")
		test(`"abc".padEnd(6, "")`, "abc")
	})
}

func TestString_at(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`"abc".at(0)`, "a")
		test(`"abc".at(2)`, "c")
		test(`"abc".at(-1)`, "c")
		test(`"abc".at(-3)`, "a")
		test(`"abc".at(3)`, "undefined")
		test(`"abc".at(-4)`, "undefined")
		test(`typeof "abc".at(5)`, "undefined")
	})
}

func TestString_replaceAll(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`"aabbaa".replaceAll("a", "x")`, "xxbbxx")
		test(`"abcabc".replaceAll("bc", "Y")`, "aYaY")
		test(`"abc".replaceAll("z", "Y")`, "abc")
		test(`"abcabc".replaceAll("a", "[$&]")`, "[a]bc[a]bc")
		test(`"a-b-c".replaceAll("-", function(m, i) { return i; })`, "a1b3c")
		test(`"aaa".replaceAll(/a/g, "x")`, "xxx")
		test(`raise: "aaa".replaceAll(/a/, "x")`, "TypeError: replaceAll must be called with a global RegExp")
	})
}
