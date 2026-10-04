package otto

import (
	"testing"
)

func TestClassBasic(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`
            class C { constructor(x) { this.x = x; } get() { return this.x; } }
            new C(5).get();
        `, 5)

		// typeof a class is "function".
		test(`class C {} typeof C;`, "function")

		// Methods live on the prototype.
		test(`
            class C { foo() { return 1; } }
            var c = new C();
            [c.foo(), C.prototype.foo === c.foo].join(",");
        `, "1,true")

		// Static methods.
		test(`class C { static bar() { return 42; } } C.bar();`, 42)

		// Method chaining via `this`.
		test(`
            class C { constructor() { this.list = []; } add(x) { this.list.push(x); return this; } }
            var c = new C();
            c.add(1).add(2);
            c.list.join(",");
        `, "1,2")

		// Class expression.
		test(`
            var C = class { m() { return "expr"; } };
            new C().m();
        `, "expr")

		// Computed method name.
		test(`
            class C { ["dyn" + "amic"]() { return "computed"; } }
            new C().dynamic();
        `, "computed")
	})
}

func TestClassAccessors(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`class C { get v() { return 9; } } new C().v;`, 9)

		test(`
            var store = 0;
            class C { set v(n) { store = n; } }
            var c = new C();
            c.v = 3;
            store;
        `, 3)

		// Getter and setter on the same property.
		test(`
            class C {
                get v() { return this._v; }
                set v(n) { this._v = n * 2; }
            }
            var c = new C();
            c.v = 5;
            c.v;
        `, 10)
	})
}

// TestClassAccessorDescriptor guards against a regression where
// Object.getOwnPropertyDescriptor panicked on accessor (getter/setter)
// properties because the descriptor was reflected as a data descriptor.
func TestClassAccessorDescriptor(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// Getter and setter together: the descriptor must expose get/set
		// (not value/writable) and the correct enumerable/configurable flags.
		test(`
            class C { get x() { return 1; } set x(v) {} }
            var d = Object.getOwnPropertyDescriptor(C.prototype, "x");
            [
                typeof d.get,
                typeof d.set,
                "value" in d,
                "writable" in d,
                d.enumerable,
                d.configurable,
            ].join(",");
        `, "function,function,false,false,false,true")

		// Getter only: set must be undefined, get must be the function.
		test(`
            class C { get x() { return 1; } }
            var d = Object.getOwnPropertyDescriptor(C.prototype, "x");
            [typeof d.get, d.set === undefined, "get" in d, "set" in d].join(",");
        `, "function,true,true,true")

		// Setter only: get must be undefined, set must be the function.
		test(`
            class C { set x(v) {} }
            var d = Object.getOwnPropertyDescriptor(C.prototype, "x");
            [d.get === undefined, typeof d.set].join(",");
        `, "true,function")
	})
}

func TestClassInheritance(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		// instanceof through the chain.
		test(`class A {} class B extends A {} (new B()) instanceof A;`, true)

		// Inherited methods.
		test(`
            class A { hi() { return "a"; } }
            class B extends A {}
            new B().hi();
        `, "a")

		// A default derived constructor forwards to super.
		test(`
            class A { constructor() { this.t = "A"; } }
            class B extends A {}
            new B().t;
        `, "A")

		// Explicit super(...) call.
		test(`
            class A { constructor(n) { this.n = n; } }
            class B extends A { constructor(n) { super(n); } }
            new B(7).n;
        `, 7)

		// super.method().
		test(`
            class A { who() { return "A"; } }
            class B extends A { who() { return "B+" + super.who(); } }
            new B().who();
        `, "B+A")

		// Overriding with full example.
		test(`
            class Animal {
                constructor(name) { this.name = name; }
                speak() { return this.name + " makes a sound"; }
            }
            class Dog extends Animal {
                speak() { return this.name + " barks"; }
            }
            new Dog("Rex").speak();
        `, "Rex barks")

		// Static inheritance.
		test(`
            class A { static make() { return "made"; } }
            class B extends A {}
            B.make();
        `, "made")
	})
}
