package otto

import (
	"fmt"
	goruntime "runtime"

	"github.com/robertkrimen/otto/file"
	"github.com/robertkrimen/otto/token"
)

func (rt *runtime) cmplEvaluateNodeStatement(node nodeStatement) Value {
	// Allow interpreter interruption
	// If the Interrupt channel is nil, then
	// we avoid runtime.Gosched() overhead (if any)
	// FIXME: Test this
	if rt.otto.Interrupt != nil {
		goruntime.Gosched()
		rt.checkInterrupt()
	}

	rt.enterNative()
	value := rt.cmplEvaluateNodeStatementInner(node)
	rt.leaveNative()
	return value
}

func (rt *runtime) cmplEvaluateNodeStatementInner(node nodeStatement) Value {
	switch node := node.(type) {
	case *nodeBlockStatement:
		return rt.cmplEvaluateNodeBlockStatement(node)

	case *nodeLexicalDeclaration:
		return rt.cmplEvaluateNodeLexicalDeclaration(node)

	case *nodeClassStatement:
		return rt.cmplEvaluateNodeClassStatement(node)

	case *nodeBranchStatement:
		target := node.label
		switch node.branch { // FIXME Maybe node.kind? node.operator?
		case token.BREAK:
			return toValue(newBreakResult(target))
		case token.CONTINUE:
			return toValue(newContinueResult(target))
		default:
			panic(fmt.Errorf("unknown node branch token %T", node))
		}

	case *nodeDebuggerStatement:
		if rt.debugger != nil {
			rt.debugger(rt.otto)
		}
		return emptyValue // Nothing happens.

	case *nodeDoWhileStatement:
		return rt.cmplEvaluateNodeDoWhileStatement(node)

	case *nodeEmptyStatement:
		return emptyValue

	case *nodeExpressionStatement:
		return rt.cmplEvaluateNodeExpression(node.expression)

	case *nodeForInStatement:
		return rt.cmplEvaluateNodeForInStatement(node)

	case *nodeForStatement:
		return rt.cmplEvaluateNodeForStatement(node)

	case *nodeIfStatement:
		return rt.cmplEvaluateNodeIfStatement(node)

	case *nodeLabelledStatement:
		rt.labels = append(rt.labels, node.label)
		defer func() {
			if len(rt.labels) > 0 {
				rt.labels = rt.labels[:len(rt.labels)-1] // Pop the label
			} else {
				rt.labels = nil
			}
		}()
		return rt.cmplEvaluateNodeStatement(node.statement)

	case *nodeReturnStatement:
		if node.argument != nil {
			return toValue(newReturnResult(rt.cmplEvaluateNodeExpression(node.argument).resolve()))
		}
		return toValue(newReturnResult(Value{}))

	case *nodeSwitchStatement:
		return rt.cmplEvaluateNodeSwitchStatement(node)

	case *nodeThrowStatement:
		value := rt.cmplEvaluateNodeExpression(node.argument).resolve()
		panic(newException(value))

	case *nodeTryStatement:
		return rt.cmplEvaluateNodeTryStatement(node)

	case *nodeVariableStatement:
		// Variables are already defined, this is initialization only
		for _, variable := range node.list {
			rt.cmplEvaluateNodeVariableExpression(variable.(*nodeVariableExpression))
		}
		return emptyValue

	case *nodeWhileStatement:
		return rt.cmplEvaluateModeWhileStatement(node)

	case *nodeWithStatement:
		return rt.cmplEvaluateNodeWithStatement(node)
	default:
		panic(fmt.Errorf("unknown node statement type %T", node))
	}
}

func (rt *runtime) cmplEvaluateNodeStatementList(list []nodeStatement) Value {
	var result Value
	for _, node := range list {
		value := rt.cmplEvaluateNodeStatement(node)
		switch value.kind {
		case valueResult:
			return value
		case valueEmpty:
		default:
			// We have getValue here to (for example) trigger a
			// ReferenceError (of the not defined variety)
			// Not sure if this is the best way to error out early
			// for such errors or if there is a better way
			// TODO Do we still need this?
			result = value.resolve()
		}
	}
	return result
}

func (rt *runtime) cmplEvaluateNodeDoWhileStatement(node *nodeDoWhileStatement) Value {
	labels := append(rt.labels, "") //nolint:gocritic
	rt.labels = nil

	test := node.test

	result := emptyValue
resultBreak:
	for {
		for _, node := range node.body {
			value := rt.cmplEvaluateNodeStatement(node)
			switch value.kind {
			case valueResult:
				switch value.evaluateBreakContinue(labels) {
				case resultReturn:
					return value
				case resultBreak:
					break resultBreak
				case resultContinue:
					goto resultContinue
				}
			case valueEmpty:
			default:
				result = value
			}
		}
	resultContinue:
		if !rt.cmplEvaluateNodeExpression(test).resolve().bool() {
			// Stahp: do ... while (false)
			break
		}
	}
	return result
}

// bindMode selects what binding a destructuring pattern performs at each leaf.
type bindMode int

const (
	bindAssign bindMode = iota // assign to an existing reference
	bindLet                    // declare a new let binding
	bindConst                  // declare a new const binding
)

// bindPattern binds a value against a destructuring target (an identifier, a
// member reference, a nested pattern, or a target-with-default), using the
// given mode at each leaf.
func (rt *runtime) bindPattern(target nodeExpression, value Value, mode bindMode) {
	switch t := target.(type) {
	case *nodeIdentifier:
		rt.bindName(t.name, value, mode, t.idx)
	case *nodeDotExpression, *nodeBracketExpression:
		// Only valid in assignment destructuring; assign to the member.
		ref := rt.cmplEvaluateNodeExpression(target)
		rt.putValue(ref.reference(), value)
	case *nodeAssignExpression:
		// target = default
		if value.IsUndefined() {
			value = rt.cmplEvaluateNodeExpression(t.right).resolve()
		}
		rt.bindPattern(t.left, value, mode)
	case *nodeArrayPattern:
		rt.bindArrayPattern(t, value, mode)
	case *nodeObjectPattern:
		rt.bindObjectPattern(t, value, mode)
	default:
		panic(rt.panicTypeError("invalid destructuring target %T", target))
	}
}

func (rt *runtime) bindName(name string, value Value, mode bindMode, idx file.Idx) {
	switch mode {
	case bindLet:
		rt.declareLexicalBinding(name, value, false)
	case bindConst:
		rt.declareLexicalBinding(name, value, true)
	default: // bindAssign
		ref := getIdentifierReference(rt, rt.scope.lexical, name, false, at(idx))
		rt.putValue(ref, value)
	}
}

func (rt *runtime) bindArrayPattern(pattern *nodeArrayPattern, value Value, mode bindMode) {
	values := rt.spreadIterable(value)
	for i, element := range pattern.elements {
		if element == nil {
			continue // elision (hole)
		}
		var elementValue Value
		if i < len(values) {
			elementValue = values[i]
		}
		rt.bindPattern(element, elementValue, mode)
	}
	if pattern.rest != nil {
		rest := []Value{}
		if len(pattern.elements) < len(values) {
			rest = append(rest, values[len(pattern.elements):]...)
		}
		rt.bindPattern(pattern.rest, objectValue(rt.newArrayOf(rest)), mode)
	}
}

func (rt *runtime) bindObjectPattern(pattern *nodeObjectPattern, value Value, mode bindMode) {
	switch value.kind {
	case valueUndefined, valueNull:
		panic(rt.panicTypeError("cannot destructure %v", value))
	}
	obj := rt.toObject(value)

	taken := map[string]bool{}
	for _, prop := range pattern.properties {
		key := prop.key
		if prop.keyExpr != nil {
			key = rt.cmplEvaluateNodeExpression(prop.keyExpr).resolve().string()
		}
		taken[key] = true
		rt.bindPattern(prop.target, obj.get(key), mode)
	}
	if pattern.rest != nil {
		rest := rt.newObject()
		obj.enumerate(false, func(name string) bool {
			if !taken[name] {
				rest.put(name, obj.get(name), false)
			}
			return true
		})
		rt.bindPattern(pattern.rest, objectValue(rest), mode)
	}
}

// cmplEvaluateNodeBlockStatement evaluates a block, introducing a fresh lexical
// environment record when the block contains let/const declarations.
func (rt *runtime) cmplEvaluateNodeBlockStatement(node *nodeBlockStatement) Value {
	labels := rt.labels
	rt.labels = nil

	if node.lexical {
		restore := rt.enterLexicalScope()
		defer restore()
	}

	value := rt.cmplEvaluateNodeStatementList(node.list)
	if value.kind == valueResult {
		if value.evaluateBreak(labels) == resultBreak {
			return emptyValue
		}
	}
	return value
}

// enterLexicalScope pushes a new declarative environment record as the current
// lexical environment, returning a function that restores the previous one.
func (rt *runtime) enterLexicalScope() func() {
	saved := rt.scope.lexical
	rt.scope.lexical = rt.newDeclarationStash(saved)
	return func() {
		rt.scope.lexical = saved
	}
}

// cmplEvaluateNodeLexicalDeclaration creates let/const bindings in the current
// lexical environment. There is no temporal dead zone: a binding read before
// its declaration resolves to an outer scope rather than throwing.
func (rt *runtime) cmplEvaluateNodeLexicalDeclaration(node *nodeLexicalDeclaration) Value {
	for _, binding := range node.bindings {
		value := Value{}
		if binding.hasValue {
			value = rt.cmplEvaluateNodeExpression(binding.initializer).resolve()
		}
		if binding.target != nil {
			mode := bindLet
			if node.immutable {
				mode = bindConst
			}
			rt.bindPattern(binding.target, value, mode)
			continue
		}
		rt.declareLexicalBinding(binding.name, value, node.immutable)
	}
	return emptyValue
}

// declareLexicalBinding creates a single let/const binding in the current
// lexical environment. When the environment is a declarative record (the usual
// case for blocks, loops and lexical-scoped programs) const immutability is
// enforced; otherwise it falls back to an ordinary binding.
func (rt *runtime) declareLexicalBinding(name string, value Value, immutable bool) {
	if ds, ok := rt.scope.lexical.(*dclStash); ok {
		if immutable {
			ds.createImmutableBinding(name, value)
			return
		}
		if ds.hasBinding(name) {
			ds.setBinding(name, value, false)
		} else {
			ds.createBinding(name, false, value)
		}
		return
	}
	rt.scope.lexical.setValue(name, value, false)
}

func (rt *runtime) cmplEvaluateNodeForInStatement(node *nodeForInStatement) Value {
	labels := append(rt.labels, "") //nolint:gocritic
	rt.labels = nil

	if node.of {
		return rt.cmplEvaluateNodeForOfStatement(node)
	}

	source := rt.cmplEvaluateNodeExpression(node.source)
	sourceValue := source.resolve()

	switch sourceValue.kind {
	case valueUndefined, valueNull:
		return emptyValue
	}

	sourceObject := rt.toObject(sourceValue)

	into := node.into
	body := node.body

	// A block-scoped loop variable (for (let k in obj)) gets a fresh binding
	// per iteration in its own lexical environment.
	lexical := node.lexical
	var outerLexical stasher
	if lexical {
		outerLexical = rt.scope.lexical
		defer func() { rt.scope.lexical = outerLexical }()
	}

	result := emptyValue
	obj := sourceObject
	for obj != nil {
		enumerateValue := emptyValue
		obj.enumerate(false, func(name string) bool {
			if lexical {
				rt.scope.lexical = rt.newDeclarationStash(outerLexical)
			}
			rt.bindForTarget(into, stringValue(name), lexical, node.immutable)
			for _, node := range body {
				value := rt.cmplEvaluateNodeStatement(node)
				switch value.kind {
				case valueResult:
					switch value.evaluateBreakContinue(labels) {
					case resultReturn:
						enumerateValue = value
						return false
					case resultBreak:
						obj = nil
						return false
					case resultContinue:
						return true
					}
				case valueEmpty:
				default:
					enumerateValue = value
				}
			}
			return true
		})
		if obj == nil {
			break
		}
		obj = obj.prototype
		if !enumerateValue.isEmpty() {
			result = enumerateValue
		}
	}
	return result
}

// bindForTarget binds a loop value to a for-of/for-in loop target, which may be
// a declared name, a destructuring pattern, or an existing assignment target.
func (rt *runtime) bindForTarget(into nodeExpression, value Value, lexical, immutable bool) {
	if ve, ok := into.(*nodeVariableExpression); ok {
		if ve.target != nil {
			mode := bindAssign
			if lexical {
				mode = bindLet
				if immutable {
					mode = bindConst
				}
			}
			rt.bindPattern(ve.target, value, mode)
			return
		}
		if lexical {
			rt.declareLexicalBinding(ve.name, value, immutable)
			return
		}
		ref := getIdentifierReference(rt, rt.scope.lexical, ve.name, false, at(ve.idx))
		rt.putValue(ref, value)
		return
	}

	// An assignment destructuring pattern, e.g. for ([a, b] of x) or
	// for ({a} in obj). These bind against existing references.
	switch into.(type) {
	case *nodeArrayPattern, *nodeObjectPattern:
		rt.bindPattern(into, value, bindAssign)
		return
	}

	// An existing identifier or member assignment target.
	ref := rt.cmplEvaluateNodeExpression(into)
	if ref.reference() == nil {
		identifier := ref.string()
		ref = toValue(getIdentifierReference(rt, rt.scope.lexical, identifier, false, -1))
	}
	rt.putValue(ref.reference(), value)
}

// cmplEvaluateNodeForOfStatement evaluates a for-of loop, iterating the values
// of an iterable. Arrays, strings and array-like objects are supported. A
// block-scoped loop variable (for (let x of ...)) gets a fresh binding per
// iteration.
func (rt *runtime) cmplEvaluateNodeForOfStatement(node *nodeForInStatement) Value {
	labels := append(rt.labels, "") //nolint:gocritic
	rt.labels = nil

	sourceValue := rt.cmplEvaluateNodeExpression(node.source).resolve()
	switch sourceValue.kind {
	case valueUndefined, valueNull:
		panic(rt.panicTypeError("%v is not iterable", sourceValue))
	}
	values := rt.spreadIterable(sourceValue)

	into := node.into
	body := node.body

	lexical := node.lexical
	var outerLexical stasher
	if lexical {
		outerLexical = rt.scope.lexical
		defer func() { rt.scope.lexical = outerLexical }()
	}

	result := emptyValue
forLoop:
	for _, iterationValue := range values {
		if lexical {
			// A fresh per-iteration environment for the loop binding(s).
			rt.scope.lexical = rt.newDeclarationStash(outerLexical)
		}
		rt.bindForTarget(into, iterationValue, lexical, node.immutable)

		for _, n := range body {
			value := rt.cmplEvaluateNodeStatement(n)
			switch value.kind {
			case valueResult:
				switch value.evaluateBreakContinue(labels) {
				case resultReturn:
					return value
				case resultBreak:
					break forLoop
				case resultContinue:
					continue forLoop
				}
			case valueEmpty:
			default:
				result = value
			}
		}
	}
	return result
}

func (rt *runtime) cmplEvaluateNodeForStatement(node *nodeForStatement) Value {
	labels := append(rt.labels, "") //nolint:gocritic
	rt.labels = nil

	initializer := node.initializer
	test := node.test
	update := node.update
	body := node.body

	// For block-scoped loop variables (for (let i ...)), each iteration runs in
	// a fresh copy of the loop environment so that closures created in the body
	// capture that iteration's bindings.
	createPerIteration := func() {}
	if len(node.lexicalBindings) > 0 {
		outerLexical := rt.scope.lexical
		loopEnv := rt.newDeclarationStash(outerLexical)
		for _, name := range node.lexicalBindings {
			loopEnv.createBinding(name, false, Value{})
		}
		rt.scope.lexical = loopEnv
		defer func() { rt.scope.lexical = outerLexical }()

		createPerIteration = func() {
			prev := rt.scope.lexical.(*dclStash)
			next := rt.newDeclarationStash(outerLexical)
			for _, name := range node.lexicalBindings {
				next.createBinding(name, false, prev.getBinding(name, false))
			}
			rt.scope.lexical = next
		}
	}

	if initializer != nil {
		initialResult := rt.cmplEvaluateNodeExpression(initializer)
		initialResult.resolve() // Side-effect trigger
	}

	createPerIteration() // CreatePerIterationEnvironment, before the first test

	result := emptyValue
resultBreak:
	for {
		if test != nil {
			testResult := rt.cmplEvaluateNodeExpression(test)
			testResultValue := testResult.resolve()
			if !testResultValue.bool() {
				break
			}
		}

		// this is to prevent for cycles with no body from running forever
		if len(body) == 0 && rt.otto.Interrupt != nil {
			goruntime.Gosched()
			rt.checkInterrupt()
		}

		for _, node := range body {
			value := rt.cmplEvaluateNodeStatement(node)
			switch value.kind {
			case valueResult:
				switch value.evaluateBreakContinue(labels) {
				case resultReturn:
					return value
				case resultBreak:
					break resultBreak
				case resultContinue:
					goto resultContinue
				}
			case valueEmpty:
			default:
				result = value
			}
		}
	resultContinue:
		createPerIteration() // copy the bindings forward for the next iteration
		if update != nil {
			updateResult := rt.cmplEvaluateNodeExpression(update)
			updateResult.resolve() // Side-effect trigger
		}
	}
	return result
}

func (rt *runtime) cmplEvaluateNodeIfStatement(node *nodeIfStatement) Value {
	test := rt.cmplEvaluateNodeExpression(node.test)
	testValue := test.resolve()
	if testValue.bool() {
		return rt.cmplEvaluateNodeStatement(node.consequent)
	} else if node.alternate != nil {
		return rt.cmplEvaluateNodeStatement(node.alternate)
	}

	return emptyValue
}

func (rt *runtime) cmplEvaluateNodeSwitchStatement(node *nodeSwitchStatement) Value {
	labels := append(rt.labels, "") //nolint:gocritic
	rt.labels = nil

	discriminantResult := rt.cmplEvaluateNodeExpression(node.discriminant)
	target := node.defaultIdx

	for index, clause := range node.body {
		test := clause.test
		if test != nil {
			if rt.calculateComparison(token.STRICT_EQUAL, discriminantResult, rt.cmplEvaluateNodeExpression(test)) {
				target = index
				break
			}
		}
	}

	result := emptyValue
	if target != -1 {
		for _, clause := range node.body[target:] {
			for _, statement := range clause.consequent {
				value := rt.cmplEvaluateNodeStatement(statement)
				switch value.kind {
				case valueResult:
					switch value.evaluateBreak(labels) {
					case resultReturn:
						return value
					case resultBreak:
						return result
					}
				case valueEmpty:
				default:
					result = value
				}
			}
		}
	}

	return result
}

func (rt *runtime) cmplEvaluateNodeTryStatement(node *nodeTryStatement) Value {
	tryCatchValue, exep := rt.tryCatchEvaluate(func() Value {
		return rt.cmplEvaluateNodeStatement(node.body)
	})

	if exep && node.catch != nil {
		outer := rt.scope.lexical
		rt.scope.lexical = rt.newDeclarationStash(outer)
		defer func() {
			rt.scope.lexical = outer
		}()
		// TODO If necessary, convert TypeError<runtime> => TypeError
		// That, is, such errors can be thrown despite not being JavaScript "native"
		// strict = false
		if node.catch.parameter != "" {
			rt.scope.lexical.setValue(node.catch.parameter, tryCatchValue, false)
		}

		// FIXME node.CatchParameter
		// FIXME node.Catch
		tryCatchValue, exep = rt.tryCatchEvaluate(func() Value {
			return rt.cmplEvaluateNodeStatement(node.catch.body)
		})
	}

	if node.finally != nil {
		finallyValue := rt.cmplEvaluateNodeStatement(node.finally)
		if finallyValue.kind == valueResult {
			return finallyValue
		}
	}

	if exep {
		panic(newException(tryCatchValue))
	}

	return tryCatchValue
}

func (rt *runtime) cmplEvaluateModeWhileStatement(node *nodeWhileStatement) Value {
	test := node.test
	body := node.body
	labels := append(rt.labels, "") //nolint:gocritic
	rt.labels = nil

	result := emptyValue
resultBreakContinue:
	for {
		if !rt.cmplEvaluateNodeExpression(test).resolve().bool() {
			// Stahp: while (false) ...
			break
		}
		for _, node := range body {
			value := rt.cmplEvaluateNodeStatement(node)
			switch value.kind {
			case valueResult:
				switch value.evaluateBreakContinue(labels) {
				case resultReturn:
					return value
				case resultBreak:
					break resultBreakContinue
				case resultContinue:
					continue resultBreakContinue
				}
			case valueEmpty:
			default:
				result = value
			}
		}
	}
	return result
}

func (rt *runtime) cmplEvaluateNodeWithStatement(node *nodeWithStatement) Value {
	obj := rt.cmplEvaluateNodeExpression(node.object)
	outer := rt.scope.lexical
	lexical := rt.newObjectStash(rt.toObject(obj.resolve()), outer)
	rt.scope.lexical = lexical
	defer func() {
		rt.scope.lexical = outer
	}()

	return rt.cmplEvaluateNodeStatement(node.body)
}
