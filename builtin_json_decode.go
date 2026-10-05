package otto

import (
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// maxJSONDepth matches the nesting limit of json.Unmarshal.
const maxJSONDepth = 10000

// jsonDecoder builds JavaScript values from jsontext's token stream, polling
// for interrupts between tokens so that parsing a large document can be
// stopped part way through.
type jsonDecoder struct {
	rt     *runtime
	dec    *jsontext.Decoder
	tokens int
}

func (rt *runtime) parseJSON(src string) Value {
	d := jsonDecoder{rt: rt, dec: jsontext.NewDecoder(
		strings.NewReader(src),
		jsontext.AllowDuplicateNames(true),
		jsontext.AllowInvalidUTF8(true),
	)}
	value := d.value()
	// jsontext reads a stream of values, so check by hand that only
	// whitespace follows the first.
	if rest := strings.TrimLeft(src[d.dec.InputOffset():], " \t\r\n"); rest != "" {
		c, _ := utf8.DecodeRuneInString(rest)
		d.fail(fmt.Errorf("invalid character %q after top-level value", c))
	}
	return value
}

func (d *jsonDecoder) token() jsontext.Token {
	d.tokens++
	d.rt.pollInterrupt(d.tokens)
	tok, err := d.dec.ReadToken()
	if err != nil {
		d.fail(err)
	}
	if d.dec.StackDepth() > maxJSONDepth {
		d.fail(errors.New("exceeded max depth"))
	}
	return tok
}

func (d *jsonDecoder) value() Value {
	tok := d.token()
	switch tok.Kind() {
	case '{':
		obj := d.rt.newObject()
		for d.dec.PeekKind() != '}' {
			name := d.token().String()
			obj.put(name, d.value(), false)
		}
		d.token()
		return objectValue(obj)
	case '[':
		var values []Value
		for d.dec.PeekKind() != ']' {
			d.rt.allocate(allocValueCost)
			values = append(values, d.value())
		}
		d.token()
		return objectValue(d.rt.newArrayOf(values))
	case '"':
		return stringValue(tok.String())
	case '0':
		// A valid JSON number only fails to parse by overflowing, in which
		// case Float returns the correctly signed infinity.
		value, _ := tok.Float()
		return float64Value(value)
	case 't':
		return trueValue
	case 'f':
		return falseValue
	default:
		return nullValue
	}
}

func (d *jsonDecoder) fail(err error) {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		err = errors.New("unexpected end of JSON input")
	}
	panic(d.rt.panicSyntaxError(strings.TrimPrefix(err.Error(), "jsontext: ")))
}
