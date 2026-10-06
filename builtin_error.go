package otto

import (
	"fmt"
)

func builtinError(call FunctionCall) Value {
	return objectValue(call.runtime.installErrorCause(call.runtime.newError(classErrorName, call.Argument(0), 1), call.Argument(1)))
}

func builtinNewError(obj *object, argumentList []Value) Value {
	return objectValue(obj.runtime.installErrorCause(obj.runtime.newError(classErrorName, valueOfArrayIndex(argumentList, 0), 0), valueOfArrayIndex(argumentList, 1)))
}

func builtinErrorToString(call FunctionCall) Value {
	thisObject := call.thisObject()
	if thisObject == nil {
		panic(call.runtime.panicTypeError("Error.toString is nil"))
	}

	name := classErrorName
	nameValue := thisObject.get("name")
	if nameValue.IsDefined() {
		name = nameValue.string()
	}

	message := ""
	messageValue := thisObject.get("message")
	if messageValue.IsDefined() {
		message = messageValue.string()
	}

	if len(name) == 0 {
		return stringValue(message)
	}

	if len(message) == 0 {
		return stringValue(name)
	}

	call.runtime.allocateString(len(name) + len(message) + 2)
	return stringValue(fmt.Sprintf("%s: %s", name, message))
}

func (rt *runtime) newEvalError(message Value) *object {
	o := rt.newErrorObject("EvalError", message, 0)
	o.prototype = rt.global.EvalErrorPrototype
	return o
}

func builtinEvalError(call FunctionCall) Value {
	return objectValue(call.runtime.installErrorCause(call.runtime.newEvalError(call.Argument(0)), call.Argument(1)))
}

func builtinNewEvalError(obj *object, argumentList []Value) Value {
	return objectValue(obj.runtime.installErrorCause(obj.runtime.newEvalError(valueOfArrayIndex(argumentList, 0)), valueOfArrayIndex(argumentList, 1)))
}

func (rt *runtime) newTypeError(message Value) *object {
	o := rt.newErrorObject("TypeError", message, 0)
	o.prototype = rt.global.TypeErrorPrototype
	return o
}

func builtinTypeError(call FunctionCall) Value {
	return objectValue(call.runtime.installErrorCause(call.runtime.newTypeError(call.Argument(0)), call.Argument(1)))
}

func builtinNewTypeError(obj *object, argumentList []Value) Value {
	return objectValue(obj.runtime.installErrorCause(obj.runtime.newTypeError(valueOfArrayIndex(argumentList, 0)), valueOfArrayIndex(argumentList, 1)))
}

func (rt *runtime) newRangeError(message Value) *object {
	o := rt.newErrorObject("RangeError", message, 0)
	o.prototype = rt.global.RangeErrorPrototype
	return o
}

func builtinRangeError(call FunctionCall) Value {
	return objectValue(call.runtime.installErrorCause(call.runtime.newRangeError(call.Argument(0)), call.Argument(1)))
}

func builtinNewRangeError(obj *object, argumentList []Value) Value {
	return objectValue(obj.runtime.installErrorCause(obj.runtime.newRangeError(valueOfArrayIndex(argumentList, 0)), valueOfArrayIndex(argumentList, 1)))
}

func (rt *runtime) newURIError(message Value) *object {
	o := rt.newErrorObject("URIError", message, 0)
	o.prototype = rt.global.URIErrorPrototype
	return o
}

func (rt *runtime) newReferenceError(message Value) *object {
	o := rt.newErrorObject("ReferenceError", message, 0)
	o.prototype = rt.global.ReferenceErrorPrototype
	return o
}

func builtinReferenceError(call FunctionCall) Value {
	return objectValue(call.runtime.installErrorCause(call.runtime.newReferenceError(call.Argument(0)), call.Argument(1)))
}

func builtinNewReferenceError(obj *object, argumentList []Value) Value {
	return objectValue(obj.runtime.installErrorCause(obj.runtime.newReferenceError(valueOfArrayIndex(argumentList, 0)), valueOfArrayIndex(argumentList, 1)))
}

func (rt *runtime) newSyntaxError(message Value) *object {
	o := rt.newErrorObject("SyntaxError", message, 0)
	o.prototype = rt.global.SyntaxErrorPrototype
	return o
}

func builtinSyntaxError(call FunctionCall) Value {
	return objectValue(call.runtime.installErrorCause(call.runtime.newSyntaxError(call.Argument(0)), call.Argument(1)))
}

func builtinNewSyntaxError(obj *object, argumentList []Value) Value {
	return objectValue(obj.runtime.installErrorCause(obj.runtime.newSyntaxError(valueOfArrayIndex(argumentList, 0)), valueOfArrayIndex(argumentList, 1)))
}

func builtinURIError(call FunctionCall) Value {
	return objectValue(call.runtime.installErrorCause(call.runtime.newURIError(call.Argument(0)), call.Argument(1)))
}

func builtinNewURIError(obj *object, argumentList []Value) Value {
	return objectValue(obj.runtime.installErrorCause(obj.runtime.newURIError(valueOfArrayIndex(argumentList, 0)), valueOfArrayIndex(argumentList, 1)))
}

// installErrorCause implements InstallErrorCause.
func (rt *runtime) installErrorCause(obj *object, options Value) *object {
	if options := options.object(); options != nil && options.hasProperty("cause") {
		obj.defineProperty("cause", options.get("cause"), 0o101, false)
	}
	return obj
}
