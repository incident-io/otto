package otto

import (
	"testing"
)

func TestLetConstBasic(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`let x = 1; x;`, 1)
		test(`const y = 2; y;`, 2)
		test(`let a = 1, b = 2; a + b;`, 3)
		test(`let u; typeof u;`, "undefined")

		// const reassignment throws a TypeError.
		test(`raise:
            const c = 1;
            c = 2;
        `, "TypeError: Assignment to constant variable.")
	})
}

func TestLetConstBlockScope(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// An inner block does not leak its binding.
		test(`
            let a = 1;
            { let a = 2; }
            a;
        `, 1)

		// A binding declared in a block is not visible outside it.
		test(`
            { let b = 5; }
            typeof b;
        `, "undefined")

		// Nested shadowing.
		test(`
            let n = 1;
            { let n = 2; { let n = 3; } }
            n;
        `, 1)

		// const is block scoped too.
		test(`
            const k = 10;
            { const k = 20; }
            k;
        `, 10)
	})
}

func TestLetPerIterationBinding(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// The classic case: each iteration of a for (let ...) loop has its own
		// binding, so closures capture distinct values.
		test(`
            var fns = [];
            for (let i = 0; i < 3; i++) {
                fns.push(function() { return i; });
            }
            [fns[0](), fns[1](), fns[2]()].join(",");
        `, "0,1,2")

		// A var-declared loop variable is shared across iterations.
		test(`
            var fns = [];
            for (var j = 0; j < 3; j++) {
                fns.push(function() { return j; });
            }
            [fns[0](), fns[2]()].join(",");
        `, "3,3")

		// A let in the loop body is fresh each iteration.
		test(`
            var fns = [];
            for (let i = 0; i < 3; i++) {
                let doubled = i * 10;
                fns.push(function() { return doubled; });
            }
            [fns[0](), fns[1](), fns[2]()].join(",");
        `, "0,10,20")

		// Sum still works (mutation across iterations behaves normally).
		test(`
            let total = 0;
            for (let i = 0; i <= 4; i++) total += i;
            total;
        `, 10)
	})
}

func TestLetForIn(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            var keys = [];
            var o = { a: 1, b: 2 };
            for (let p in o) keys.push(p);
            keys.sort().join(",");
        `, "a,b")

		// Each iteration's binding is independent.
		test(`
            var fns = [];
            for (let p in { x: 1, y: 1 }) {
                fns.push(function() { return p; });
            }
            fns.length;
        `, 2)
	})
}

func TestLetInFunctionAndWhile(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            function f() { let z = 42; return z; }
            f();
        `, 42)

		test(`
            let total = 0, c = 0;
            while (c < 3) {
                let step = c * 2;
                total += step;
                c++;
            }
            total;
        `, 6)
	})
}
