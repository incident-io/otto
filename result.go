package otto

type resultKind int

const (
	_ resultKind = iota
	resultReturn
	resultBreak
	resultContinue
)

type result struct {
	value  Value
	target string
	kind   resultKind
}

func newReturnResult(value Value) result {
	return result{kind: resultReturn, value: value, target: ""}
}

func newContinueResult(target string) result {
	return result{kind: resultContinue, value: emptyValue, target: target}
}

func newBreakResult(target string) result {
	return result{kind: resultBreak, value: emptyValue, target: target}
}

// updateEmpty is ES2015's UpdateEmpty: it gives an empty completion, or a break
// or continue with no value yet, the given value.
func updateEmpty(completion, value Value) Value {
	switch completion.kind {
	case valueEmpty:
		return value
	case valueResult:
		r := completion.value.(result)
		if r.kind != resultReturn && r.value.isEmpty() {
			r.value = value
			return toValue(r)
		}
	}
	return completion
}

// resultValue returns the value carried by a return, break or continue.
func (v Value) resultValue() Value {
	return v.value.(result).value
}
