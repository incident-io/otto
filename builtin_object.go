package otto

import (
	"fmt"
	"strconv"
)

// Object

func builtinObject(call FunctionCall) Value {
	value := call.Argument(0)
	switch value.kind {
	case valueUndefined, valueNull:
		return objectValue(call.runtime.newObject())
	}

	return objectValue(call.runtime.toObject(value))
}

func builtinNewObject(obj *object, argumentList []Value) Value {
	value := valueOfArrayIndex(argumentList, 0)
	switch value.kind {
	case valueNull, valueUndefined:
	case valueNumber, valueString, valueBoolean:
		return objectValue(obj.runtime.toObject(value))
	case valueObject:
		return value
	default:
	}
	return objectValue(obj.runtime.newObject())
}

func builtinObjectValueOf(call FunctionCall) Value {
	return objectValue(call.thisObject())
}

func builtinObjectHasOwnProperty(call FunctionCall) Value {
	propertyName := call.Argument(0).string()
	thisObject := call.thisObject()
	return boolValue(thisObject.hasOwnProperty(propertyName))
}

func builtinObjectHasOwn(call FunctionCall) Value {
	obj := call.runtime.toObject(call.Argument(0))
	return boolValue(obj.hasOwnProperty(call.Argument(1).string()))
}

func builtinObjectIsPrototypeOf(call FunctionCall) Value {
	value := call.Argument(0)
	if !value.IsObject() {
		return falseValue
	}
	prototype := call.toObject(value).prototype
	thisObject := call.thisObject()
	for prototype != nil {
		if thisObject == prototype {
			return trueValue
		}
		prototype = prototype.prototype
	}
	return falseValue
}

func builtinObjectPropertyIsEnumerable(call FunctionCall) Value {
	propertyName := call.Argument(0).string()
	thisObject := call.thisObject()
	prop := thisObject.getOwnProperty(propertyName)
	if prop != nil && prop.enumerable() {
		return trueValue
	}
	return falseValue
}

func builtinObjectToString(call FunctionCall) Value {
	var result string
	switch {
	case call.This.IsUndefined():
		result = "[object Undefined]"
	case call.This.IsNull():
		result = "[object Null]"
	default:
		result = fmt.Sprintf("[object %s]", call.thisObject().class)
	}
	return stringValue(result)
}

func builtinObjectToLocaleString(call FunctionCall) Value {
	toString := call.thisObject().get("toString")
	if !toString.isCallable() {
		panic(call.runtime.panicTypeError("Object.toLocaleString %q is not callable", toString))
	}
	return toString.call(call.runtime, call.This)
}

func builtinObjectGetPrototypeOf(call FunctionCall) Value {
	val := call.Argument(0)
	obj := val.object()
	if obj == nil {
		panic(call.runtime.panicTypeError("Object.GetPrototypeOf is nil"))
	}

	if obj.prototype == nil {
		return nullValue
	}

	return objectValue(obj.prototype)
}

func builtinObjectGetOwnPropertyDescriptor(call FunctionCall) Value {
	val := call.Argument(0)
	obj := val.object()
	if obj == nil {
		panic(call.runtime.panicTypeError("Object.GetOwnPropertyDescriptor is nil"))
	}

	name := call.Argument(1).string()
	descriptor := obj.getOwnProperty(name)
	if descriptor == nil {
		return Value{}
	}
	return objectValue(call.runtime.fromPropertyDescriptor(*descriptor))
}

func builtinObjectDefineProperty(call FunctionCall) Value {
	val := call.Argument(0)
	obj := val.object()
	if obj == nil {
		panic(call.runtime.panicTypeError("Object.DefineProperty is nil"))
	}
	name := call.Argument(1).string()
	descriptor := toPropertyDescriptor(call.runtime, call.Argument(2))
	obj.defineOwnProperty(name, descriptor, true)
	return val
}

func builtinObjectDefineProperties(call FunctionCall) Value {
	val := call.Argument(0)
	obj := val.object()
	if obj == nil {
		panic(call.runtime.panicTypeError("Object.DefineProperties is nil"))
	}

	properties := call.runtime.toObject(call.Argument(1))
	properties.enumerate(false, func(name string) bool {
		descriptor := toPropertyDescriptor(call.runtime, properties.get(name))
		obj.defineOwnProperty(name, descriptor, true)
		return true
	})

	return val
}

func builtinObjectCreate(call FunctionCall) Value {
	prototypeValue := call.Argument(0)
	if !prototypeValue.IsNull() && !prototypeValue.IsObject() {
		panic(call.runtime.panicTypeError("Object.Create is nil"))
	}

	obj := call.runtime.newObject()
	obj.prototype = prototypeValue.object()

	propertiesValue := call.Argument(1)
	if propertiesValue.IsDefined() {
		properties := call.runtime.toObject(propertiesValue)
		properties.enumerate(false, func(name string) bool {
			descriptor := toPropertyDescriptor(call.runtime, properties.get(name))
			obj.defineOwnProperty(name, descriptor, true)
			return true
		})
	}

	return objectValue(obj)
}

func builtinObjectIsExtensible(call FunctionCall) Value {
	val := call.Argument(0)
	if obj := val.object(); obj != nil {
		return boolValue(obj.extensible)
	}
	panic(call.runtime.panicTypeError("Object.IsExtensible is nil"))
}

func builtinObjectPreventExtensions(call FunctionCall) Value {
	val := call.Argument(0)
	if obj := val.object(); obj != nil {
		obj.extensible = false
		return val
	}
	panic(call.runtime.panicTypeError("Object.PreventExtensions is nil"))
}

func builtinObjectAssign(call FunctionCall) Value {
	target := call.Argument(0)
	if target.IsUndefined() || target.IsNull() {
		panic(call.runtime.panicTypeError("Object.assign TypeError: Cannot convert undefined or null to object"))
	}
	targetObj := call.runtime.toObject(target)

	for _, source := range call.ArgumentList[1:] {
		if source.IsUndefined() || source.IsNull() {
			continue
		}
		sourceObj := call.runtime.toObject(source)
		sourceObj.enumerate(false, func(name string) bool {
			targetObj.put(name, sourceObj.get(name), true)
			return true
		})
	}

	return objectValue(targetObj)
}

func builtinObjectIsSealed(call FunctionCall) Value {
	val := call.Argument(0)
	if obj := val.object(); obj != nil {
		if obj.extensible {
			return boolValue(false)
		}
		result := true
		obj.enumerate(true, func(name string) bool {
			prop := obj.getProperty(name)
			if prop.configurable() {
				result = false
			}
			return true
		})
		return boolValue(result)
	}
	panic(call.runtime.panicTypeError("Object.IsSealed is nil"))
}

func builtinObjectSeal(call FunctionCall) Value {
	val := call.Argument(0)
	if obj := val.object(); obj != nil {
		obj.enumerate(true, func(name string) bool {
			if prop := obj.getOwnProperty(name); nil != prop && prop.configurable() {
				prop.configureOff()
				obj.defineOwnProperty(name, *prop, true)
			}
			return true
		})
		obj.extensible = false
		return val
	}
	panic(call.runtime.panicTypeError("Object.Seal is nil"))
}

func builtinObjectIsFrozen(call FunctionCall) Value {
	val := call.Argument(0)
	if obj := val.object(); obj != nil {
		if obj.extensible {
			return boolValue(false)
		}
		result := true
		obj.enumerate(true, func(name string) bool {
			prop := obj.getProperty(name)
			if prop.configurable() || prop.writable() {
				result = false
			}
			return true
		})
		return boolValue(result)
	}
	panic(call.runtime.panicTypeError("Object.IsFrozen is nil"))
}

func builtinObjectFreeze(call FunctionCall) Value {
	val := call.Argument(0)
	if obj := val.object(); obj != nil {
		obj.enumerate(true, func(name string) bool {
			if prop, update := obj.getOwnProperty(name), false; nil != prop {
				if prop.isDataDescriptor() && prop.writable() {
					prop.writeOff()
					update = true
				}
				if prop.configurable() {
					prop.configureOff()
					update = true
				}
				if update {
					obj.defineOwnProperty(name, *prop, true)
				}
			}
			return true
		})
		obj.extensible = false
		return val
	}
	panic(call.runtime.panicTypeError("Object.Freeze is nil"))
}

func builtinObjectKeys(call FunctionCall) Value {
	if obj, keys := call.Argument(0).object(), []Value(nil); nil != obj {
		obj.enumerate(false, func(name string) bool {
			keys = append(keys, stringValue(name))
			return true
		})
		return objectValue(call.runtime.newArrayOf(keys))
	}
	panic(call.runtime.panicTypeError("Object.Keys is nil"))
}

func builtinObjectValues(call FunctionCall) Value {
	if obj, values := call.Argument(0).object(), []Value(nil); nil != obj {
		obj.enumerate(false, func(name string) bool {
			values = append(values, obj.get(name))
			return true
		})
		return objectValue(call.runtime.newArrayOf(values))
	}
	panic(call.runtime.panicTypeError("Object.Values is nil"))
}

func builtinObjectEntries(call FunctionCall) Value {
	if obj, entries := call.Argument(0).object(), []Value(nil); nil != obj {
		// Own enumerable string-keyed properties, in the same order as
		// Object.keys/Object.values.
		obj.enumerate(false, func(name string) bool {
			entry := call.runtime.newArrayOf([]Value{stringValue(name), obj.get(name)})
			entries = append(entries, objectValue(entry))
			return true
		})
		return objectValue(call.runtime.newArrayOf(entries))
	}
	panic(call.runtime.panicTypeError("Object.Entries is nil"))
}

func builtinObjectIs(call FunctionCall) Value {
	// SameValue(x, y) - like === except NaN === NaN and +0 !== -0.
	return boolValue(sameValue(call.Argument(0), call.Argument(1)))
}

func builtinObjectFromEntries(call FunctionCall) Value {
	// NOTE: A fully spec-compliant implementation iterates the argument via its
	// Symbol.iterator. otto does not implement Symbol.iterator, so we accept any
	// array-like list of [key, value] pairs, which covers the common case
	// (e.g. arrays and Map-like array-of-entries). General iterables are not
	// supported.
	iterable := call.Argument(0)
	if !iterable.IsObject() {
		panic(call.runtime.panicTypeError("Object.fromEntries requires an array-like argument"))
	}
	source := iterable.object()
	length := objectLength(source)

	result := call.runtime.newObject()
	for index := range length {
		call.runtime.checkInterrupt()
		entryValue := source.get(strconv.FormatUint(uint64(index), 10))
		if !entryValue.IsObject() {
			panic(call.runtime.panicTypeError("Object.fromEntries entry is not an object"))
		}
		entry := entryValue.object()
		key := entry.get("0").string()
		value := entry.get("1")
		result.put(key, value, true)
	}

	return objectValue(result)
}

func builtinObjectSetPrototypeOf(call FunctionCall) Value {
	val := call.Argument(0)
	switch val.kind {
	case valueUndefined, valueNull:
		panic(call.runtime.panicTypeError("Object.setPrototypeOf called on null or undefined"))
	}

	proto := call.Argument(1)
	if !proto.IsObject() && !proto.IsNull() {
		panic(call.runtime.panicTypeError("Object.setPrototypeOf: prototype must be an object or null"))
	}

	// For non-object values (primitives) the prototype cannot change, but per
	// spec the value is returned unchanged.
	obj := val.object()
	if obj != nil && !obj.setPrototype(proto.object()) {
		panic(call.runtime.panicTypeError("Cyclic __proto__ value"))
	}

	return val
}

func builtinObjectGetOwnPropertyDescriptors(call FunctionCall) Value {
	val := call.Argument(0)
	obj := val.object()
	if obj == nil {
		panic(call.runtime.panicTypeError("Object.GetOwnPropertyDescriptors is nil"))
	}

	result := call.runtime.newObject()
	obj.enumerate(true, func(name string) bool {
		if descriptor := obj.getOwnProperty(name); descriptor != nil {
			result.put(name, objectValue(call.runtime.fromPropertyDescriptor(*descriptor)), true)
		}
		return true
	})

	return objectValue(result)
}

func builtinObjectGetOwnPropertyNames(call FunctionCall) Value {
	if obj, propertyNames := call.Argument(0).object(), []Value(nil); nil != obj {
		obj.enumerate(true, func(name string) bool {
			if obj.hasOwnProperty(name) {
				propertyNames = append(propertyNames, stringValue(name))
			}
			return true
		})
		return objectValue(call.runtime.newArrayOf(propertyNames))
	}

	// Default to empty array for non object types.
	return objectValue(call.runtime.newArray(0))
}

func builtinObjectGroupBy(call FunctionCall) Value {
	itemsValue := call.Argument(0)
	callback := call.Argument(1)
	if !callback.isCallable() {
		panic(call.runtime.panicTypeError("Object.groupBy %q is not a function", callback))
	}
	var items *object
	switch {
	case itemsValue.IsString():
		items = call.runtime.newArrayOf(stringCodePoints(call.runtime, itemsValue.string()))
	case isArray(itemsValue.object()):
		items = itemsValue.object()
	default:
		panic(call.runtime.panicTypeError("Object.groupBy %q is not iterable", itemsValue))
	}

	result := call.runtime.newObject()
	result.prototype = nil
	groups := map[string]*object{}
	length := int64(toUint32(items.get(propertyLength)))
	for index := range length {
		call.runtime.checkInterrupt()
		value := items.get(arrayIndexToString(index))
		key := callback.call(call.runtime, Value{}, value, index).string()
		group, exists := groups[key]
		if !exists {
			call.runtime.allocate(allocObjectCost + allocPropertyCost)
			group = call.runtime.newArray(0)
			groups[key] = group
			result.put(key, objectValue(group), true)
		}
		call.runtime.allocate(allocPropertyCost)
		group.put(arrayIndexToString(int64(toUint32(group.get(propertyLength)))), value, true)
	}
	return objectValue(result)
}

// stringCodePoints splits str into its code points, as string iteration does.
func stringCodePoints(rt *runtime, str string) []Value {
	rt.allocateDense(int64(len(str)), allocValueCost)
	values := make([]Value, 0, len(str))
	for _, chr := range str {
		rt.pollInterrupt(len(values))
		values = append(values, stringValue(string(chr)))
	}
	return values
}
