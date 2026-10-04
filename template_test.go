package otto

import (
	"testing"
)

func TestTemplateLiteral(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// A template with no substitutions is just a string.
		test("`hello`", "hello")

		// typeof a template literal is "string".
		test("typeof `abc`", "string")

		// Single substitution.
		test(`
            var name = "world";
            `+"`hello ${name}`"+`;
        `, "hello world")

		// Expressions in substitutions.
		test("`${1 + 2} and ${3 * 4}`", "3 and 12")

		// Adjacent substitutions and surrounding text.
		test("`a${1}b${2}c`", "a1b2c")

		// A leading and trailing substitution.
		test("`${1}middle${2}`", "1middle2")

		// Escape sequences are cooked.
		test("`line1\\nline2`", "line1\nline2")
		test("`tab\\there`", "tab\there")
		test("`\\u0041\\u{1F600}`", "A\U0001F600")
	})
}

func TestTemplateLiteral_expressions(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Function calls, including arrow callbacks, inside a substitution.
		test("`sum=${[1, 2, 3].reduce((a, b) => a + b, 0)}`", "sum=6")

		// A ternary inside a substitution.
		test(`
            var x = 5;
            `+"`${x > 3 ? \"big\" : \"small\"}`"+`;
        `, "big")

		// An object's toString is invoked (ToString semantics).
		test("`value ${ {toString: function() { return \"X\"; }} }`", "value X")
	})
}

func TestTemplateLiteral_nesting(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// A nested template literal inside a substitution.
		test("`outer ${`inner ${1 + 1}`} end`", "outer inner 2 end")

		// A string containing a closing brace inside a substitution must not
		// terminate the substitution early.
		test("`a ${ \"}b{\" } c`", "a }b{ c")

		// Object literal (with its braces) inside a substitution.
		test("`${ {a: 1}.a }`", "1")
	})
}
