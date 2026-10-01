package otto

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// jsonParser decodes JSON text directly into JavaScript values. Unlike
// encoding/json it polls for interrupts, accounts for its allocations and
// bounds its nesting depth as it goes, so hostile input cannot run past a
// deadline or exhaust memory before the runtime gets a chance to stop it.
type jsonParser struct {
	rt    *runtime
	src   string
	pos   int
	depth int
	steps int
}

// maxJSONDepth matches the nesting limit of encoding/json.
const maxJSONDepth = 10000

type jsonSyntaxError struct{}

func (rt *runtime) parseJSON(src string) Value {
	p := jsonParser{rt: rt, src: src}
	value, ok := p.parseDocument()
	if !ok {
		message := "invalid JSON"
		if err := json.Unmarshal([]byte(src), new(json.RawMessage)); err != nil {
			message = err.Error()
		} else if p.depth > maxJSONDepth {
			message = "exceeded max depth"
		}
		panic(rt.panicSyntaxError(message))
	}
	return value
}

func (p *jsonParser) parseDocument() (value Value, ok bool) { //nolint:nonamedreturns
	defer func() {
		if caught := recover(); caught != nil {
			if _, isSyntax := caught.(jsonSyntaxError); !isSyntax {
				panic(caught)
			}
			value, ok = Value{}, false
		}
	}()
	p.skipSpace()
	value = p.parseValue()
	p.skipSpace()
	if p.pos != len(p.src) {
		p.fail()
	}
	return value, true
}

func (p *jsonParser) fail() {
	panic(jsonSyntaxError{})
}

func (p *jsonParser) tick() {
	p.steps++
	p.rt.pollInterrupt(p.steps)
}

func (p *jsonParser) skipSpace() {
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *jsonParser) expect(literal string) {
	if !strings.HasPrefix(p.src[p.pos:], literal) {
		p.fail()
	}
	p.pos += len(literal)
}

func (p *jsonParser) parseValue() Value {
	p.tick()
	if p.pos >= len(p.src) {
		p.fail()
	}
	switch c := p.src[p.pos]; {
	case c == '{':
		return p.parseObject()
	case c == '[':
		return p.parseArray()
	case c == '"':
		return stringValue(p.parseString())
	case c == 't':
		p.expect("true")
		return trueValue
	case c == 'f':
		p.expect("false")
		return falseValue
	case c == 'n':
		p.expect("null")
		return nullValue
	case c == '-' || ('0' <= c && c <= '9'):
		return p.parseNumber()
	}
	p.fail()
	return Value{}
}

func (p *jsonParser) enter() {
	p.depth++
	if p.depth > maxJSONDepth {
		p.fail()
	}
}

func (p *jsonParser) parseObject() Value {
	p.enter()
	p.pos++
	obj := p.rt.newObject()
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == '}' {
		p.pos++
		p.depth--
		return objectValue(obj)
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != '"' {
			p.fail()
		}
		name := p.parseString()
		p.skipSpace()
		p.expect(":")
		p.skipSpace()
		value := p.parseValue()
		obj.put(name, value, false)
		p.skipSpace()
		if p.pos >= len(p.src) {
			p.fail()
		}
		switch p.src[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			p.depth--
			return objectValue(obj)
		default:
			p.fail()
		}
	}
}

func (p *jsonParser) parseArray() Value {
	p.enter()
	p.pos++
	var values []Value
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == ']' {
		p.pos++
		p.depth--
		return objectValue(p.rt.newArrayOf(values))
	}
	for {
		p.skipSpace()
		p.rt.allocate(allocValueCost)
		values = append(values, p.parseValue())
		p.skipSpace()
		if p.pos >= len(p.src) {
			p.fail()
		}
		switch p.src[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			p.depth--
			return objectValue(p.rt.newArrayOf(values))
		default:
			p.fail()
		}
	}
}

func (p *jsonParser) digits() int {
	start := p.pos
	for p.pos < len(p.src) && '0' <= p.src[p.pos] && p.src[p.pos] <= '9' {
		p.pos++
	}
	return p.pos - start
}

func (p *jsonParser) parseNumber() Value {
	start := p.pos
	if p.src[p.pos] == '-' {
		p.pos++
	}
	if p.pos < len(p.src) && p.src[p.pos] == '0' {
		p.pos++
	} else if p.digits() == 0 {
		p.fail()
	}
	if p.pos < len(p.src) && p.src[p.pos] == '.' {
		p.pos++
		if p.digits() == 0 {
			p.fail()
		}
	}
	if p.pos < len(p.src) && (p.src[p.pos] == 'e' || p.src[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.src) && (p.src[p.pos] == '+' || p.src[p.pos] == '-') {
			p.pos++
		}
		if p.digits() == 0 {
			p.fail()
		}
	}
	// A syntactically valid number only fails to parse by overflowing, in
	// which case ParseFloat returns the correctly signed infinity.
	value, _ := strconv.ParseFloat(p.src[start:p.pos], 64)
	return float64Value(value)
}

func (p *jsonParser) parseString() string {
	p.pos++
	start := p.pos
	// Fast path: no escapes and valid UTF-8.
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '"' {
			s := p.src[start:p.pos]
			if utf8.ValidString(s) {
				p.pos++
				return s
			}
			break
		}
		if c == '\\' || c < 0x20 {
			break
		}
		p.pos++
	}
	p.pos = start

	var b strings.Builder
	for {
		if p.pos >= len(p.src) {
			p.fail()
		}
		if p.pos%interruptEvery == 0 {
			p.rt.checkInterrupt()
		}
		c := p.src[p.pos]
		switch {
		case c == '"':
			p.pos++
			return b.String()
		case c < 0x20:
			p.fail()
		case c == '\\':
			p.pos++
			if p.pos >= len(p.src) {
				p.fail()
			}
			esc := p.src[p.pos]
			p.pos++
			switch esc {
			case '"', '\\', '/':
				b.WriteByte(esc)
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				r := p.hex4()
				if utf16.IsSurrogate(r) {
					r2 := rune(-1)
					if strings.HasPrefix(p.src[p.pos:], `\u`) {
						save := p.pos
						p.pos += 2
						r2 = p.hex4()
						if utf16.DecodeRune(r, r2) == utf8.RuneError {
							p.pos = save
							r2 = -1
						}
					}
					if r2 < 0 {
						r = utf8.RuneError
					} else {
						r = utf16.DecodeRune(r, r2)
					}
				}
				b.WriteRune(r)
			default:
				p.fail()
			}
		default:
			r, size := utf8.DecodeRuneInString(p.src[p.pos:])
			b.WriteRune(r)
			p.pos += size
		}
	}
}

func (p *jsonParser) hex4() rune {
	if p.pos+4 > len(p.src) {
		p.fail()
	}
	value, err := strconv.ParseUint(p.src[p.pos:p.pos+4], 16, 32)
	if err != nil {
		p.fail()
	}
	p.pos += 4
	return rune(value)
}
