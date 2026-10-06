package otto

import (
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
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

type jsonStringifier struct {
	replacer     *Value
	enc          *jsontext.Encoder
	out          strings.Builder
	propertyList []string
	stack        []*object
	splices      []jsonSplice
	call         FunctionCall
	charged      int64
	spliced      int
}

// jsonSplice is the quoted contents of a long string, which result inserts
// into the empty string the encoder wrote ending at offset at.
type jsonSplice struct {
	quoted []byte
	at     int
}

func builtinJSONStringify(call FunctionCall) Value {
	s := &jsonStringifier{call: call}
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
			s.propertyList = propertyList
		} else if replacer.class == classFunctionName {
			value := objectValue(replacer)
			s.replacer = &value
		}
	}
	var gap string
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
				gap = value[0:10]
			} else {
				gap = value
			}
		case valueNumber:
			value := spaceValue.number().int64
			if value > 10 {
				value = 10
			} else if value < 0 {
				value = 0
			}
			gap = strings.Repeat(" ", int(value))
		}
	}

	// jsontext only indents with spaces and tabs, so indent with spaces and
	// substitute any other gap afterwards.
	indent := gap
	if strings.Trim(gap, " \t") != "" {
		indent = strings.Repeat(" ", len(gap))
	}
	opts := []jsontext.Options{
		jsontext.AllowDuplicateNames(true),
		jsontext.AllowInvalidUTF8(true),
		jsontext.PreserveRawStrings(true),
	}
	if indent != "" {
		opts = append(opts, jsontext.WithIndent(indent))
	}
	s.enc = jsontext.NewEncoder(&s.out, opts...)

	holder := call.runtime.newObject()
	holder.put("", call.Argument(0), false)
	value, marshaler, exists := s.prepare(holder, "")
	if !exists {
		return Value{}
	}
	s.write(value, marshaler)
	out := strings.TrimSuffix(s.result(), "\n")
	if indent != gap {
		out = reindent(call.runtime, out, len(indent), gap)
	}
	return stringValue(out)
}

// prepare returns the value SerializeJSONProperty serializes for holder[key],
// after toJSON and the replacer, or a json.Marshaler for a Go value. It
// reports false if the property isn't serialized at all.
func (s *jsonStringifier) prepare(holder *object, key string) (Value, json.Marshaler, bool) {
	rt := s.call.runtime
	value := holder.get(key)
	if obj := value.object(); obj != nil {
		if toJSON := obj.get("toJSON"); toJSON.IsFunction() {
			value = toJSON.call(rt, value, key)
		} else if obj.objectClass.marshalJSON != nil {
			if marshaler := obj.objectClass.marshalJSON(obj); marshaler != nil {
				return Value{}, marshaler, true
			}
		}
	}
	if s.replacer != nil {
		value = s.replacer.call(rt, objectValue(holder), key, value)
	}
	if value.kind == valueObject {
		switch value.value.(*object).class {
		case classBooleanName:
			value = value.object().value.(Value)
		case classStringName:
			value = stringValue(value.string())
		case classNumberName:
			value = value.numberValue()
		case classFunctionName:
			return Value{}, nil, false
		}
	}
	switch value.kind {
	case valueBoolean, valueString, valueNumber, valueNull, valueObject:
		return value, nil, true
	}
	return Value{}, nil, false
}

// token writes tok, checking the text written so far against the string
// length limit and charging it against the allocation limit.
func (s *jsonStringifier) token(tok jsontext.Token) {
	if err := s.enc.WriteToken(tok); err != nil {
		panic(s.call.runtime.panicTypeError("JSON.stringify: %s", err))
	}
	s.grow()
}

func (s *jsonStringifier) grow() {
	rt := s.call.runtime
	written := s.enc.OutputOffset() + int64(s.spliced)
	rt.checkInterrupt()
	rt.checkStringLength(int(written))
	rt.allocate(written - s.charged)
	s.charged = written
}

// result returns the encoder's output with the long strings spliced in.
func (s *jsonStringifier) result() string {
	out := s.out.String()
	if len(s.splices) == 0 {
		return out
	}
	var result strings.Builder
	result.Grow(len(out) + s.spliced)
	prev := 0
	for _, splice := range s.splices {
		s.call.runtime.checkInterrupt()
		result.WriteString(out[prev:splice.at])
		result.Write(splice.quoted)
		prev = splice.at
	}
	result.WriteString(out[prev:])
	return result.String()
}

// write writes a value returned by prepare.
func (s *jsonStringifier) write(value Value, marshaler json.Marshaler) {
	if marshaler != nil {
		rt := s.call.runtime
		encoded, err := json.Marshal(marshaler)
		if err != nil {
			panic(rt.panicTypeError("JSON.stringify marshal: %s", err))
		}
		rt.checkStringLength(int(s.enc.OutputOffset()) + s.spliced + len(encoded))
		if err = s.enc.WriteValue(encoded); err != nil {
			panic(rt.panicTypeError("JSON.stringify marshal: %s", err))
		}
		s.grow()
		return
	}
	switch value.kind {
	case valueBoolean:
		s.token(jsontext.Bool(value.bool()))
	case valueString:
		s.writeString(value.string())
	case valueNumber:
		number := value.number()
		switch {
		case number.kind == numberInteger:
			s.token(jsontext.Int(number.int64))
		case number.kind != numberFloat || math.IsNaN(number.float64) || math.IsInf(number.float64, 0):
			s.token(jsontext.Null)
		case number.float64 == 0:
			s.token(jsontext.Int(0))
		default:
			s.token(jsontext.Float(number.float64))
		}
	case valueNull:
		s.token(jsontext.Null)
	case valueObject:
		s.writeObject(value.object())
	}
}

// jsonStringChunk is how many bytes of a string writeString quotes between
// checks of the interrupt and limits.
const jsonStringChunk = 1 << 16

// writeString writes str as a JSON string. The encoder can't be interrupted
// part way through a string, so long strings are quoted a chunk at a time
// and spliced into the output afterwards.
func (s *jsonStringifier) writeString(str string) {
	rt := s.call.runtime
	written := int(s.enc.OutputOffset()) + s.spliced
	if len(str) <= jsonStringChunk {
		rt.checkStringLength(written + len(str))
		s.token(jsontext.String(str))
		return
	}
	var quoted []byte
	for len(str) > 0 {
		n := min(len(str), jsonStringChunk)
		for n < len(str) && !utf8.RuneStart(str[n]) {
			n++
		}
		start := len(quoted)
		quoted, _ = jsontext.AppendQuote(quoted, str[:n])
		quoted = append(quoted[:start], quoted[start+1:len(quoted)-1]...)
		str = str[n:]
		rt.checkInterrupt()
		rt.checkStringLength(written + len(quoted) + 2)
	}
	s.token(jsontext.String(""))
	at := int(s.enc.OutputOffset()) - 1
	if s.enc.StackDepth() == 0 {
		// The encoder ends a top-level value with a newline.
		at--
	}
	s.splices = append(s.splices, jsonSplice{at: at, quoted: quoted})
	s.spliced += len(quoted)
	s.grow()
}

func (s *jsonStringifier) writeObject(obj *object) {
	rt := s.call.runtime
	rt.enterNative()
	defer rt.leaveNative()
	if slices.Contains(s.stack, obj) {
		panic(rt.panicTypeError("Converting circular structure to JSON"))
	}
	s.stack = append(s.stack, obj)
	defer func() { s.stack = s.stack[:len(s.stack)-1] }()

	if isArray(obj) {
		var length uint32
		switch value := obj.get(propertyLength).value.(type) {
		case uint32:
			length = value
		case int:
			if value >= 0 {
				length = uint32(value)
			}
		default:
			panic(rt.panicTypeError(fmt.Sprintf("JSON.stringify: invalid length: %v (%[1]T)", value)))
		}
		s.token(jsontext.BeginArray)
		for index := range length {
			value, marshaler, exists := s.prepare(obj, arrayIndexToString(int64(index)))
			if exists {
				s.write(value, marshaler)
			} else {
				s.token(jsontext.Null)
			}
		}
		s.token(jsontext.EndArray)
		return
	}

	s.token(jsontext.BeginObject)
	member := func(name string) bool {
		if value, marshaler, exists := s.prepare(obj, name); exists {
			s.writeString(name)
			s.write(value, marshaler)
		}
		return true
	}
	if s.propertyList != nil {
		for _, name := range s.propertyList {
			member(name)
		}
	} else {
		obj.enumerate(false, member)
	}
	s.token(jsontext.EndObject)
}

// reindent replaces the width-space indentation at the start of each line of
// text with copies of gap. Newlines in strings are escaped, so every newline
// in text starts an indented line.
func reindent(rt *runtime, text string, width int, gap string) string {
	var out strings.Builder
	out.Grow(len(text))
	for line := 0; ; line++ {
		rt.pollInterrupt(line)
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:i+1])
		text = text[i+1:]
		spaces := len(text) - len(strings.TrimLeft(text, " "))
		for range spaces / width {
			out.WriteString(gap)
		}
		text = text[spaces:]
	}
}
