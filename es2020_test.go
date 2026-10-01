package otto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestES2020Syntax(t *testing.T) {
	tests := map[string]string{
		// Optional chaining.
		"var a = {b: {c: 1}}; a?.b?.c":                   "1",
		"var a = null; String(a?.b.c)":                   "undefined",
		"var a = {f() { return 2 }}; a.f?.()":            "2",
		"var a = {}; String(a.f?.())":                    "undefined",
		"var a = [1]; a?.[0]":                            "1",
		"var n = 0; var a = null; a?.[n++]; n":           "0",
		"var n = 0; var a = null; a?.f(n++); n":          "0",
		"var o = {v: 3, f() { return this.v }}; o?.f()":  "3",
		"var o = {v: 3, f() { return this.v }}; o.f?.()": "3",
		"true ? .5 : 1":                                  "0.5",
		"var a = 1; a?.5:2":                              "0.5",
		"var x = null; (x?.a).b === undefined":           "TypeError",
		"var o = null; delete o?.a":                      "true",

		// Nullish coalescing.
		"null ?? 3":                  "3",
		"0 ?? 3":                     "0",
		"'' ?? 3":                    "",
		"var n = 0; 1 ?? n++; n":     "0",
		"(null || undefined) ?? 'd'": "d",
		"null ?? 1 || 2":             "SyntaxError",
		"1 && null ?? 2":             "SyntaxError",

		// Exponentiation.
		"2 ** 10":               "1024",
		"2 ** 3 ** 2":           "512",
		"(-2) ** 2":             "4",
		"-2 ** 2":               "SyntaxError",
		"2 ** -1":               "0.5",
		"String(1 ** Infinity)": "NaN",
		"var x = 2; x **= 3; x": "8",

		// Logical assignment.
		"var x = 0; x ||= 5; x":              "5",
		"var x = 1; x ||= 5; x":              "1",
		"var x = 1; x &&= 7; x":              "7",
		"var x = 0; x &&= 7; x":              "0",
		"var x = null; x ??= 9; x":           "9",
		"var x = 0; x ??= 9; x":              "0",
		"var n = 0; var x = 1; x ||= n++; n": "0",
		"var o = {get a() { return 1 }, set a(v) { throw 1 }}; o.a ||= 2": "1",

		// Numeric literals.
		"1_000_000":                              "1000000",
		"0xff_ff":                                "65535",
		"0b1010_1010":                            "170",
		"0o7_7":                                  "63",
		"0B11 + 0O7":                             "10",
		"1e1_0":                                  "10000000000",
		"1_0.0_1":                                "10.01",
		"1__0":                                   "SyntaxError",
		"1_":                                     "SyntaxError",
		"0x_1":                                   "SyntaxError",
		"0b2":                                    "SyntaxError",
		"String(Number('1_000'))":                "NaN",
		"var o = {1_0: 'a'}; o[10]":              "a",
		"var o = {0x10: 'a'}; Object.keys(o)[0]": "16",
		"var o = {1.50: 'a'}; o['1.5']":          "a",

		// String escapes.
		"'\\u{1F600}'.length":               "2",
		"'\\u{1F600}' === '\\uD83D\\uDE00'": "true",
		"'\\uD83D\\uDE00' === '😀'":          "true",
		"'\\u{110000}'":                     "SyntaxError",
		"'\\u{}'":                           "SyntaxError",

		// Arrow rest parameters.
		"((...args) => args.length)(1, 2, 3)":  "3",
		"((a, ...r) => a + r.length)(1, 2, 3)": "3",
		"var f = (...r) => r.join(); f()":      "",
		"(...r)":                               "SyntaxError",

		// Other synchronous syntax.
		"var {a, ...rest} = {a: 1, b: 2, c: 3}; JSON.stringify(rest)": `{"b":2,"c":3}`,
		"JSON.stringify({...{a: 1}, b: 2})":                           `{"a":1,"b":2}`,
		"try { null.x } catch { 'caught' }":                           "caught",
		"switch (2) { case 1: 'a'; break; case 2: 'b'; break }":       "b",
	}

	for src, want := range tests {
		t.Run(src, func(t *testing.T) {
			value, err := New().Run(src)
			if strings.HasSuffix(want, "Error") {
				requireErrorKind(t, err, want)
				return
			}
			require.NoError(t, err)
			require.Equal(t, want, value.String())
		})
	}
}

func TestES2020Builtins(t *testing.T) {
	tests := map[string]string{
		"Object.hasOwn({a: 1}, 'a')":                                      "true",
		"Object.hasOwn(Object.create({a: 1}), 'a')":                       "false",
		"globalThis.Math === Math":                                        "true",
		"'😀'.codePointAt(0)":                                              "128512",
		"'😀'.codePointAt(1)":                                              "56832",
		"'abc'.codePointAt(1)":                                            "98",
		"String('abc'.codePointAt(3))":                                    "undefined",
		"String.fromCodePoint(128512) === '😀'":                            "true",
		"String.fromCodePoint(97, 98)":                                    "ab",
		"String.fromCodePoint(0x110000)":                                  "RangeError",
		"String.fromCodePoint(1.5)":                                       "RangeError",
		"'\\u0041\\u030A'.normalize().length":                             "1",
		"'\\u00C5'.normalize('NFD').length":                               "2",
		"'\\uFB01'.normalize('NFKC')":                                     "fi",
		"'a'.normalize('x')":                                              "RangeError",
		"Number.EPSILON === 2 ** -52":                                     "true",
		"Number.MAX_SAFE_INTEGER":                                         "9007199254740991",
		"Number.MIN_SAFE_INTEGER":                                         "-9007199254740991",
		"Number.isNaN('x')":                                               "false",
		"Number.isNaN(NaN)":                                               "true",
		"(0.000001234).toPrecision(2)":                                    "0.0000012",
		"(123456).toPrecision(2)":                                         "1.2e+5",
		"(123.456).toPrecision(4)":                                        "123.5",
		"(1).toPrecision(101)":                                            "RangeError",
		"(1.5).toExponential(3)":                                          "1.500e+0",
		"(-12345).toExponential(2)":                                       "-1.23e+4",
		"(Infinity).toExponential()":                                      "Infinity",
		"(1).toExponential(101)":                                          "RangeError",
		"Math.imul(3, 4)":                                                 "12",
		"Math.imul(0xffffffff, 5)":                                        "-5",
		"/a.c/s.test('a\\nc')":                                            "true",
		"/a.c/.test('a\\nc')":                                             "false",
		"/x/sgm.flags":                                                    "gms",
		"/x/s.dotAll":                                                     "true",
		"/(?<y>\\d{4})-(?<m>\\d\\d)/.exec('in 2024-05').groups.m":         "05",
		"String(/(\\d)/.exec('1').groups)":                                "undefined",
		"'abc'.replace(/(?<first>a)/, '$<first>!')":                       "a!bc",
		"'abc'.replace(/(a)/, '$<first>!')":                               "$<first>!bc",
		"'abc'.replace(/(?<first>a)/g, '[$<first>]')":                     "[a]bc",
		"[1, 2, 3].toReversed().join()":                                   "3,2,1",
		"var a = [3, 1, 2]; a.toSorted().join() + '|' + a.join()":         "1,2,3|3,1,2",
		"[3, 1, 2].toSorted((x, y) => y - x).join()":                      "3,2,1",
		"[1, 2, 3].with(-1, 9).join()":                                    "1,2,9",
		"[1, 2, 3].with(3, 9)":                                            "RangeError",
		"[1, 2].toSorted(1)":                                              "TypeError",
		"JSON.stringify({b: 1, a: 2, c: {z: 1, y: [1, 'x']}})":            `{"b":1,"a":2,"c":{"z":1,"y":[1,"x"]}}`,
		"JSON.stringify({b: 1, 2: 1, 1: 2, a: 4})":                        `{"1":2,"2":1,"b":1,"a":4}`,
		`JSON.stringify('\u2028<>&')`:                                     "\"\u2028<>&\"",
		`JSON.stringify('\u0001\n')`:                                      `"\u0001\n"`,
		"JSON.stringify({a: 1e21, b: -0, c: NaN, d: 1e-7})":               `{"a":1e+21,"b":0,"c":null,"d":1e-7}`,
		"JSON.stringify(Object.getOwnPropertyDescriptor({a: 1}, 'a'))":    `{"value":1,"writable":true,"enumerable":true,"configurable":true}`,
		"var a = [1, 2]; a.x = 1; a[5] = 3; Object.keys(a).join()":        "0,1,5,x",
		"Object.keys({b: 1, 10: 1, 2: 1, '01': 1, 4294967295: 1}).join()": "2,10,b,01,4294967295",
		"var r = []; for (var k in {b: 1, 1: 1}) r.push(k); r.join()":     "1,b",
		"new Error('m', {cause: 1}).cause":                                "1",
		"TypeError('m', {cause: 'x'}).cause":                              "x",
		"'cause' in new Error('m', {})":                                   "false",
		"'cause' in new Error('m', {cause: undefined})":                   "true",
		"Object.keys(new RangeError('m', {cause: 1})).length":             "0",
		"JSON.stringify(new Error('m'))":                                  "{}",
		"JSON.stringify(Object.groupBy([1, 2, 3, 4, 5], x => x % 2 ? 'odd' : 'even'))": `{"odd":[1,3,5],"even":[2,4]}`,
		"Object.getPrototypeOf(Object.groupBy([], x => x))":                            "null",
		"JSON.stringify(Object.groupBy('a😀b', c => c.length))":                         `{"1":["a","b"],"2":["😀"]}`,
		"Object.groupBy({length: 1, 0: 'a'}, x => x)":                                  "TypeError",
		"Object.groupBy([1], 1)":                                                       "TypeError",
		"'abc'.padEnd(6, '\\uD83D\\uDCA9') === 'abc\\uD83D\\uDCA9\\uD83D'":             "true",
		"'x'.padEnd(4, '😀').length":                                                    "4",
		"'abcd'.replace(/(.)(.)|(x)/, '$<42$1>')":                                      "$<42a>cd",
		"'abcd'.replace(/(?<fst>.)(?<snd>.)/, '$<snd>$<fst>|$<nope>|$1')":              "ba||acd",
		"function F(a, b, c) { this.s = a + b + c }; new F(...[1, 2], 3).s":            "6",
		"var r = /a/g; Object.defineProperty(r, 'flags', {value: 'x'}); r.flags":       "x",
		"JSON.stringify({a: [1, {b: 2, c: []}, {}], d: 'x'}, null, 2)":                 "{\n  \"a\": [\n    1,\n    {\n      \"b\": 2,\n      \"c\": []\n    },\n    {}\n  ],\n  \"d\": \"x\"\n}",
		"JSON.stringify([1, [2]], null, 'abcdefghijklmn')":                             "[\nabcdefghij1,\nabcdefghij[\nabcdefghijabcdefghij2\nabcdefghij]\n]",
		"00_0": "SyntaxError",
		"07_1": "SyntaxError",
		"Array.prototype.toReversed.call(undefined)":                        "TypeError",
		"Array.prototype.with.call(undefined, 0, 0)":                        "TypeError",
		"String.prototype.codePointAt.call(undefined, 0)":                   "TypeError",
		"String.prototype.normalize.call(null)":                             "TypeError",
		"String.prototype.charAt.call(12, 1)":                               "2",
		"String.prototype.codePointAt.call({toString() { return 'z' }}, 0)": "122",
		"Object.prototype.toString.call(undefined)":                         "[object Undefined]",
		"(function () { return this === globalThis }).call(undefined)":      "true",
		"(function () { return this === globalThis }).bind(undefined)()":    "true",
	}

	for src, want := range tests {
		t.Run(src, func(t *testing.T) {
			value, err := New().Run(src)
			if strings.HasSuffix(want, "Error") {
				requireErrorKind(t, err, want)
				return
			}
			require.NoError(t, err)
			require.Equal(t, want, value.String())
		})
	}
}

func TestES2020BuiltinsRespectLimits(t *testing.T) {
	vm := New()
	vm.SetAllocationLimit(1 << 20)
	_, err := vm.Run(`var a = []; a.length = 1e6; a.toSorted()`)
	require.ErrorContains(t, err, "RangeError")

	vm = New()
	vm.SetStringLengthLimit(1 << 10)
	_, err = vm.Run(`'\u00C5'.repeat(500).normalize('NFD')`)
	require.ErrorContains(t, err, "RangeError")
}

// requireErrorKind checks err is a JavaScript error of the given kind. The
// parser reports syntax errors without the "SyntaxError" prefix.
func requireErrorKind(t *testing.T, err error, kind string) {
	t.Helper()
	require.Error(t, err)
	if kind == "SyntaxError" && strings.HasPrefix(err.Error(), "(anonymous): Line ") {
		return
	}
	require.True(t, strings.HasPrefix(err.Error(), kind), err.Error())
}
