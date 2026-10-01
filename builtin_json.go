package otto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type builtinJSONParseContext struct {
	reviver Value
	call    FunctionCall
}

func builtinJSONParse(call FunctionCall) Value {
	ctx := builtinJSONParseContext{
		call: call,
	}
	revive := false
	if reviver := call.Argument(1); reviver.isCallable() {
		revive = true
		ctx.reviver = reviver
	}

	value := call.runtime.parseJSON(call.Argument(0).string())
	if revive {
		root := ctx.call.runtime.newObject()
		root.put("", value, false)
		return builtinJSONReviveWalk(ctx, root, "")
	}
	return value
}

func builtinJSONReviveWalk(ctx builtinJSONParseContext, holder *object, name string) Value {
	ctx.call.runtime.enterNative()
	defer ctx.call.runtime.leaveNative()
	value := holder.get(name)
	if obj := value.object(); obj != nil {
		if isArray(obj) {
			length := int64(objectLength(obj))
			for index := range length {
				ctx.call.runtime.checkInterrupt()
				idxName := arrayIndexToString(index)
				idxValue := builtinJSONReviveWalk(ctx, obj, idxName)
				if idxValue.IsUndefined() {
					obj.delete(idxName, false)
				} else {
					obj.defineProperty(idxName, idxValue, 0o111, false)
				}
			}
		} else {
			obj.enumerate(false, func(name string) bool {
				enumVal := builtinJSONReviveWalk(ctx, obj, name)
				if enumVal.IsUndefined() {
					obj.delete(name, false)
				} else {
					obj.defineProperty(name, enumVal, 0o111, false)
				}
				return true
			})
		}
	}
	return ctx.reviver.call(ctx.call.runtime, objectValue(holder), name, value)
}

type builtinJSONStringifyContext struct {
	replacerFunction *Value
	size             *int
	gap              string
	stack            []*object
	propertyList     []string
	call             FunctionCall
}

// grow adds an estimate of the encoded size of a value to the running total
// so that the string length limit is enforced before marshalling.
func (ctx builtinJSONStringifyContext) grow(size int) {
	*ctx.size += size
	ctx.call.runtime.checkStringLength(*ctx.size)
	ctx.call.runtime.allocate(int64(size))
}

func builtinJSONStringify(call FunctionCall) Value {
	ctx := builtinJSONStringifyContext{
		call:  call,
		stack: []*object{nil},
		size:  new(int),
	}
	replacer := call.Argument(1).object()
	if replacer != nil {
		if isArray(replacer) {
			length := objectLength(replacer)
			seen := map[string]bool{}
			propertyList := make([]string, 0, preallocation(int64(length)))
			for index := range length {
				call.runtime.checkInterrupt()
				value := replacer.get(arrayIndexToString(int64(index)))
				switch value.kind {
				case valueObject:
					switch value.value.(*object).class {
					case classStringName, classNumberName:
					default:
						continue
					}
				case valueString, valueNumber:
				default:
					continue
				}
				name := value.string()
				if seen[name] {
					continue
				}
				seen[name] = true
				propertyList = append(propertyList, name)
			}
			ctx.propertyList = propertyList
		} else if replacer.class == classFunctionName {
			value := objectValue(replacer)
			ctx.replacerFunction = &value
		}
	}
	if spaceValue, exists := call.getArgument(2); exists {
		if spaceValue.kind == valueObject {
			switch spaceValue.value.(*object).class {
			case classStringName:
				spaceValue = stringValue(spaceValue.string())
			case classNumberName:
				spaceValue = spaceValue.numberValue()
			}
		}
		switch spaceValue.kind {
		case valueString:
			value := spaceValue.string()
			if len(value) > 10 {
				ctx.gap = value[0:10]
			} else {
				ctx.gap = value
			}
		case valueNumber:
			value := spaceValue.number().int64
			if value > 10 {
				value = 10
			} else if value < 0 {
				value = 0
			}
			ctx.gap = strings.Repeat(" ", int(value))
		}
	}
	holder := call.runtime.newObject()
	holder.put("", call.Argument(0), false)
	value, exists := builtinJSONStringifyWalk(ctx, "", holder)
	if !exists {
		return Value{}
	}
	encoder := jsonEncoder{rt: call.runtime, gap: ctx.gap}
	if err := encoder.encode(value, ""); err != nil {
		panic(call.runtime.panicTypeError("JSON.stringify marshal: %s", err))
	}
	return stringValue(encoder.out.String())
}

func builtinJSONStringifyWalk(ctx builtinJSONStringifyContext, key string, holder *object) (interface{}, bool) {
	ctx.call.runtime.enterNative()
	defer ctx.call.runtime.leaveNative()
	value := holder.get(key)

	if value.IsObject() {
		obj := value.object()
		if toJSON := obj.get("toJSON"); toJSON.IsFunction() {
			value = toJSON.call(ctx.call.runtime, value, key)
		} else if obj.objectClass.marshalJSON != nil {
			// If the object is a GoStruct or something that implements json.Marshaler
			marshaler := obj.objectClass.marshalJSON(obj)
			if marshaler != nil {
				return marshaler, true
			}
		}
	}

	if ctx.replacerFunction != nil {
		value = ctx.replacerFunction.call(ctx.call.runtime, objectValue(holder), key, value)
	}

	if value.kind == valueObject {
		switch value.value.(*object).class {
		case classBooleanName:
			value = value.object().value.(Value)
		case classStringName:
			value = stringValue(value.string())
		case classNumberName:
			value = value.numberValue()
		}
	}

	switch value.kind {
	case valueBoolean:
		ctx.grow(len(key) + 5)
		return value.bool(), true
	case valueString:
		str := value.string()
		ctx.grow(len(key) + len(str))
		return str, true
	case valueNumber:
		ctx.grow(len(key) + 1)
		integer := value.number()
		switch integer.kind {
		case numberInteger:
			return integer.int64, true
		case numberFloat:
			return integer.float64, true
		default:
			return nil, true
		}
	case valueNull:
		ctx.grow(len(key) + 4)
		return nil, true
	case valueObject:
		ctx.grow(len(key) + 2)
		objHolder := value.object()
		if value := value.object(); nil != value {
			for _, obj := range ctx.stack {
				if objHolder == obj {
					panic(ctx.call.runtime.panicTypeError("Converting circular structure to JSON"))
				}
			}
			ctx.stack = append(ctx.stack, value)
			defer func() { ctx.stack = ctx.stack[:len(ctx.stack)-1] }()
		}
		if isArray(objHolder) {
			var length uint32
			switch value := objHolder.get(propertyLength).value.(type) {
			case uint32:
				length = value
			case int:
				if value >= 0 {
					length = uint32(value)
				}
			default:
				panic(ctx.call.runtime.panicTypeError(fmt.Sprintf("JSON.stringify: invalid length: %v (%[1]T)", value)))
			}
			array := make([]interface{}, 0, preallocation(int64(length)))
			for index := range length {
				ctx.call.runtime.checkInterrupt()
				name := arrayIndexToString(int64(index))
				value, _ := builtinJSONStringifyWalk(ctx, name, objHolder)
				array = append(array, value)
			}
			return array, true
		} else if objHolder.class != classFunctionName {
			obj := &jsonObject{}
			if ctx.propertyList != nil {
				for _, name := range ctx.propertyList {
					value, exists := builtinJSONStringifyWalk(ctx, name, objHolder)
					if exists {
						obj.set(name, value)
					}
				}
			} else {
				objHolder.enumerate(false, func(name string) bool {
					value, exists := builtinJSONStringifyWalk(ctx, name, objHolder)
					if exists {
						obj.set(name, value)
					}
					return true
				})
			}
			return obj, true
		}
	}
	return nil, false
}

// jsonObject is an object being stringified, keeping its keys in property
// order.
type jsonObject struct {
	index  map[string]int
	keys   []string
	values []interface{}
}

func (o *jsonObject) set(key string, value interface{}) {
	if i, exists := o.index[key]; exists {
		o.values[i] = value
		return
	}
	if o.index == nil {
		o.index = map[string]int{}
	}
	o.index[key] = len(o.keys)
	o.keys = append(o.keys, key)
	o.values = append(o.values, value)
}

// jsonEncoder writes values, as built by builtinJSONStringifyWalk, as JSON
// text following SerializeJSONProperty, enforcing the string length limit as
// the text grows.
type jsonEncoder struct {
	gap     string
	rt      *runtime
	out     strings.Builder
	charged int
}

// grow checks the text written so far against the string length limit and
// charges it against the allocation limit.
func (e *jsonEncoder) grow() {
	e.rt.checkInterrupt()
	e.rt.checkStringLength(e.out.Len())
	e.rt.allocate(int64(e.out.Len() - e.charged))
	e.charged = e.out.Len()
}

// separate writes what precedes the element at index i of an array or
// object whose members are indented by indent.
func (e *jsonEncoder) separate(i int, indent string) {
	if i > 0 {
		e.out.WriteByte(',')
	}
	if e.gap != "" {
		e.out.WriteByte('\n')
		e.out.WriteString(indent)
	}
	e.grow()
}

// close writes the end of a non-empty array or object, given the indent of
// its opening line.
func (e *jsonEncoder) close(indent string, end byte) {
	if e.gap != "" {
		e.out.WriteByte('\n')
		e.out.WriteString(indent)
	}
	e.out.WriteByte(end)
}

func (e *jsonEncoder) encode(value interface{}, indent string) error {
	switch value := value.(type) {
	case nil:
		e.out.WriteString("null")
	case bool:
		e.out.WriteString(strconv.FormatBool(value))
	case int64:
		e.out.WriteString(strconv.FormatInt(value, 10))
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			e.out.WriteString("null")
		} else {
			e.out.WriteString(floatToString(value, 64))
		}
	case string:
		quoteJSONString(e.rt, &e.out, value)
	case []interface{}:
		if len(value) == 0 {
			e.out.WriteString("[]")
			break
		}
		inner := indent + e.gap
		e.out.WriteByte('[')
		for i, element := range value {
			e.separate(i, inner)
			if err := e.encode(element, inner); err != nil {
				return err
			}
		}
		e.close(indent, ']')
	case *jsonObject:
		if len(value.keys) == 0 {
			e.out.WriteString("{}")
			break
		}
		inner := indent + e.gap
		e.out.WriteByte('{')
		for i, key := range value.keys {
			e.separate(i, inner)
			quoteJSONString(e.rt, &e.out, key)
			e.out.WriteByte(':')
			if e.gap != "" {
				e.out.WriteByte(' ')
			}
			if err := e.encode(value.values[i], inner); err != nil {
				return err
			}
		}
		e.close(indent, '}')
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if e.gap != "" {
			indented := bytes.Buffer{}
			if indentErr := json.Indent(&indented, encoded, indent, e.gap); indentErr != nil {
				return indentErr
			}
			encoded = indented.Bytes()
		}
		e.out.Write(encoded)
	}
	e.grow()
	return nil
}

// quoteJSONString implements QuoteJSONString.
func quoteJSONString(rt *runtime, out *strings.Builder, value string) {
	out.WriteByte('"')
	for i, chr := range value {
		if i&0xfff == 0 {
			rt.checkInterrupt()
		}
		switch chr {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if chr < 0x20 || (chr >= 0xd800 && chr <= 0xdfff) {
				fmt.Fprintf(out, `\u%04x`, chr)
			} else {
				out.WriteRune(chr)
			}
		}
	}
	out.WriteByte('"')
}
