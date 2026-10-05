package otto

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

// maxJSONDepth matches the nesting limit of json.Unmarshal.
const maxJSONDepth = 10000

// jsonDecoder builds JavaScript values from encoding/json's token stream,
// polling for interrupts between tokens so that parsing a large document can
// be stopped part way through.
type jsonDecoder struct {
	rt     *runtime
	dec    *json.Decoder
	src    string
	tokens int
	depth  int
}

func (rt *runtime) parseJSON(src string) Value {
	d := jsonDecoder{rt: rt, src: src, dec: json.NewDecoder(strings.NewReader(src))}
	d.dec.UseNumber()
	value := d.value(d.token())
	if _, err := d.dec.Token(); !errors.Is(err, io.EOF) {
		d.fail()
	}
	return value
}

func (d *jsonDecoder) token() json.Token {
	d.tokens++
	d.rt.pollInterrupt(d.tokens)
	tok, err := d.dec.Token()
	if err != nil {
		d.fail()
	}
	return tok
}

func (d *jsonDecoder) value(tok json.Token) Value {
	switch tok := tok.(type) {
	case json.Delim:
		d.depth++
		defer func() { d.depth-- }()
		if d.depth > maxJSONDepth {
			d.fail()
		}
		if tok == '{' {
			obj := d.rt.newObject()
			for d.dec.More() {
				name, _ := d.token().(string)
				obj.put(name, d.value(d.token()), false)
			}
			d.token()
			return objectValue(obj)
		}
		var values []Value
		for d.dec.More() {
			d.rt.allocate(allocValueCost)
			values = append(values, d.value(d.token()))
		}
		d.token()
		return objectValue(d.rt.newArrayOf(values))
	case string:
		return stringValue(tok)
	case json.Number:
		// A valid JSON number only fails to parse by overflowing, in which
		// case ParseFloat returns the correctly signed infinity.
		value, _ := strconv.ParseFloat(string(tok), 64)
		return float64Value(value)
	case bool:
		return boolValue(tok)
	default:
		return nullValue
	}
}

// fail throws a SyntaxError with the message json.Unmarshal gives for src.
func (d *jsonDecoder) fail() {
	message := "exceeded max depth"
	if err := json.Unmarshal([]byte(d.src), new(json.RawMessage)); err != nil {
		message = err.Error()
	}
	panic(d.rt.panicSyntaxError(message))
}
