package otto

import "unicode/utf8"

type referencer interface {
	invalid() bool               // IsUnresolvableReference
	getValue() Value             // getValue
	putValue(value Value) string // PutValue
	delete() bool
}

// PropertyReference

type propertyReference struct {
	base    *object
	runtime *runtime
	name    string
	at      at
	strict  bool
}

func newPropertyReference(rt *runtime, base *object, name string, strict bool, atv at) *propertyReference {
	return &propertyReference{
		runtime: rt,
		name:    name,
		strict:  strict,
		base:    base,
		at:      atv,
	}
}

func (pr *propertyReference) invalid() bool {
	return pr.base == nil
}

func (pr *propertyReference) getValue() Value {
	if pr.base == nil {
		panic(pr.runtime.panicReferenceError("'%s' is not defined", pr.name, pr.at))
	}
	return pr.base.get(pr.name)
}

func (pr *propertyReference) putValue(value Value) string {
	if pr.base == nil {
		return pr.name
	}
	pr.base.put(pr.name, value, pr.strict)
	return ""
}

func (pr *propertyReference) delete() bool {
	if pr.base == nil {
		// TODO Throw an error if strict
		return true
	}
	return pr.base.delete(pr.name, pr.strict)
}

type stashReference struct {
	base   stasher
	name   string
	strict bool
}

func (sr *stashReference) invalid() bool {
	return false // The base (an environment) will never be nil
}

func (sr *stashReference) getValue() Value {
	return sr.base.getBinding(sr.name, sr.strict)
}

func (sr *stashReference) putValue(value Value) string {
	sr.base.setValue(sr.name, value, sr.strict)
	return ""
}

func (sr *stashReference) delete() bool {
	if sr.base == nil {
		// This should never be reached, but just in case
		return false
	}
	return sr.base.deleteBinding(sr.name)
}

// getIdentifierReference.
func getIdentifierReference(rt *runtime, stash stasher, name string, strict bool, atv at) referencer {
	if stash == nil {
		return newPropertyReference(rt, nil, name, strict, atv)
	}
	if stash.hasBinding(name) {
		return stash.newReference(name, strict, atv)
	}
	return getIdentifierReference(rt, stash.outer(), name, strict, atv)
}

// stringReference is a reference to a property of a primitive string. It
// reads the string's own properties, and data properties it inherits, without
// boxing the string in a String object.
type stringReference struct {
	runtime *runtime
	name    string
	base    Value
	strict  bool
	at      at
}

func (sr *stringReference) invalid() bool {
	return false
}

func (sr *stringReference) getValue() Value {
	rt := sr.runtime
	if sr.name == propertyLength {
		return intValue(rt.stringObjecter(sr.base.string()).Length())
	}
	if index := stringToArrayIndex(sr.name); index >= 0 {
		if chr := stringAt(rt.stringObjecter(sr.base.string()), int(index)); chr != utf8.RuneError {
			return stringValue(string(chr))
		}
	}
	prop := rt.global.StringPrototype.getProperty(sr.name)
	if prop == nil {
		return Value{}
	}
	if value, ok := prop.value.(Value); ok {
		return value
	}
	return prop.get(sr.object())
}

func (sr *stringReference) putValue(value Value) string {
	return newPropertyReference(sr.runtime, sr.object(), sr.name, sr.strict, sr.at).putValue(value)
}

func (sr *stringReference) delete() bool {
	return newPropertyReference(sr.runtime, sr.object(), sr.name, sr.strict, sr.at).delete()
}

// object boxes the string.
func (sr *stringReference) object() *object {
	return sr.runtime.newString(sr.base)
}

// this returns the this value for a call of fn through the reference. Built-in
// functions coerce this themselves, so they are passed the primitive string;
// other functions are passed it boxed, as non-strict code expects.
func (sr *stringReference) this(fn Value) Value {
	if obj := fn.object(); obj != nil {
		if _, ok := obj.value.(nativeFunctionObject); ok {
			return sr.base
		}
	}
	return objectValue(sr.object())
}
