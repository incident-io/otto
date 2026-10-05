package otto

import (
	"fmt"
	"math"
	goruntime "runtime"
	"strings"

	"github.com/robertkrimen/otto/token"
)

func (rt *runtime) cmplEvaluateNodeExpression(node nodeExpression) Value {
	// Allow interpreter interruption
	// If the Interrupt channel is nil, then
	// we avoid runtime.Gosched() overhead (if any)
	// FIXME: Test this
	if rt.otto.Interrupt != nil {
		goruntime.Gosched()
		rt.checkInterrupt()
	}

	rt.enterNative()
	value := rt.cmplEvaluateNodeExpressionInner(node)
	rt.leaveNative()
	return value
}

func (rt *runtime) cmplEvaluateNodeExpressionInner(node nodeExpression) Value {
	switch node := node.(type) {
	case *nodeArrayLiteral:
		return rt.cmplEvaluateNodeArrayLiteral(node)

	case *nodeAssignExpression:
		return rt.cmplEvaluateNodeAssignExpression(node)

	case *nodeBinaryExpression:
		if node.comparison {
			return rt.cmplEvaluateNodeBinaryExpressionComparison(node)
		}
		return rt.cmplEvaluateNodeBinaryExpression(node)

	case *nodeBracketExpression:
		return rt.cmplEvaluateNodeBracketExpression(node)

	case *nodeCallExpression:
		return rt.cmplEvaluateNodeCallExpression(node, nil)

	case *nodeConditionalExpression:
		return rt.cmplEvaluateNodeConditionalExpression(node)

	case *nodeDotExpression:
		return rt.cmplEvaluateNodeDotExpression(node)

	case *nodeOptionalChain:
		value, _ := rt.evaluateChainLink(node.expression)
		return value

	case *nodeFunctionLiteral:
		local := rt.scope.lexical
		if node.name != "" {
			local = rt.newDeclarationStash(local)
		}

		fnObj := rt.newNodeFunction(node, local)
		if node.isArrow {
			// Capture the enclosing `this` for lexical binding at call time.
			fn := fnObj.value.(nodeFunctionObject)
			fn.this = objectValue(rt.scope.this)
			fnObj.value = fn
		}
		value := objectValue(fnObj)
		if node.name != "" {
			local.createBinding(node.name, false, value)
		}
		return value

	case *nodeIdentifier:
		name := node.name
		// TODO Should be true or false (strictness) depending on context
		// getIdentifierReference should not return nil, but we check anyway and panic
		// so as not to propagate the nil into something else
		reference := getIdentifierReference(rt, rt.scope.lexical, name, false, at(node.idx))
		if reference == nil {
			// Should never get here!
			panic(hereBeDragons("referenceError == nil: " + name))
		}
		return toValue(reference)

	case *nodeLiteral:
		return node.value

	case *nodeNewExpression:
		return rt.cmplEvaluateNodeNewExpression(node)

	case *nodeObjectLiteral:
		return rt.cmplEvaluateNodeObjectLiteral(node)

	case *nodeRegExpLiteral:
		return objectValue(rt.newRegExpDirect(node.pattern, node.flags))

	case *nodeSequenceExpression:
		return rt.cmplEvaluateNodeSequenceExpression(node)

	case *nodeTemplateLiteral:
		var b strings.Builder
		for i, str := range node.strings {
			rt.allocateString(b.Len() + len(str))
			b.WriteString(str)
			if i < len(node.expressions) {
				sub := rt.cmplEvaluateNodeExpression(node.expressions[i]).resolve().string()
				rt.allocateString(b.Len() + len(sub))
				b.WriteString(sub)
			}
		}
		return stringValue(b.String())

	case *nodeClassLiteral:
		return rt.cmplEvaluateNodeClassLiteral(node)

	case *nodeSuperExpression:
		// A bare `super` is only meaningful as part of super(...) or super.x,
		// which are handled by the call and member evaluators.
		panic(rt.panicSyntaxError("'super' keyword unexpected here"))

	case *nodeTaggedTemplate:
		return rt.cmplEvaluateNodeTaggedTemplate(node)

	case *nodeThisExpression:
		return objectValue(rt.scope.this)

	case *nodeUnaryExpression:
		return rt.cmplEvaluateNodeUnaryExpression(node)

	case *nodeVariableExpression:
		return rt.cmplEvaluateNodeVariableExpression(node)
	default:
		panic(fmt.Sprintf("unknown node type: %T", node))
	}
}

func (rt *runtime) cmplEvaluateNodeArrayLiteral(node *nodeArrayLiteral) Value {
	valueArray := []Value{}

	for _, node := range node.value {
		switch node := node.(type) {
		case nil:
			valueArray = append(valueArray, emptyValue)
		case *nodeSpreadExpression:
			spread := rt.cmplEvaluateNodeExpression(node.value).resolve()
			valueArray = append(valueArray, rt.spreadIterable(spread)...)
		default:
			valueArray = append(valueArray, rt.cmplEvaluateNodeExpression(node).resolve())
		}
	}

	result := rt.newArrayOf(valueArray)

	return objectValue(result)
}

// spreadIterable expands a spread value (...value) into a slice of values. It
// supports arrays, strings (by code point) and array-like objects with a
// length property.
func (rt *runtime) spreadIterable(value Value) []Value {
	switch value.kind {
	case valueString:
		runes := []rune(value.string())
		rt.allocateDense(int64(len(runes)), allocValueCost)
		out := make([]Value, len(runes))
		for i, r := range runes {
			rt.pollInterrupt(i)
			out[i] = stringValue(string(r))
		}
		return out
	case valueObject:
		obj := value.object()
		length := int64(toUint32(obj.get(propertyLength)))
		out := make([]Value, 0, preallocation(length))
		for i := range length {
			rt.checkInterrupt()
			rt.allocateElement(i, allocValueCost)
			out = append(out, obj.get(arrayIndexToString(i)))
		}
		return out
	default:
		panic(rt.panicTypeError("%v is not iterable", value))
	}
}

func (rt *runtime) cmplEvaluateNodeAssignExpression(node *nodeAssignExpression) Value {
	// Destructuring assignment: [a, b] = ... or ({a, b} = ...).
	switch node.left.(type) {
	case *nodeArrayPattern, *nodeObjectPattern:
		rightValue := rt.cmplEvaluateNodeExpression(node.right).resolve()
		rt.bindPattern(node.left, rightValue, bindAssign)
		return rightValue
	}

	left := rt.cmplEvaluateNodeExpression(node.left)

	// Logical assignment (a ||= b, a &&= b, a ??= b) only evaluates and
	// assigns b when it would change a.
	switch node.operator {
	case token.LOGICAL_AND, token.LOGICAL_OR, token.NULLISH:
		leftValue := left.resolve()
		switch node.operator {
		case token.LOGICAL_AND:
			if !leftValue.bool() {
				return leftValue
			}
		case token.LOGICAL_OR:
			if leftValue.bool() {
				return leftValue
			}
		default:
			if !isNullish(leftValue) {
				return leftValue
			}
		}
		rightValue := rt.cmplEvaluateNodeExpression(node.right).resolve()
		rt.putValue(left.reference(), rightValue)
		return rightValue
	}

	right := rt.cmplEvaluateNodeExpression(node.right)
	rightValue := right.resolve()

	result := rightValue
	if node.operator != token.ASSIGN {
		result = rt.calculateBinaryExpression(node.operator, left, rightValue)
	}

	rt.putValue(left.reference(), result)

	return result
}

func (rt *runtime) cmplEvaluateNodeBinaryExpression(node *nodeBinaryExpression) Value {
	left := rt.cmplEvaluateNodeExpression(node.left)
	leftValue := left.resolve()

	switch node.operator {
	// Logical
	case token.LOGICAL_AND:
		if !leftValue.bool() {
			return leftValue
		}
		right := rt.cmplEvaluateNodeExpression(node.right)
		return right.resolve()
	case token.LOGICAL_OR:
		if leftValue.bool() {
			return leftValue
		}
		right := rt.cmplEvaluateNodeExpression(node.right)
		return right.resolve()
	case token.NULLISH:
		if !isNullish(leftValue) {
			return leftValue
		}
		return rt.cmplEvaluateNodeExpression(node.right).resolve()
	}

	return rt.calculateBinaryExpression(node.operator, leftValue, rt.cmplEvaluateNodeExpression(node.right))
}

func (rt *runtime) cmplEvaluateNodeBinaryExpressionComparison(node *nodeBinaryExpression) Value {
	left := rt.cmplEvaluateNodeExpression(node.left).resolve()
	right := rt.cmplEvaluateNodeExpression(node.right).resolve()

	return boolValue(rt.calculateComparison(node.operator, left, right))
}

func (rt *runtime) cmplEvaluateNodeBracketExpression(node *nodeBracketExpression) Value {
	return rt.bracketMember(node, rt.cmplEvaluateNodeExpression(node.left).resolve())
}

// bracketMember evaluates node's member and returns a reference to that
// property of targetValue.
func (rt *runtime) bracketMember(node *nodeBracketExpression, targetValue Value) Value {
	member := rt.cmplEvaluateNodeExpression(node.member)
	memberValue := member.resolve()

	// TODO Pass in base value as-is, and defer toObject till later?
	obj, err := rt.objectCoerce(targetValue)
	if err != nil {
		panic(rt.panicTypeError("Cannot access member %q of %s", memberValue.string(), err, at(node.idx)))
	}
	return toValue(newPropertyReference(rt, obj, memberValue.string(), false, at(node.idx)))
}

func (rt *runtime) cmplEvaluateNodeCallExpression(node *nodeCallExpression, withArgumentList []interface{}) Value {
	// super(...) and super.method(...) calls.
	switch callee := node.callee.(type) {
	case *nodeSuperExpression:
		return rt.evaluateSuperConstructorCall(node)
	case *nodeDotExpression:
		if _, ok := callee.left.(*nodeSuperExpression); ok {
			return rt.evaluateSuperMethodCall(callee.identifier, node)
		}
	case *nodeBracketExpression:
		if _, ok := callee.left.(*nodeSuperExpression); ok {
			key := rt.cmplEvaluateNodeExpression(callee.member).resolve().string()
			return rt.evaluateSuperMethodCall(key, node)
		}
	}

	callee := rt.cmplEvaluateNodeExpression(node.callee)
	return rt.callNode(node, callee, callee.resolve(), withArgumentList)
}

// callNode calls fn, the resolved value of callee, with node's arguments.
func (rt *runtime) callNode(node *nodeCallExpression, callee, fn Value, withArgumentList []interface{}) Value {
	this := Value{}
	var argumentList []Value
	if withArgumentList != nil {
		argumentList = rt.toValueArray(withArgumentList...)
	} else {
		argumentList = rt.evaluateArgumentList(node.argumentList)
	}

	eval := false // Whether this call is a (candidate for) direct call to eval
	name := ""
	if rf := callee.reference(); rf != nil {
		switch rf := rf.(type) {
		case *propertyReference:
			name = rf.name
			this = objectValue(rf.base)
			eval = rf.name == functionEval && !node.optional // Possible direct eval
		case *stashReference:
			// TODO ImplicitThisValue
			name = rf.name
			eval = rf.name == functionEval && !node.optional // Possible direct eval
		default:
			// FIXME?
			panic(rt.panicTypeError("unexpected callee type %T to node call expression", rf))
		}
	}

	atv := at(-1)
	switch callee := node.callee.(type) {
	case *nodeIdentifier:
		atv = at(callee.idx)
	case *nodeDotExpression:
		atv = at(callee.idx)
	case *nodeBracketExpression:
		atv = at(callee.idx)
	}

	frm := frame{
		callee: name,
		file:   rt.scope.frame.file,
	}

	if !fn.IsFunction() {
		if name == "" {
			// FIXME Maybe typeof?
			panic(rt.panicTypeError("%v is not a function", fn, atv))
		}
		panic(rt.panicTypeError("%q is not a function", name, atv))
	}

	rt.scope.frame.offset = int(atv)

	return fn.object().call(this, argumentList, eval, frm)
}

func isNullish(v Value) bool {
	return v.kind == valueUndefined || v.kind == valueNull
}

// evaluateChainLink evaluates a member or call expression inside an optional
// chain, reporting whether an optional link short-circuited the chain because
// its base was null or undefined.
func (rt *runtime) evaluateChainLink(node nodeExpression) (Value, bool) {
	switch node := node.(type) {
	case *nodeDotExpression:
		if _, ok := node.left.(*nodeSuperExpression); ok {
			break
		}
		base, ok := rt.evaluateChainBase(node.left, node.optional)
		if !ok {
			return Value{}, true
		}
		return rt.dotMember(node, base), false
	case *nodeBracketExpression:
		if _, ok := node.left.(*nodeSuperExpression); ok {
			break
		}
		base, ok := rt.evaluateChainBase(node.left, node.optional)
		if !ok {
			return Value{}, true
		}
		return rt.bracketMember(node, base), false
	case *nodeCallExpression:
		switch callee := node.callee.(type) {
		case *nodeSuperExpression:
			return rt.cmplEvaluateNodeExpression(node), false
		case *nodeDotExpression:
			if _, ok := callee.left.(*nodeSuperExpression); ok {
				return rt.cmplEvaluateNodeExpression(node), false
			}
		case *nodeBracketExpression:
			if _, ok := callee.left.(*nodeSuperExpression); ok {
				return rt.cmplEvaluateNodeExpression(node), false
			}
		}
		rt.enterNative()
		callee, short := rt.evaluateChainLink(node.callee)
		rt.leaveNative()
		if short {
			return Value{}, true
		}
		fn := callee.resolve()
		if node.optional && isNullish(fn) {
			return Value{}, true
		}
		return rt.callNode(node, callee, fn, nil), false
	}
	return rt.cmplEvaluateNodeExpression(node), false
}

// evaluateChainBase evaluates the object a chain link accesses a member of,
// returning false if the chain short-circuits before or at this link.
func (rt *runtime) evaluateChainBase(node nodeExpression, optional bool) (Value, bool) {
	rt.enterNative()
	base, short := rt.evaluateChainLink(node)
	rt.leaveNative()
	if short {
		return Value{}, false
	}
	value := base.resolve()
	if optional && isNullish(value) {
		return Value{}, false
	}
	return value, true
}

func (rt *runtime) cmplEvaluateNodeConditionalExpression(node *nodeConditionalExpression) Value {
	test := rt.cmplEvaluateNodeExpression(node.test)
	testValue := test.resolve()
	if testValue.bool() {
		return rt.cmplEvaluateNodeExpression(node.consequent)
	}
	return rt.cmplEvaluateNodeExpression(node.alternate)
}

func (rt *runtime) cmplEvaluateNodeTaggedTemplate(node *nodeTaggedTemplate) Value {
	tag := rt.cmplEvaluateNodeExpression(node.tag)

	// A tag of the form obj.fn`...` is called with `this` = obj.
	this := Value{}
	if rf, ok := tag.reference().(*propertyReference); ok && rf != nil {
		this = objectValue(rf.base)
	}

	fn := tag.resolve()
	if !fn.IsFunction() {
		panic(rt.panicTypeError("Tagged template tag is not a function"))
	}

	// Build the strings array, carrying the raw segments on its `raw` property.
	cooked := make([]Value, len(node.quasis))
	for i, s := range node.quasis {
		cooked[i] = stringValue(s)
	}
	stringsArray := rt.newArrayOf(cooked)
	raw := make([]Value, len(node.raw))
	for i, s := range node.raw {
		raw[i] = stringValue(s)
	}
	stringsArray.defineProperty("raw", objectValue(rt.newArrayOf(raw)), 0o000, false)

	argumentList := []Value{objectValue(stringsArray)}
	for _, expr := range node.expressions {
		argumentList = append(argumentList, rt.cmplEvaluateNodeExpression(expr).resolve())
	}

	return fn.object().call(this, argumentList, false, frame{})
}

func (rt *runtime) cmplEvaluateNodeDotExpression(node *nodeDotExpression) Value {
	if _, ok := node.left.(*nodeSuperExpression); ok {
		// super.x: read the member from the parent prototype.
		return rt.superPrototype().get(node.identifier)
	}
	return rt.dotMember(node, rt.cmplEvaluateNodeExpression(node.left).resolve())
}

// dotMember returns a reference to node's named property of targetValue.
func (rt *runtime) dotMember(node *nodeDotExpression, targetValue Value) Value {
	// TODO Pass in base value as-is, and defer toObject till later?
	obj, err := rt.objectCoerce(targetValue)
	if err != nil {
		panic(rt.panicTypeError("Cannot access member %q of %s", node.identifier, err, at(node.idx)))
	}
	return toValue(newPropertyReference(rt, obj, node.identifier, false, at(node.idx)))
}

func (rt *runtime) cmplEvaluateNodeNewExpression(node *nodeNewExpression) Value {
	callee := rt.cmplEvaluateNodeExpression(node.callee)
	argumentList := rt.evaluateArgumentList(node.argumentList)

	var name string
	if rf := callee.reference(); rf != nil {
		switch rf := rf.(type) {
		case *propertyReference:
			name = rf.name
		case *stashReference:
			name = rf.name
		default:
			panic(rt.panicTypeError("node new expression unexpected callee type %T", rf))
		}
	}

	atv := at(-1)
	switch callee := node.callee.(type) {
	case *nodeIdentifier:
		atv = at(callee.idx)
	case *nodeDotExpression:
		atv = at(callee.idx)
	case *nodeBracketExpression:
		atv = at(callee.idx)
	}

	vl := callee.resolve()
	if !vl.IsFunction() {
		if name == "" {
			// FIXME Maybe typeof?
			panic(rt.panicTypeError("%v is not a function", vl, atv))
		}
		panic(rt.panicTypeError("'%s' is not a function", name, atv))
	}

	rt.scope.frame.offset = int(atv)

	return vl.object().construct(argumentList)
}

func (rt *runtime) cmplEvaluateNodeObjectLiteral(node *nodeObjectLiteral) Value {
	result := rt.newObject()
	for _, prop := range node.value {
		key := prop.key
		if prop.keyExpression != nil {
			key = rt.cmplEvaluateNodeExpression(prop.keyExpression).resolve().string()
		}
		switch prop.kind {
		case "value":
			result.defineProperty(key, rt.cmplEvaluateNodeExpression(prop.value).resolve(), 0o111, false)
		case "spread":
			source := rt.cmplEvaluateNodeExpression(prop.value).resolve()
			if isNullish(source) {
				continue
			}
			obj := rt.toObject(source)
			i := 0
			obj.enumerate(false, func(name string) bool {
				i++
				rt.pollInterrupt(i)
				result.defineProperty(name, obj.get(name), 0o111, false)
				return true
			})
		case "get":
			getter := rt.newNodeFunction(prop.value.(*nodeFunctionLiteral), rt.scope.lexical)
			descriptor := property{}
			descriptor.mode = 0o211
			descriptor.value = propertyGetSet{getter, nil}
			result.defineOwnProperty(key, descriptor, false)
		case "set":
			setter := rt.newNodeFunction(prop.value.(*nodeFunctionLiteral), rt.scope.lexical)
			descriptor := property{}
			descriptor.mode = 0o211
			descriptor.value = propertyGetSet{nil, setter}
			result.defineOwnProperty(key, descriptor, false)
		default:
			panic(fmt.Sprintf("unknown node object literal property kind %T", prop.kind))
		}
	}

	return objectValue(result)
}

func (rt *runtime) cmplEvaluateNodeSequenceExpression(node *nodeSequenceExpression) Value {
	var result Value
	for _, node := range node.sequence {
		result = rt.cmplEvaluateNodeExpression(node)
		result = result.resolve()
	}
	return result
}

func (rt *runtime) cmplEvaluateNodeUnaryExpression(node *nodeUnaryExpression) Value {
	target := rt.cmplEvaluateNodeExpression(node.operand)
	switch node.operator {
	case token.TYPEOF, token.DELETE:
		if target.kind == valueReference && target.reference().invalid() {
			if node.operator == token.TYPEOF {
				return stringValue("undefined")
			}
			return trueValue
		}
	}

	switch node.operator {
	case token.NOT:
		targetValue := target.resolve()
		if targetValue.bool() {
			return falseValue
		}
		return trueValue
	case token.BITWISE_NOT:
		targetValue := target.resolve()
		integerValue := toInt32(targetValue)
		return int32Value(^integerValue)
	case token.PLUS:
		targetValue := target.resolve()
		return float64Value(targetValue.float64())
	case token.MINUS:
		targetValue := target.resolve()
		value := targetValue.float64()
		// TODO Test this
		sign := float64(-1)
		if math.Signbit(value) {
			sign = 1
		}
		return float64Value(math.Copysign(value, sign))
	case token.INCREMENT:
		targetValue := target.resolve()
		if node.postfix {
			// Postfix++
			oldValue := targetValue.float64()
			newValue := float64Value(+1 + oldValue)
			rt.putValue(target.reference(), newValue)
			return float64Value(oldValue)
		}

		// ++Prefix
		newValue := float64Value(+1 + targetValue.float64())
		rt.putValue(target.reference(), newValue)
		return newValue
	case token.DECREMENT:
		targetValue := target.resolve()
		if node.postfix {
			// Postfix--
			oldValue := targetValue.float64()
			newValue := float64Value(-1 + oldValue)
			rt.putValue(target.reference(), newValue)
			return float64Value(oldValue)
		}

		// --Prefix
		newValue := float64Value(-1 + targetValue.float64())
		rt.putValue(target.reference(), newValue)
		return newValue
	case token.VOID:
		target.resolve() // FIXME Side effect?
		return Value{}
	case token.DELETE:
		reference := target.reference()
		if reference == nil {
			return trueValue
		}
		return boolValue(target.reference().delete())
	case token.TYPEOF:
		targetValue := target.resolve()
		switch targetValue.kind {
		case valueUndefined:
			return stringValue("undefined")
		case valueNull:
			return stringValue("object")
		case valueBoolean:
			return stringValue("boolean")
		case valueNumber:
			return stringValue("number")
		case valueString:
			return stringValue("string")
		case valueObject:
			if targetValue.object().isCall() {
				return stringValue("function")
			}
			return stringValue("object")
		default:
			// FIXME ?
		}
	}

	panic(hereBeDragons())
}

func (rt *runtime) cmplEvaluateNodeVariableExpression(node *nodeVariableExpression) Value {
	if node.target != nil {
		// var destructuring: the bound names are already hoisted, so assign
		// into them.
		value := Value{}
		if node.initializer != nil {
			value = rt.cmplEvaluateNodeExpression(node.initializer).resolve()
		}
		rt.bindPattern(node.target, value, bindAssign)
		return emptyValue
	}
	if node.initializer != nil {
		// FIXME If reference is nil
		left := getIdentifierReference(rt, rt.scope.lexical, node.name, false, at(node.idx))
		right := rt.cmplEvaluateNodeExpression(node.initializer)
		rightValue := right.resolve()

		rt.putValue(left, rightValue)
	}
	return stringValue(node.name)
}
