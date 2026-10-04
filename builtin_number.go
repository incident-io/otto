package otto

import (
	"math"
	"strconv"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

// Number

func numberValueFromNumberArgumentList(argumentList []Value) Value {
	if len(argumentList) > 0 {
		return argumentList[0].numberValue()
	}
	return intValue(0)
}

func builtinNumber(call FunctionCall) Value {
	return numberValueFromNumberArgumentList(call.ArgumentList)
}

func builtinNewNumber(obj *object, argumentList []Value) Value {
	return objectValue(obj.runtime.newNumber(numberValueFromNumberArgumentList(argumentList)))
}

func builtinNumberToString(call FunctionCall) Value {
	// Will throw a TypeError if ThisObject is not a Number
	value := call.thisClassObject(classNumberName).primitiveValue()
	radix := 10
	radixArgument := call.Argument(0)
	if radixArgument.IsDefined() {
		integer := toIntegerFloat(radixArgument)
		if integer < 2 || integer > 36 {
			panic(call.runtime.panicRangeError("toString() radix must be between 2 and 36"))
		}
		radix = int(integer)
	}
	if radix == 10 {
		return stringValue(value.string())
	}
	return stringValue(numberToStringRadix(value, radix))
}

func builtinNumberValueOf(call FunctionCall) Value {
	return call.thisClassObject(classNumberName).primitiveValue()
}

func builtinNumberToFixed(call FunctionCall) Value {
	precision := toIntegerFloat(call.Argument(0))
	if 20 < precision || 0 > precision {
		panic(call.runtime.panicRangeError("toFixed() precision must be between 0 and 20"))
	}
	if call.This.IsNaN() {
		return stringValue("NaN")
	}
	if value := call.This.float64(); math.Abs(value) >= 1e21 {
		return stringValue(floatToString(value, 64))
	}
	return stringValue(strconv.FormatFloat(call.This.float64(), 'f', int(precision), 64))
}

func builtinNumberToExponential(call FunctionCall) Value {
	if call.This.IsNaN() {
		return stringValue("NaN")
	}
	precision := float64(-1)
	if value := call.Argument(0); value.IsDefined() {
		precision = toIntegerFloat(value)
		if 0 > precision {
			panic(call.runtime.panicRangeError("toString() radix must be between 2 and 36"))
		}
	}
	return stringValue(strconv.FormatFloat(call.This.float64(), 'e', int(precision), 64))
}

func builtinNumberToPrecision(call FunctionCall) Value {
	if call.This.IsNaN() {
		return stringValue("NaN")
	}
	value := call.Argument(0)
	if value.IsUndefined() {
		return stringValue(call.This.string())
	}
	precision := toIntegerFloat(value)
	if 1 > precision {
		panic(call.runtime.panicRangeError("toPrecision() precision must be greater than 1"))
	}
	return stringValue(strconv.FormatFloat(call.This.float64(), 'g', int(precision), 64))
}

func builtinNumberIsNaN(call FunctionCall) Value {
	if len(call.ArgumentList) < 1 {
		return boolValue(false)
	}
	return boolValue(call.Argument(0).IsNaN())
}

func builtinNumberIsInteger(call FunctionCall) Value {
	// No coercion: non-Number arguments are always false.
	value := call.Argument(0)
	if value.kind != valueNumber {
		return boolValue(false)
	}
	num := value.float64()
	if math.IsNaN(num) || math.IsInf(num, 0) {
		return boolValue(false)
	}
	return boolValue(math.Trunc(num) == num)
}

func builtinNumberIsFinite(call FunctionCall) Value {
	// No coercion (unlike the global isFinite): non-Number arguments are false.
	value := call.Argument(0)
	if value.kind != valueNumber {
		return boolValue(false)
	}
	num := value.float64()
	return boolValue(!math.IsNaN(num) && !math.IsInf(num, 0))
}

func builtinNumberIsSafeInteger(call FunctionCall) Value {
	// isInteger AND abs(value) <= 2^53 - 1. No coercion.
	value := call.Argument(0)
	if value.kind != valueNumber {
		return boolValue(false)
	}
	num := value.float64()
	if math.IsNaN(num) || math.IsInf(num, 0) {
		return boolValue(false)
	}
	if math.Trunc(num) != num {
		return boolValue(false)
	}
	const maxSafeInteger = float64(1<<53 - 1)
	return boolValue(math.Abs(num) <= maxSafeInteger)
}

func builtinNumberToLocaleString(call FunctionCall) Value {
	value := call.thisClassObject(classNumberName).primitiveValue()
	locale := call.Argument(0)
	lang := defaultLanguage
	if locale.IsDefined() {
		lang = language.MustParse(locale.string())
	}

	p := message.NewPrinter(lang)
	return stringValue(p.Sprintf("%v", number.Decimal(value.value)))
}
