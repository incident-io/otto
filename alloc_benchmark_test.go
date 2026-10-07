package otto

import (
	"testing"
)

func benchmarkScript(b *testing.B, src string) {
	b.Helper()
	script, err := New().Compile("", src)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	var allocated int64
	for b.Loop() {
		vm := New()
		vm.SetAllocationLimit(1 << 30)
		if _, runErr := vm.Run(script); runErr != nil {
			b.Fatal(runErr)
		}
		allocated = vm.ResourceUsage().AllocatedBytes
	}
	b.ReportMetric(float64(allocated), "js-B/op")
}

func BenchmarkAppendTableRows(b *testing.B) {
	benchmarkScript(b, `
		var s = "";
		for (var i = 0; i < 5000; i++) {
			var row = ["a" + i, "b" + i, "c" + i, "d" + i];
			s += "| " + row.join(" | ") + " |\n";
		}
		s.length
	`)
}

func BenchmarkFunctionCalls(b *testing.B) {
	benchmarkScript(b, `
		function label(o, key) { return o[key] ? key + "=" + o[key] : ""; }
		var out = [];
		for (var i = 0; i < 5000; i++) {
			var o = {name: "n" + i, value: i};
			out.push(label(o, "name"), label(o, "value"));
		}
		out.length
	`)
}

func BenchmarkStringMembers(b *testing.B) {
	benchmarkScript(b, `
		var n = 0;
		for (var i = 0; i < 5000; i++) {
			var s = "key-" + i;
			n += s.length + s.indexOf("-") + s.toUpperCase().length + s.split("-").length;
		}
		n
	`)
}

func BenchmarkRegExpLiterals(b *testing.B) {
	benchmarkScript(b, `
		var n = 0;
		for (var i = 0; i < 5000; i++) {
			var s = "value " + i;
			n += s.replace(/\s+/g, "_").length;
			if (/^value \d+$/.test(s)) n++;
		}
		n
	`)
}
