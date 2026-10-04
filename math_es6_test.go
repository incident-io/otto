package otto

import (
	"testing"
)

func TestMath_sign(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`Math.sign.length`, 1)
		test(`Math.sign(-3)`, -1)
		test(`Math.sign(3)`, 1)
		test(`Math.sign(0)`, 0)
		test(`1 / Math.sign(0)`, infinity)
		test(`1 / Math.sign(-0)`, -infinity)
		test(`Math.sign(NaN)`, naN)
		test(`Math.sign(-Infinity)`, -1)
		test(`Math.sign(Infinity)`, 1)
		test(`Math.sign("-5")`, -1)
		test(`Math.sign()`, naN)
	})
}

func TestMath_hypot(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`Math.hypot.length`, 2)
		test(`Math.hypot(3, 4)`, 5)
		test(`Math.hypot(3, 4, 12)`, 13)
		test(`Math.hypot()`, 0)
		test(`Math.hypot(0, 0)`, 0)
		test(`Math.hypot(5)`, 5)
		test(`Math.hypot(Infinity, NaN)`, infinity)
		test(`Math.hypot(NaN, Infinity)`, infinity)
		test(`Math.hypot(-Infinity, 1)`, infinity)
		test(`Math.hypot(NaN, 1)`, naN)
		test(`Math.hypot(1, NaN)`, naN)
		// Guard against intermediate overflow.
		test(`Math.hypot(3e300, 4e300)`, 5e300)
	})
}

func TestMath_clz32(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`Math.clz32.length`, 1)
		test(`Math.clz32(1)`, 31)
		test(`Math.clz32(0)`, 32)
		test(`Math.clz32(2)`, 30)
		test(`Math.clz32(0xffffffff)`, 0)
		test(`Math.clz32(0x80000000)`, 0)
		test(`Math.clz32()`, 32)
		test(`Math.clz32(NaN)`, 32)
		test(`Math.clz32(Infinity)`, 32)
		// ToUint32 wraps values larger than 2^32.
		test(`Math.clz32(4294967297)`, 31)
	})
}

func TestMath_fround(t *testing.T) {
	tt(t, func() {
		test, _ := test()

		test(`Math.fround.length`, 1)
		test(`Math.fround(0)`, 0)
		test(`1 / Math.fround(-0)`, -infinity)
		test(`Math.fround(1)`, 1)
		test(`Math.fround(NaN)`, naN)
		test(`Math.fround(Infinity)`, infinity)
		test(`Math.fround(-Infinity)`, -infinity)
		// 1.1 is not representable as a float32; rounds to nearest single.
		test(`Math.fround(1.1)`, 1.100000023841858)
		test(`Math.fround(Math.pow(2, 150))`, infinity)
	})
}
