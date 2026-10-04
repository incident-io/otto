package otto

// cmplEvaluateNodeClassStatement evaluates a class declaration, binding the
// class constructor to its name in the current lexical environment.
func (rt *runtime) cmplEvaluateNodeClassStatement(node *nodeClassStatement) Value {
	value := rt.cmplEvaluateNodeClassLiteral(node.class)
	if node.name != "" {
		rt.declareLexicalBinding(node.name, value, false)
	}
	return emptyValue
}

// cmplEvaluateNodeClassLiteral builds a class: a constructor function whose
// prototype carries the instance methods, with static members on the
// constructor itself and the prototype/static chains wired up for extends.
func (rt *runtime) cmplEvaluateNodeClassLiteral(node *nodeClassLiteral) Value {
	var superConstructor *object
	var superPrototype *object
	hasSuper := node.superClass != nil
	if hasSuper {
		superValue := rt.cmplEvaluateNodeExpression(node.superClass).resolve()
		if !superValue.IsNull() {
			if !superValue.IsFunction() {
				panic(rt.panicTypeError("Class extends value is not a constructor or null"))
			}
			superConstructor = superValue.object()
			if proto := superConstructor.get("prototype"); proto.IsObject() {
				superPrototype = proto.object()
			}
		}
	}

	// The prototype object holding instance methods.
	prototype := rt.newObject()
	if hasSuper {
		prototype.prototype = superPrototype
	}

	// Locate an explicit constructor, if any.
	var constructorNode *nodeFunctionLiteral
	for i := range node.elements {
		if node.elements[i].kind == "constructor" {
			constructorNode = node.elements[i].method
		}
	}

	var constructor *object
	if constructorNode != nil {
		constructor = rt.newNodeFunction(constructorNode, rt.scope.lexical)
	} else {
		// Synthesise a default constructor.
		empty := &nodeFunctionLiteral{
			name: node.name,
			body: &nodeBlockStatement{},
			file: rt.scope.frame.file,
		}
		constructor = rt.newNodeFunction(empty, rt.scope.lexical)
		if hasSuper {
			// A default derived constructor forwards its arguments to super().
			fn := constructor.value.(nodeFunctionObject)
			fn.defaultDerivedCtor = true
			constructor.value = fn
		}
	}

	constructor.defineProperty("prototype", objectValue(prototype), 0o000, false)
	constructor.defineProperty("name", stringValue(node.name), 0o001, false)
	prototype.defineProperty("constructor", objectValue(constructor), 0o101, false)
	rt.setHomeObject(constructor, prototype)
	if superConstructor != nil {
		// Static inheritance: the constructor's prototype is the super constructor.
		constructor.prototype = superConstructor
	}

	// Define the methods, getters and setters.
	for i := range node.elements {
		element := &node.elements[i]
		if element.kind == "constructor" {
			continue
		}

		target := prototype
		if element.static {
			target = constructor
		}

		key := element.key
		if element.keyExpr != nil {
			key = rt.cmplEvaluateNodeExpression(element.keyExpr).resolve().string()
		}

		fn := rt.newNodeFunction(element.method, rt.scope.lexical)
		rt.setHomeObject(fn, target)

		switch element.kind {
		case "get":
			rt.defineClassAccessor(target, key, fn, true)
		case "set":
			rt.defineClassAccessor(target, key, fn, false)
		default: // method
			target.defineProperty(key, objectValue(fn), 0o101, false)
		}
	}

	return objectValue(constructor)
}

// setHomeObject records the object a function was defined on, for super.
func (rt *runtime) setHomeObject(fn *object, home *object) {
	value := fn.value.(nodeFunctionObject)
	value.homeObject = home
	fn.value = value
}

// defineClassAccessor defines (or augments) a getter/setter accessor property.
func (rt *runtime) defineClassAccessor(target *object, key string, fn *object, isGetter bool) {
	getset := propertyGetSet{}
	if existing, exists := target.property[key]; exists {
		if current, ok := existing.value.(propertyGetSet); ok {
			getset = current
		}
	}
	if isGetter {
		getset[0] = fn
	} else {
		getset[1] = fn
	}
	target.defineOwnProperty(key, property{value: getset, mode: 0o101}, false)
}

// currentHomeObject returns the home object of the nearest enclosing class
// method or constructor, used to resolve `super`.
func (rt *runtime) currentHomeObject() *object {
	for sc := rt.scope; sc != nil; sc = sc.outer {
		obj, ok := sc.frame.fn.(*object)
		if !ok {
			continue
		}
		if fn, isNode := obj.value.(nodeFunctionObject); isNode && fn.homeObject != nil {
			return fn.homeObject
		}
	}
	return nil
}

// superConstructor returns the parent constructor for a super(...) call.
func (rt *runtime) superConstructor() *object {
	home := rt.currentHomeObject()
	if home == nil || home.prototype == nil {
		panic(rt.panicSyntaxError("'super' keyword unexpected here"))
	}
	ctor := home.prototype.get("constructor")
	if !ctor.IsFunction() {
		panic(rt.panicTypeError("Super constructor is not a constructor"))
	}
	return ctor.object()
}

// evaluateArgumentList evaluates a call/new argument list, expanding spreads.
func (rt *runtime) evaluateArgumentList(nodes []nodeExpression) []Value {
	argumentList := []Value{}
	for _, argumentNode := range nodes {
		if spread, ok := argumentNode.(*nodeSpreadExpression); ok {
			value := rt.cmplEvaluateNodeExpression(spread.value).resolve()
			argumentList = append(argumentList, rt.spreadIterable(value)...)
			continue
		}
		argumentList = append(argumentList, rt.cmplEvaluateNodeExpression(argumentNode).resolve())
	}
	return argumentList
}

// evaluateSuperConstructorCall handles super(...), invoking the parent
// constructor on the current `this`.
func (rt *runtime) evaluateSuperConstructorCall(node *nodeCallExpression) Value {
	parent := rt.superConstructor()
	argumentList := rt.evaluateArgumentList(node.argumentList)
	this := objectValue(rt.scope.this)
	parent.call(this, argumentList, false, frame{})
	return emptyValue
}

// evaluateSuperMethodCall handles super.method(...) and super[expr](...),
// looking the method up on the parent prototype but calling it with the
// current `this`.
func (rt *runtime) evaluateSuperMethodCall(key string, node *nodeCallExpression) Value {
	proto := rt.superPrototype()
	method := proto.get(key)
	if !method.IsFunction() {
		panic(rt.panicTypeError("(intermediate value).%s is not a function", key))
	}
	argumentList := rt.evaluateArgumentList(node.argumentList)
	this := objectValue(rt.scope.this)
	return method.object().call(this, argumentList, false, frame{})
}

// superPrototype returns the object on which `super.x` member accesses resolve.
func (rt *runtime) superPrototype() *object {
	home := rt.currentHomeObject()
	if home == nil {
		panic(rt.panicSyntaxError("'super' keyword unexpected here"))
	}
	return home.prototype
}
