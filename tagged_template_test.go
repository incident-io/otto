package otto

import (
	"testing"
)

func TestTaggedTemplate(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// The tag receives the cooked string parts and the interpolated values.
		test(`
            function tag(strings, ...values) {
                return strings.join("|") + "#" + values.join(",");
            }
            tag`+"`a${1}b${2}c`"+`;
        `, "a|b|c#1,2")

		// strings always has one more element than values.
		test(`
            function tag(strings, ...values) { return strings.length + ":" + values.length; }
            tag`+"`${1}${2}${3}`"+`;
        `, "4:3")

		// Cooked vs raw strings.
		test(`
            function cooked(strings) { return strings[0]; }
            cooked`+"`a\\nb${1}`"+`;
        `, "a\nb")
		test(`
            function raw(strings) { return strings.raw[0]; }
            raw`+"`a\\nb${1}`"+`;
        `, `a\nb`)

		// A method tag is called with the right `this`.
		test(`
            var o = { x: "X", tag(strings) { return this.x + strings[0]; } };
            o.tag`+"`y`"+`;
        `, "Xy")
	})
}

func TestStringRaw(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test("String.raw`a\\tb`;", `a\tb`)
		test("String.raw`x${1}y${2}z`;", "x1y2z")
		test("String.raw`line\\nbreak ${1 + 1}`;", `line\nbreak 2`)
	})
}
