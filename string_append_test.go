package otto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringAppend(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
			var s = "x".repeat(300);
			var a = s + "a";
			var b = s + "b";
			var aa = a + "a";
			var ab = a + "b";
			[a.slice(-2), b.slice(-2), aa.slice(-3), ab.slice(-3), a.length, aa.length].join()
		`, "xa,xb,xaa,xab,301,302")

		test(`
			var s = "";
			var parts = [];
			for (var i = 0; i < 2000; i++) {
				s += "| " + i + " |\n";
				if (i % 500 === 0) parts.push(s);
			}
			var t = "";
			for (var i = 0; i < 2000; i++) t = t.concat("| " + i + " |\n");
			[s === t, parts[1] === t.slice(0, parts[1].length), parts[1].length, s.charAt(s.length - 2)].join()
		`, "true,true,3898,|")

		test(`
			var s = "é".repeat(200);
			s += "😀";
			s += "x";
			[s.length, s.charCodeAt(200).toString(16), s.at(-1), s.indexOf("😀"), JSON.stringify(s).length].join()
		`, "203,d83d,x,200,205")

		test(`
			var s = "y".repeat(300);
			s += s;
			s += s;
			[s.length, /^y+$/.test(s)].join()
		`, "1200,true")
	})
}

func TestStringAppendLimits(t *testing.T) {
	vm := New()
	vm.SetStringLengthLimit(1 << 20)
	vm.SetAllocationLimit(64 << 20)
	v, err := vm.Run(`var s = ""; for (var i = 0; i < 100000; i++) s += "0123456789"; s.length`)
	require.NoError(t, err)
	require.Equal(t, "1000000", v.String())
	require.Less(t, vm.ResourceUsage().AllocatedBytes, int64(8<<20))

	_, err = vm.Run(`for (;;) s += "0123456789"`)
	require.EqualError(t, err, "RangeError: Invalid string length")
	require.Greater(t, vm.ResourceUsage().MaxStringBytes, 1<<20)

	vm.SetAllocationLimit(64 << 20)
	_, err = vm.Run(`var a = []; for (;;) { var t = "x".repeat(1000); a.push(t + "a"); a.push(t + "b") }`)
	require.EqualError(t, err, "RangeError: Allocation limit exceeded")
}

func TestPrimitiveStringMember(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`"abc".length + "abc"[1] + "abc"[5] + "abc".nope`, "3bundefinedundefined")
		test(`"abc".hasOwnProperty("1") + "," + "abc".hasOwnProperty("x")`, "true,false")
		test(`
			Object.defineProperty(String.prototype, "self", {get: function() { return typeof this; }, configurable: true});
			String.prototype.kind = function() { return typeof this + this.length; };
			var r = ["abc".self, "abc".kind(), "abc".toUpperCase(), Object.prototype.toString.call("abc")].join();
			delete String.prototype.self;
			delete String.prototype.kind;
			r
		`, "object,object3,ABC,[object String]")
		test(`var s = "abc"; s.x = 1; s.length = 9; [s.x, s.length, delete s.length].join()`, ",3,false")
		test(`var s = "abc"; s.length += 1; s.length`, 3)
	})
}

func TestArgumentsCreatedOnRead(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`(function(a) { a = 2; return arguments[0]; })(1)`, 2)
		test(`(function(a) { arguments[0] = 3; return a; })(1)`, 3)
		test(`(function(a) { return (() => arguments.length + arguments[1])(); })(1, 2)`, 4)
		test(`(function() { return eval("arguments[0]"); })(5)`, 5)
		test(`(function() { arguments = 7; return arguments; })(1)`, 7)
		test(`(function(arguments) { return arguments; })(8)`, 8)
		test(`(function f() { return arguments.callee === f; })()`, true)
		test(`(function() { var arguments; return arguments.length; })(1, 2)`, 2)
		test(`(function() { function arguments() {} return typeof arguments; })()`, "function")
	})
}

func TestRegExpLiteralInLoop(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
			var res = [];
			for (var i = 0; i < 3; i++) {
				var re = /a(\d)/g;
				re.exec("a1a2");
				res.push(re);
			}
			[res[0] !== res[1], res[0].lastIndex, res[1].exec("a1a2")[1], res[2].lastIndex].join()
		`, "true,2,2,2")
		test(`new RegExp("b", "i").test("B") && !new RegExp("b").test("B")`, true)
	})
}
