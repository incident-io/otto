package otto

import (
	"encoding/hex"
	"errors"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Global.
func builtinGlobalEval(call FunctionCall) Value {
	src := call.Argument(0)
	if !src.IsString() {
		return src
	}
	rt := call.runtime
	program := rt.cmplParseOrThrow(src.string(), nil)
	if !call.eval {
		// Not a direct call to eval, so we enter the global ExecutionContext
		rt.enterGlobalScope()
		defer rt.leaveScope()
	}
	returnValue := rt.cmplEvaluateNodeProgram(program, true)
	if returnValue.isEmpty() {
		return Value{}
	}
	return returnValue
}

func builtinGlobalIsNaN(call FunctionCall) Value {
	value := call.Argument(0).float64()
	return boolValue(math.IsNaN(value))
}

func builtinGlobalIsFinite(call FunctionCall) Value {
	value := call.Argument(0).float64()
	return boolValue(!math.IsNaN(value) && !math.IsInf(value, 0))
}

func digitValue(chr rune) int {
	switch {
	case '0' <= chr && chr <= '9':
		return int(chr - '0')
	case 'a' <= chr && chr <= 'z':
		return int(chr - 'a' + 10)
	case 'A' <= chr && chr <= 'Z':
		return int(chr - 'A' + 10)
	}
	return 36 // Larger than any legal digit value
}

func builtinGlobalParseInt(call FunctionCall) Value {
	input := strings.Trim(call.Argument(0).string(), builtinStringTrimWhitespace)
	if len(input) == 0 {
		return NaNValue()
	}

	radix := int(toInt32(call.Argument(1)))

	negative := false
	switch input[0] {
	case '+':
		input = input[1:]
	case '-':
		negative = true
		input = input[1:]
	}

	strip := true
	if radix == 0 {
		radix = 10
	} else {
		if radix < 2 || radix > 36 {
			return NaNValue()
		} else if radix != 16 {
			strip = false
		}
	}

	switch len(input) {
	case 0:
		return NaNValue()
	case 1:
	default:
		if strip {
			if input[0] == '0' && (input[1] == 'x' || input[1] == 'X') {
				input = input[2:]
				radix = 16
			}
		}
	}

	base := radix
	index := 0
	for ; index < len(input); index++ {
		digit := digitValue(rune(input[index])) // If not ASCII, then an error anyway
		if digit >= base {
			break
		}
	}
	input = input[0:index]

	value, err := strconv.ParseInt(input, radix, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			base := float64(base)
			// Could just be a very large number (e.g. 0x8000000000000000)
			var value float64
			for _, chr := range input {
				digit := float64(digitValue(chr))
				if digit >= base {
					return NaNValue()
				}
				value = value*base + digit
			}
			if negative {
				value *= -1
			}
			return float64Value(value)
		}
		return NaNValue()
	}
	if negative {
		value *= -1
	}

	return int64Value(value)
}

func builtinGlobalParseFloat(call FunctionCall) Value {
	input := strings.TrimLeft(call.Argument(0).string(), builtinStringTrimWhitespace)
	prefix := parseFloatPrefix(input)
	if prefix == "" {
		return NaNValue()
	}
	// prefix is a valid decimal literal, so ParseFloat can only fail by
	// overflowing, in which case it returns the correctly signed infinity.
	value, _ := strconv.ParseFloat(prefix, 64)
	return float64Value(value)
}

// parseFloatPrefix returns the longest prefix of input that is a
// StrDecimalLiteral, or "" if there is none.
func parseFloatPrefix(input string) string {
	pos := 0
	if pos < len(input) && (input[pos] == '+' || input[pos] == '-') {
		pos++
	}
	if strings.HasPrefix(input[pos:], "Infinity") {
		return input[:pos+len("Infinity")]
	}
	digits := func() int {
		start := pos
		for pos < len(input) && '0' <= input[pos] && input[pos] <= '9' {
			pos++
		}
		return pos - start
	}
	mantissa := digits()
	if pos < len(input) && input[pos] == '.' {
		pos++
		mantissa += digits()
	}
	if mantissa == 0 {
		return ""
	}
	end := pos
	if pos < len(input) && (input[pos] == 'e' || input[pos] == 'E') {
		pos++
		if pos < len(input) && (input[pos] == '+' || input[pos] == '-') {
			pos++
		}
		if digits() > 0 {
			end = pos
		}
	}
	return input[:end]
}

// encodeURI/decodeURI

func encodeDecodeURI(call FunctionCall, unescaped string) Value {
	rt := call.runtime
	var output []byte
	encode := make([]byte, utf8.UTFMax)
	n := 0
	appendRune := func(r rune) {
		if n%interruptEvery == 0 {
			rt.checkInterrupt()
			rt.checkStringLength(len(output))
		}
		n++
		size := utf8.EncodeRune(encode, r)
		for _, b := range encode[0:size] {
			if b < utf8.RuneSelf && (isAlphanumeric(b) || strings.IndexByte(unescaped, b) >= 0) {
				output = append(output, b)
			} else {
				output = append(output, '%', escapeBase16[b>>4], escapeBase16[b&15])
			}
		}
	}

	value := call.Argument(0)
	if input, ok := value.value.([]uint16); ok {
		for index := 0; index < len(input); index++ {
			r := rune(input[index])
			switch {
			case utf16.IsSurrogate(r) && r >= 0xDC00:
				panic(rt.panicURIError("URI malformed"))
			case utf16.IsSurrogate(r):
				index++
				if index >= len(input) {
					panic(rt.panicURIError("URI malformed"))
				}
				r = utf16.DecodeRune(r, rune(input[index]))
				if r == utf8.RuneError {
					panic(rt.panicURIError("URI malformed"))
				}
			}
			appendRune(r)
		}
	} else {
		for _, r := range value.string() {
			appendRune(r)
		}
	}
	rt.allocateString(len(output))
	return stringValue(string(output))
}

func isAlphanumeric(b byte) bool {
	return 'A' <= b && b <= 'Z' || 'a' <= b && b <= 'z' || '0' <= b && b <= '9'
}

// encodeURIUnescaped and encodeURIComponentUnescaped are the non-alphanumeric
// characters that encodeURI and encodeURIComponent leave unescaped.
const (
	encodeURIUnescaped          = "-_.!~*'();/?:@&=+$,#"
	encodeURIComponentUnescaped = "-_.!~*'()"
)

func builtinGlobalEncodeURI(call FunctionCall) Value {
	return encodeDecodeURI(call, encodeURIUnescaped)
}

func builtinGlobalEncodeURIComponent(call FunctionCall) Value {
	return encodeDecodeURI(call, encodeURIComponentUnescaped)
}

// 3B/2F/3F/3A/40/26/3D/2B/24/2C/23.
var decodeURIGuard = regexp.MustCompile(`(?i)(?:%)(3B|2F|3F|3A|40|26|3D|2B|24|2C|23)`)

func decodeURI(input string, reserve bool) (string, bool) {
	if reserve {
		input = decodeURIGuard.ReplaceAllString(input, "%25$1")
	}
	input = strings.ReplaceAll(input, "+", "%2B") // Ugly hack to make QueryUnescape work with our use case
	output, err := url.QueryUnescape(input)
	if err != nil || !utf8.ValidString(output) {
		return "", true
	}
	return output, false
}

func builtinGlobalDecodeURI(call FunctionCall) Value {
	output, err := decodeURI(call.Argument(0).string(), true)
	if err {
		panic(call.runtime.panicURIError("URI malformed"))
	}
	return stringValue(output)
}

func builtinGlobalDecodeURIComponent(call FunctionCall) Value {
	output, err := decodeURI(call.Argument(0).string(), false)
	if err {
		panic(call.runtime.panicURIError("URI malformed"))
	}
	return stringValue(output)
}

// escape/unescape

func builtinShouldEscape(chr byte) bool {
	if 'A' <= chr && chr <= 'Z' || 'a' <= chr && chr <= 'z' || '0' <= chr && chr <= '9' {
		return false
	}
	return !strings.ContainsRune("*_+-./", rune(chr))
}

const escapeBase16 = "0123456789ABCDEF"

func builtinEscape(input string) string {
	return escapeString(input, nil)
}

// escapeString implements escape, calling step, if not nil, with the length of
// the output so far after each input character.
func escapeString(input string, step func(int)) string {
	var output strings.Builder
	length := len(input)
	for index := 0; index < length; {
		if builtinShouldEscape(input[index]) {
			chr, width := utf8.DecodeRuneInString(input[index:])
			chr16 := utf16.Encode([]rune{chr})[0]
			if 256 > chr16 {
				output.Write([]byte{'%', escapeBase16[chr16>>4], escapeBase16[chr16&15]})
			} else {
				output.Write([]byte{
					'%', 'u',
					escapeBase16[chr16>>12],
					escapeBase16[(chr16>>8)&15],
					escapeBase16[(chr16>>4)&15],
					escapeBase16[chr16&15],
				})
			}
			index += width
		} else {
			output.WriteByte(input[index])
			index++
		}
		if step != nil {
			step(output.Len())
		}
	}
	return output.String()
}

func builtinUnescape(input string) string {
	return unescapeString(input, nil)
}

// unescapeString implements unescape, calling step, if not nil, with the length
// of the output so far after each input character.
func unescapeString(input string, step func(int)) string {
	var output strings.Builder
	length := len(input)
	for index := 0; index < length; {
		if step != nil {
			step(output.Len())
		}
		if input[index] == '%' {
			if index <= length-6 && input[index+1] == 'u' {
				byte16, err := hex.DecodeString(input[index+2 : index+6])
				if err == nil {
					value := uint16(byte16[0])<<8 + uint16(byte16[1])
					output.WriteRune(utf16.Decode([]uint16{value})[0])
					index += 6
					continue
				}
			}
			if index <= length-3 {
				byte8, err := hex.DecodeString(input[index+1 : index+3])
				if err == nil {
					output.WriteRune(rune(byte8[0]))
					index += 3
					continue
				}
			}
		}
		output.WriteRune(rune(input[index]))
		index++
	}
	return output.String()
}

// stringStep returns a step function for a native loop building a string,
// which polls for interrupts and enforces the string length limit as the
// output grows.
func (rt *runtime) stringStep() func(int) {
	i := 0
	return func(length int) {
		i++
		rt.pollInterrupt(i)
		if i%interruptEvery == 0 {
			rt.checkStringLength(length)
		}
	}
}

func builtinGlobalEscape(call FunctionCall) Value {
	output := escapeString(call.Argument(0).string(), call.runtime.stringStep())
	call.runtime.allocateString(len(output))
	return stringValue(output)
}

func builtinGlobalUnescape(call FunctionCall) Value {
	output := unescapeString(call.Argument(0).string(), call.runtime.stringStep())
	call.runtime.allocateString(len(output))
	return stringValue(output)
}
