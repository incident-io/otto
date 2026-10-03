package parser

import (
	"github.com/robertkrimen/otto/ast"
	"github.com/robertkrimen/otto/file"
	"github.com/robertkrimen/otto/token"
)

func (p *parser) parseBlockStatement() *ast.BlockStatement {
	node := &ast.BlockStatement{}

	// Find comments before the leading brace
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, p.comments.FetchAll(), ast.LEADING)
		p.comments.Unset()
	}

	node.LeftBrace = p.expect(token.LEFT_BRACE)
	node.List = p.parseStatementList()

	if p.mode&StoreComments != 0 {
		p.comments.Unset()
		p.comments.CommentMap.AddComments(node, p.comments.FetchAll(), ast.FINAL)
		p.comments.AfterBlock()
	}

	node.RightBrace = p.expect(token.RIGHT_BRACE)

	// Find comments after the trailing brace
	if p.mode&StoreComments != 0 {
		p.comments.ResetLineBreak()
		p.comments.CommentMap.AddComments(node, p.comments.Fetch(), ast.TRAILING)
	}

	return node
}

func (p *parser) parseEmptyStatement() ast.Statement {
	idx := p.expect(token.SEMICOLON)
	return &ast.EmptyStatement{Semicolon: idx}
}

func (p *parser) parseStatementList() (list []ast.Statement) { //nolint:nonamedreturns
	for p.token != token.RIGHT_BRACE && p.token != token.EOF {
		statement := p.parseStatement()
		list = append(list, statement)
	}

	return list
}

func (p *parser) parseStatement() ast.Statement {
	if p.token == token.EOF {
		p.errorUnexpectedToken(p.token)
		return &ast.BadStatement{From: p.idx, To: p.idx + 1}
	}

	if p.mode&StoreComments != 0 {
		p.comments.ResetLineBreak()
	}

	switch p.token {
	case token.SEMICOLON:
		return p.parseEmptyStatement()
	case token.LEFT_BRACE:
		return p.parseBlockStatement()
	case token.IF:
		return p.parseIfStatement()
	case token.DO:
		statement := p.parseDoWhileStatement()
		p.comments.PostProcessNode(statement)
		return statement
	case token.WHILE:
		return p.parseWhileStatement()
	case token.FOR:
		return p.parseForOrForInStatement()
	case token.BREAK:
		return p.parseBreakStatement()
	case token.CONTINUE:
		return p.parseContinueStatement()
	case token.DEBUGGER:
		return p.parseDebuggerStatement()
	case token.WITH:
		return p.parseWithStatement()
	case token.VAR:
		return p.parseVariableStatement()
	case token.LET, token.CONST:
		return p.parseLexicalDeclaration()
	case token.CLASS:
		return p.parseClass(true)
	case token.FUNCTION:
		return p.parseFunctionStatement()
	case token.SWITCH:
		return p.parseSwitchStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	case token.THROW:
		return p.parseThrowStatement()
	case token.TRY:
		return p.parseTryStatement()
	}

	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}

	expression := p.parseExpression()

	if identifier, isIdentifier := expression.(*ast.Identifier); isIdentifier && p.token == token.COLON {
		// LabelledStatement
		colon := p.idx
		if p.mode&StoreComments != 0 {
			p.comments.Unset()
		}
		p.next() // :

		label := identifier.Name
		for _, value := range p.scope.labels {
			if label == value {
				p.error(identifier.Idx0(), "Label '%s' already exists", label)
			}
		}
		var labelComments []*ast.Comment
		if p.mode&StoreComments != 0 {
			labelComments = p.comments.FetchAll()
		}
		p.scope.labels = append(p.scope.labels, label) // Push the label
		statement := p.parseStatement()
		p.scope.labels = p.scope.labels[:len(p.scope.labels)-1] // Pop the label
		exp := &ast.LabelledStatement{
			Label:     identifier,
			Colon:     colon,
			Statement: statement,
		}
		if p.mode&StoreComments != 0 {
			p.comments.CommentMap.AddComments(exp, labelComments, ast.LEADING)
		}

		return exp
	}

	p.optionalSemicolon()

	statement := &ast.ExpressionStatement{
		Expression: expression,
	}

	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(statement, comments, ast.LEADING)
	}
	return statement
}

func (p *parser) parseTryStatement() ast.Statement {
	var tryComments []*ast.Comment
	if p.mode&StoreComments != 0 {
		tryComments = p.comments.FetchAll()
	}
	node := &ast.TryStatement{
		Try:  p.expect(token.TRY),
		Body: p.parseBlockStatement(),
	}
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, tryComments, ast.LEADING)
		p.comments.CommentMap.AddComments(node.Body, p.comments.FetchAll(), ast.TRAILING)
	}

	if p.token == token.CATCH {
		catch := p.idx
		if p.mode&StoreComments != 0 {
			p.comments.Unset()
		}
		p.next()
		p.expect(token.LEFT_PARENTHESIS)
		if p.token != token.IDENTIFIER {
			p.expect(token.IDENTIFIER)
			p.nextStatement()
			return &ast.BadStatement{From: catch, To: p.idx}
		}

		identifier := p.parseIdentifier()
		p.expect(token.RIGHT_PARENTHESIS)
		node.Catch = &ast.CatchStatement{
			Catch:     catch,
			Parameter: identifier,
			Body:      p.parseBlockStatement(),
		}

		if p.mode&StoreComments != 0 {
			p.comments.CommentMap.AddComments(node.Catch.Body, p.comments.FetchAll(), ast.TRAILING)
		}
	}

	if p.token == token.FINALLY {
		if p.mode&StoreComments != 0 {
			p.comments.Unset()
		}
		p.next()
		if p.mode&StoreComments != 0 {
			tryComments = p.comments.FetchAll()
		}

		node.Finally = p.parseBlockStatement()

		if p.mode&StoreComments != 0 {
			p.comments.CommentMap.AddComments(node.Finally, tryComments, ast.LEADING)
		}
	}

	if node.Catch == nil && node.Finally == nil {
		p.error(node.Try, "Missing catch or finally after try")
		return &ast.BadStatement{From: node.Try, To: node.Body.Idx1()}
	}

	return node
}

func (p *parser) parseFunctionParameterList() *ast.ParameterList {
	opening := p.expect(token.LEFT_PARENTHESIS)
	if p.mode&StoreComments != 0 {
		p.comments.Unset()
	}
	var list []*ast.Identifier
	var defaults []ast.Expression
	var targets []ast.Expression
	var rest *ast.Identifier
	hasDefault := false
	hasTarget := false
	for p.token != token.RIGHT_PARENTHESIS && p.token != token.EOF {
		if p.token == token.ELLIPSIS {
			// A rest parameter (...name) must be the last parameter.
			p.next()
			if p.token != token.IDENTIFIER {
				p.expect(token.IDENTIFIER)
				break
			}
			rest = p.parseIdentifier()
			break
		}

		var target ast.Expression
		switch p.token {
		case token.LEFT_BRACKET, token.LEFT_BRACE:
			// A destructuring parameter, e.g. function f({a}, [b]) {}.
			target = p.parseBindingTarget()
			list = append(list, &ast.Identifier{Idx: p.idx})
			hasTarget = true
		case token.IDENTIFIER:
			list = append(list, p.parseIdentifier())
		default:
			p.expect(token.IDENTIFIER)
			if p.token != token.RIGHT_PARENTHESIS {
				p.next()
			}
			continue
		}
		targets = append(targets, target)

		var def ast.Expression
		if p.token == token.ASSIGN {
			if p.mode&StoreComments != 0 {
				p.comments.Unset()
			}
			p.next()
			def = p.parseAssignmentExpression()
			hasDefault = true
		}
		defaults = append(defaults, def)

		if p.token != token.RIGHT_PARENTHESIS {
			if p.mode&StoreComments != 0 {
				p.comments.Unset()
			}
			p.expect(token.COMMA)
		}
	}
	closing := p.expect(token.RIGHT_PARENTHESIS)

	node := &ast.ParameterList{
		Opening: opening,
		List:    list,
		Rest:    rest,
		Closing: closing,
	}
	// Only retain the parallel slices when they carry information, keeping the
	// common (plain-identifier) case unchanged.
	if hasDefault {
		node.Defaults = defaults
	}
	if hasTarget {
		node.Targets = targets
	}

	return node
}

func (p *parser) parseFunctionStatement() *ast.FunctionStatement {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	function := &ast.FunctionStatement{
		Function: p.parseFunction(true),
	}
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(function, comments, ast.LEADING)
	}

	return function
}

func (p *parser) parseFunction(declaration bool) *ast.FunctionLiteral {
	node := &ast.FunctionLiteral{
		Function: p.expect(token.FUNCTION),
	}

	var name *ast.Identifier
	if p.token == token.IDENTIFIER {
		name = p.parseIdentifier()
		if declaration {
			p.scope.declare(&ast.FunctionDeclaration{
				Function: node,
			})
		}
	} else if declaration {
		// Use expect error handling
		p.expect(token.IDENTIFIER)
	}
	if p.mode&StoreComments != 0 {
		p.comments.Unset()
	}
	node.Name = name
	node.ParameterList = p.parseFunctionParameterList()
	p.parseFunctionBlock(node)
	node.Source = p.slice(node.Idx0(), node.Idx1())

	return node
}

func (p *parser) parseClass(declaration bool) *ast.ClassLiteral {
	node := &ast.ClassLiteral{
		Class: p.expect(token.CLASS),
	}

	if p.token == token.IDENTIFIER {
		node.Name = p.parseIdentifier()
	} else if declaration {
		p.expect(token.IDENTIFIER)
	}

	if p.token == token.EXTENDS {
		p.next()
		node.SuperClass = p.parseLeftHandSideExpressionAllowCall()
	}

	p.expect(token.LEFT_BRACE)
	for p.token != token.RIGHT_BRACE && p.token != token.EOF {
		if p.token == token.SEMICOLON {
			p.next()
			continue
		}
		node.Body = append(node.Body, p.parseClassElement())
	}
	node.RightBrace = p.expect(token.RIGHT_BRACE)
	node.Source = p.slice(node.Idx0(), node.Idx1())

	return node
}

func (p *parser) parseClassElement() *ast.ClassElement {
	element := &ast.ClassElement{Kind: "method"}
	methodIdx := p.idx

	// static modifier (unless "static" is itself the method name).
	if p.token == token.IDENTIFIER && p.literal == "static" {
		p.next()
		if p.token == token.LEFT_PARENTHESIS {
			element.Key = "static"
			element.Method = p.parseClassMethodDefinition(methodIdx)
			return element
		}
		element.Static = true
		methodIdx = p.idx
	}

	// get / set accessor (unless "get"/"set" is itself the method name).
	if p.token == token.IDENTIFIER && (p.literal == "get" || p.literal == "set") {
		accessor := p.literal
		p.next()
		if p.token == token.LEFT_PARENTHESIS {
			element.Key = accessor
			element.Method = p.parseClassMethodDefinition(methodIdx)
			return element
		}
		element.Kind = accessor
		p.parseClassElementKey(element)
		element.Method = p.parseClassMethodDefinition(methodIdx)
		return element
	}

	p.parseClassElementKey(element)
	element.Method = p.parseClassMethodDefinition(methodIdx)
	if !element.Static && element.Kind == "method" && element.Key == "constructor" {
		element.Kind = "constructor"
	}
	return element
}

func (p *parser) parseClassElementKey(element *ast.ClassElement) {
	if p.token == token.LEFT_BRACKET {
		keyExpr, _ := p.parseComputedPropertyKey()
		element.KeyExpression = keyExpr
		return
	}
	_, key := p.parseObjectPropertyKey()
	element.Key = key
}

func (p *parser) parseClassMethodDefinition(idx file.Idx) *ast.FunctionLiteral {
	parameterList := p.parseFunctionParameterList()
	node := &ast.FunctionLiteral{
		Function:      idx,
		ParameterList: parameterList,
	}
	p.parseFunctionBlock(node)
	node.Source = p.slice(node.Idx0(), node.Idx1())
	return node
}

func (p *parser) parseFunctionBlock(node *ast.FunctionLiteral) {
	p.openScope()
	inFunction := p.scope.inFunction
	p.scope.inFunction = true
	defer func() {
		p.scope.inFunction = inFunction
		p.closeScope()
	}()
	node.Body = p.parseBlockStatement()
	node.DeclarationList = p.scope.declarationList
}

// arrowParameterList converts an already-parsed expression (the contents of a
// parenthesised group) into a list of arrow function parameters. It only
// accepts plain identifiers; defaults, rest and destructuring are not yet
// supported.
func arrowParameterList(expression ast.Expression, opening, closing file.Idx) (*ast.ParameterList, bool) {
	pl := &ast.ParameterList{Opening: opening, Closing: closing}
	hasTarget := false
	hasDefault := false

	add := func(e ast.Expression) bool {
		var def ast.Expression
		if assign, ok := e.(*ast.AssignExpression); ok {
			if assign.Operator != token.ASSIGN {
				return false
			}
			def = assign.Right
			e = assign.Left
			hasDefault = true
		}

		var name *ast.Identifier
		var target ast.Expression
		switch t := e.(type) {
		case *ast.Identifier:
			name = t
		case *ast.ArrayLiteral, *ast.ObjectLiteral:
			pattern, ok := literalToPattern(e)
			if !ok {
				return false
			}
			target = pattern
			name = &ast.Identifier{Idx: e.Idx0()}
			hasTarget = true
		case *ast.ArrayPattern, *ast.ObjectPattern:
			target = e
			name = &ast.Identifier{Idx: e.Idx0()}
			hasTarget = true
		default:
			return false
		}
		pl.List = append(pl.List, name)
		pl.Targets = append(pl.Targets, target)
		pl.Defaults = append(pl.Defaults, def)
		return true
	}

	if seq, ok := expression.(*ast.SequenceExpression); ok {
		for _, item := range seq.Sequence {
			if !add(item) {
				return nil, false
			}
		}
	} else if !add(expression) {
		return nil, false
	}

	if !hasTarget {
		pl.Targets = nil
	}
	if !hasDefault {
		pl.Defaults = nil
	}
	return pl, true
}

// parseArrowFunction parses the "=> body" portion of an arrow function, given a
// parameter list that has already been collected by the caller. p.token is
// expected to be token.ARROW.
func (p *parser) parseArrowFunction(start file.Idx, params *ast.ParameterList) ast.Expression {
	node := &ast.FunctionLiteral{
		Function:      start,
		IsArrow:       true,
		ParameterList: params,
	}
	if p.mode&StoreComments != 0 {
		p.comments.Unset()
	}
	p.expect(token.ARROW)
	p.parseArrowFunctionBody(node)
	node.Source = p.slice(node.Idx0(), node.Idx1())

	return node
}

func (p *parser) parseArrowFunctionBody(node *ast.FunctionLiteral) {
	if p.token == token.LEFT_BRACE {
		// A block body behaves like an ordinary function body.
		p.openScope()
		inFunction := p.scope.inFunction
		p.scope.inFunction = true
		defer func() {
			p.scope.inFunction = inFunction
			p.closeScope()
		}()
		node.Body = p.parseBlockStatement()
		node.DeclarationList = p.scope.declarationList
		return
	}

	// A concise body is a single expression with an implicit return.
	expression := p.parseAssignmentExpression()
	node.Body = &ast.BlockStatement{
		LeftBrace: expression.Idx0(),
		List: []ast.Statement{
			&ast.ReturnStatement{
				Return:   expression.Idx0(),
				Argument: expression,
			},
		},
		RightBrace: expression.Idx1(),
	}
}

func (p *parser) parseDebuggerStatement() ast.Statement {
	idx := p.expect(token.DEBUGGER)

	node := &ast.DebuggerStatement{
		Debugger: idx,
	}
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, p.comments.FetchAll(), ast.TRAILING)
	}

	p.semicolon()
	return node
}

func (p *parser) parseReturnStatement() ast.Statement {
	idx := p.expect(token.RETURN)
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}

	if !p.scope.inFunction {
		p.error(idx, "Illegal return statement")
		p.nextStatement()
		return &ast.BadStatement{From: idx, To: p.idx}
	}

	node := &ast.ReturnStatement{
		Return: idx,
	}

	if !p.implicitSemicolon && p.token != token.SEMICOLON && p.token != token.RIGHT_BRACE && p.token != token.EOF {
		node.Argument = p.parseExpression()
	}
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, comments, ast.LEADING)
	}

	p.semicolon()

	return node
}

func (p *parser) parseThrowStatement() ast.Statement {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	idx := p.expect(token.THROW)

	if p.implicitSemicolon {
		if p.chr == -1 { // Hackish
			p.error(idx, "Unexpected end of input")
		} else {
			p.error(idx, "Illegal newline after throw")
		}
		p.nextStatement()
		return &ast.BadStatement{From: idx, To: p.idx}
	}

	node := &ast.ThrowStatement{
		Throw:    idx,
		Argument: p.parseExpression(),
	}
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, comments, ast.LEADING)
	}

	p.semicolon()

	return node
}

func (p *parser) parseSwitchStatement() ast.Statement {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	idx := p.expect(token.SWITCH)
	if p.mode&StoreComments != 0 {
		comments = append(comments, p.comments.FetchAll()...)
	}
	p.expect(token.LEFT_PARENTHESIS)
	node := &ast.SwitchStatement{
		Switch:       idx,
		Discriminant: p.parseExpression(),
		Default:      -1,
	}
	p.expect(token.RIGHT_PARENTHESIS)
	if p.mode&StoreComments != 0 {
		comments = append(comments, p.comments.FetchAll()...)
	}

	p.expect(token.LEFT_BRACE)

	inSwitch := p.scope.inSwitch
	p.scope.inSwitch = true
	defer func() {
		p.scope.inSwitch = inSwitch
	}()

	for index := 0; p.token != token.EOF; index++ {
		if p.token == token.RIGHT_BRACE {
			node.RightBrace = p.idx
			p.next()
			break
		}

		clause := p.parseCaseStatement()
		if clause.Test == nil {
			if node.Default != -1 {
				p.error(clause.Case, "Already saw a default in switch")
			}
			node.Default = index
		}
		node.Body = append(node.Body, clause)
	}

	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, comments, ast.LEADING)
	}

	return node
}

func (p *parser) parseWithStatement() ast.Statement {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	idx := p.expect(token.WITH)
	var withComments []*ast.Comment
	if p.mode&StoreComments != 0 {
		withComments = p.comments.FetchAll()
	}

	p.expect(token.LEFT_PARENTHESIS)

	node := &ast.WithStatement{
		With:   idx,
		Object: p.parseExpression(),
	}
	p.expect(token.RIGHT_PARENTHESIS)

	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, comments, ast.LEADING)
		p.comments.CommentMap.AddComments(node, withComments, ast.WITH)
	}

	node.Body = p.parseStatement()

	return node
}

func (p *parser) parseCaseStatement() *ast.CaseStatement {
	node := &ast.CaseStatement{
		Case: p.idx,
	}

	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
		p.comments.Unset()
	}

	if p.token == token.DEFAULT {
		p.next()
	} else {
		p.expect(token.CASE)
		node.Test = p.parseExpression()
	}

	if p.mode&StoreComments != 0 {
		p.comments.Unset()
	}
	p.expect(token.COLON)

	for {
		if p.token == token.EOF ||
			p.token == token.RIGHT_BRACE ||
			p.token == token.CASE ||
			p.token == token.DEFAULT {
			break
		}
		consequent := p.parseStatement()
		node.Consequent = append(node.Consequent, consequent)
	}

	// Link the comments to the case statement
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, comments, ast.LEADING)
	}

	return node
}

func (p *parser) parseIterationStatement() ast.Statement {
	inIteration := p.scope.inIteration
	p.scope.inIteration = true
	defer func() {
		p.scope.inIteration = inIteration
	}()
	return p.parseStatement()
}

// isForOf reports whether the current token is the contextual keyword "of",
// used to recognise a for-of loop.
func (p *parser) isForOf() bool {
	return p.token == token.IDENTIFIER && p.literal == "of"
}

func (p *parser) parseForIn(into ast.Expression) *ast.ForInStatement {
	// Already have consumed "<into> in"

	source := p.parseExpression()
	p.expect(token.RIGHT_PARENTHESIS)
	body := p.parseIterationStatement()

	forin := &ast.ForInStatement{
		Into:   into,
		Source: source,
		Body:   body,
	}

	return forin
}

func (p *parser) parseFor(initializer ast.Expression) *ast.ForStatement {
	// Already have consumed "<initializer> ;"

	var test, update ast.Expression

	if p.token != token.SEMICOLON {
		test = p.parseExpression()
	}
	if p.mode&StoreComments != 0 {
		p.comments.Unset()
	}
	p.expect(token.SEMICOLON)

	if p.token != token.RIGHT_PARENTHESIS {
		update = p.parseExpression()
	}
	p.expect(token.RIGHT_PARENTHESIS)
	body := p.parseIterationStatement()

	forstatement := &ast.ForStatement{
		Initializer: initializer,
		Test:        test,
		Update:      update,
		Body:        body,
	}

	return forstatement
}

func (p *parser) parseForOrForInStatement() ast.Statement {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	idx := p.expect(token.FOR)
	var forComments []*ast.Comment
	if p.mode&StoreComments != 0 {
		forComments = p.comments.FetchAll()
	}
	p.expect(token.LEFT_PARENTHESIS)

	var left []ast.Expression

	forIn := false
	forOf := false
	var lexicalKind token.Token
	if p.token != token.SEMICOLON {
		allowIn := p.scope.allowIn
		p.scope.allowIn = false
		if p.token == token.VAR || p.token == token.LET || p.token == token.CONST {
			tokenIdx := p.idx
			kind := p.token
			var varComments []*ast.Comment
			if p.mode&StoreComments != 0 {
				varComments = p.comments.FetchAll()
				p.comments.Unset()
			}
			p.next()
			var list []ast.Expression
			if kind == token.VAR {
				list = p.parseVariableDeclarationList(tokenIdx)
			} else {
				lexicalKind = kind
				list = p.parseLexicalBindingList()
			}
			if len(list) == 1 && (p.token == token.IN || p.isForOf()) {
				if p.mode&StoreComments != 0 {
					p.comments.Unset()
				}
				forOf = p.isForOf()
				p.next() // in or of
				forIn = true
				left = []ast.Expression{list[0]} // There is only one declaration
			} else {
				left = list
			}
			if p.mode&StoreComments != 0 {
				p.comments.CommentMap.AddComments(left[0], varComments, ast.LEADING)
			}
		} else {
			left = append(left, p.parseExpression())
			if p.token == token.IN || p.isForOf() {
				forOf = p.isForOf()
				p.next()
				forIn = true
			}
		}
		p.scope.allowIn = allowIn
	}

	if forIn {
		switch left[0].(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression, *ast.VariableExpression:
			// These are all acceptable
		case *ast.ArrayPattern, *ast.ObjectPattern:
			// A binding pattern declared with var/let/const, e.g.
			// for (const [a, b] of x) or for (let {x} in obj).
		case *ast.ArrayLiteral, *ast.ObjectLiteral:
			// An assignment destructuring target, e.g. for ([a, b] of x)
			// or for ({a} in obj). Reinterpret the literal as a pattern.
			pattern, ok := literalToPattern(left[0])
			if !ok {
				p.error(idx, "Invalid left-hand side in for-in")
				p.nextStatement()
				return &ast.BadStatement{From: idx, To: p.idx}
			}
			left[0] = pattern
		default:
			p.error(idx, "Invalid left-hand side in for-in")
			p.nextStatement()
			return &ast.BadStatement{From: idx, To: p.idx}
		}

		forin := p.parseForIn(left[0])
		forin.For = idx
		forin.Lexical = lexicalKind
		forin.Of = forOf
		if p.mode&StoreComments != 0 {
			p.comments.CommentMap.AddComments(forin, comments, ast.LEADING)
			p.comments.CommentMap.AddComments(forin, forComments, ast.FOR)
		}
		return forin
	}

	if p.mode&StoreComments != 0 {
		p.comments.Unset()
	}
	p.expect(token.SEMICOLON)
	initializer := &ast.SequenceExpression{Sequence: left}
	forstatement := p.parseFor(initializer)
	forstatement.For = idx
	forstatement.Lexical = lexicalKind
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(forstatement, comments, ast.LEADING)
		p.comments.CommentMap.AddComments(forstatement, forComments, ast.FOR)
	}
	return forstatement
}

func (p *parser) parseVariableStatement() *ast.VariableStatement {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	idx := p.expect(token.VAR)

	list := p.parseVariableDeclarationList(idx)

	statement := &ast.VariableStatement{
		Var:  idx,
		List: list,
	}
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(statement, comments, ast.LEADING)
		p.comments.Unset()
	}
	p.semicolon()

	return statement
}

func (p *parser) parseLexicalDeclaration() *ast.LexicalDeclaration {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	kind := p.token // token.LET or token.CONST
	idx := p.expect(kind)

	// Parse the binding list without registering it for var-style hoisting;
	// lexical bindings are scoped to their enclosing block.
	list := p.parseLexicalBindingList()

	statement := &ast.LexicalDeclaration{
		Idx:   idx,
		Token: kind,
		List:  list,
	}
	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(statement, comments, ast.LEADING)
		p.comments.Unset()
	}
	p.semicolon()

	return statement
}

// parseLexicalBindingList parses a comma-separated list of let/const bindings.
// Unlike parseVariableDeclarationList it does not declare the names in the
// surrounding function scope, since lexical declarations are block-scoped.
func (p *parser) parseLexicalBindingList() []ast.Expression {
	var list []ast.Expression
	for {
		list = append(list, p.parseVariableDeclaration(nil))
		if p.token != token.COMMA {
			break
		}
		p.next()
	}
	return list
}

func (p *parser) parseDoWhileStatement() ast.Statement {
	inIteration := p.scope.inIteration
	p.scope.inIteration = true
	defer func() {
		p.scope.inIteration = inIteration
	}()

	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	idx := p.expect(token.DO)
	var doComments []*ast.Comment
	if p.mode&StoreComments != 0 {
		doComments = p.comments.FetchAll()
	}

	node := &ast.DoWhileStatement{Do: idx}
	if p.token == token.LEFT_BRACE {
		node.Body = p.parseBlockStatement()
	} else {
		node.Body = p.parseStatement()
	}

	p.expect(token.WHILE)
	var whileComments []*ast.Comment
	if p.mode&StoreComments != 0 {
		whileComments = p.comments.FetchAll()
	}
	p.expect(token.LEFT_PARENTHESIS)
	node.Test = p.parseExpression()
	node.RightParenthesis = p.expect(token.RIGHT_PARENTHESIS)

	p.implicitSemicolon = true
	p.optionalSemicolon()

	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, comments, ast.LEADING)
		p.comments.CommentMap.AddComments(node, doComments, ast.DO)
		p.comments.CommentMap.AddComments(node, whileComments, ast.WHILE)
	}

	return node
}

func (p *parser) parseWhileStatement() ast.Statement {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	idx := p.expect(token.WHILE)

	var whileComments []*ast.Comment
	if p.mode&StoreComments != 0 {
		whileComments = p.comments.FetchAll()
	}

	p.expect(token.LEFT_PARENTHESIS)
	node := &ast.WhileStatement{
		While: idx,
		Test:  p.parseExpression(),
	}
	p.expect(token.RIGHT_PARENTHESIS)
	node.Body = p.parseIterationStatement()

	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, comments, ast.LEADING)
		p.comments.CommentMap.AddComments(node, whileComments, ast.WHILE)
	}

	return node
}

func (p *parser) parseIfStatement() ast.Statement {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	pos := p.expect(token.IF)
	var ifComments []*ast.Comment
	if p.mode&StoreComments != 0 {
		ifComments = p.comments.FetchAll()
	}

	p.expect(token.LEFT_PARENTHESIS)
	node := &ast.IfStatement{
		If:   pos,
		Test: p.parseExpression(),
	}
	p.expect(token.RIGHT_PARENTHESIS)
	if p.token == token.LEFT_BRACE {
		node.Consequent = p.parseBlockStatement()
	} else {
		node.Consequent = p.parseStatement()
	}

	if p.token == token.ELSE {
		p.next()
		node.Alternate = p.parseStatement()
	}

	if p.mode&StoreComments != 0 {
		p.comments.CommentMap.AddComments(node, comments, ast.LEADING)
		p.comments.CommentMap.AddComments(node, ifComments, ast.IF)
	}

	return node
}

func (p *parser) parseSourceElement() ast.Statement {
	statement := p.parseStatement()
	return statement
}

func (p *parser) parseSourceElements() []ast.Statement {
	body := []ast.Statement(nil)

	for {
		if p.token != token.STRING {
			break
		}
		body = append(body, p.parseSourceElement())
	}

	for p.token != token.EOF {
		body = append(body, p.parseSourceElement())
	}

	return body
}

func (p *parser) parseProgram() *ast.Program {
	p.openScope()
	defer p.closeScope()
	return &ast.Program{
		Body:            p.parseSourceElements(),
		DeclarationList: p.scope.declarationList,
		File:            p.file,
	}
}

func (p *parser) parseBreakStatement() ast.Statement {
	var comments []*ast.Comment
	if p.mode&StoreComments != 0 {
		comments = p.comments.FetchAll()
	}
	idx := p.expect(token.BREAK)
	semicolon := p.implicitSemicolon
	if p.token == token.SEMICOLON {
		semicolon = true
		p.next()
	}

	if semicolon || p.token == token.RIGHT_BRACE {
		p.implicitSemicolon = false
		if !p.scope.inIteration && !p.scope.inSwitch {
			goto illegal
		}
		breakStatement := &ast.BranchStatement{
			Idx:   idx,
			Token: token.BREAK,
		}

		if p.mode&StoreComments != 0 {
			p.comments.CommentMap.AddComments(breakStatement, comments, ast.LEADING)
			p.comments.CommentMap.AddComments(breakStatement, p.comments.FetchAll(), ast.TRAILING)
		}

		return breakStatement
	}

	if p.token == token.IDENTIFIER {
		identifier := p.parseIdentifier()
		if !p.scope.hasLabel(identifier.Name) {
			p.error(idx, "Undefined label '%s'", identifier.Name)
			return &ast.BadStatement{From: idx, To: identifier.Idx1()}
		}
		p.semicolon()
		breakStatement := &ast.BranchStatement{
			Idx:   idx,
			Token: token.BREAK,
			Label: identifier,
		}
		if p.mode&StoreComments != 0 {
			p.comments.CommentMap.AddComments(breakStatement, comments, ast.LEADING)
		}

		return breakStatement
	}

	p.expect(token.IDENTIFIER)

illegal:
	p.error(idx, "Illegal break statement")
	p.nextStatement()
	return &ast.BadStatement{From: idx, To: p.idx}
}

func (p *parser) parseContinueStatement() ast.Statement {
	idx := p.expect(token.CONTINUE)
	semicolon := p.implicitSemicolon
	if p.token == token.SEMICOLON {
		semicolon = true
		p.next()
	}

	if semicolon || p.token == token.RIGHT_BRACE {
		p.implicitSemicolon = false
		if !p.scope.inIteration {
			goto illegal
		}
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.CONTINUE,
		}
	}

	if p.token == token.IDENTIFIER {
		identifier := p.parseIdentifier()
		if !p.scope.hasLabel(identifier.Name) {
			p.error(idx, "Undefined label '%s'", identifier.Name)
			return &ast.BadStatement{From: idx, To: identifier.Idx1()}
		}
		if !p.scope.inIteration {
			goto illegal
		}
		p.semicolon()
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.CONTINUE,
			Label: identifier,
		}
	}

	p.expect(token.IDENTIFIER)

illegal:
	p.error(idx, "Illegal continue statement")
	p.nextStatement()
	return &ast.BadStatement{From: idx, To: p.idx}
}

// Find the next statement after an error (recover).
func (p *parser) nextStatement() {
	for {
		switch p.token {
		case token.BREAK, token.CONTINUE,
			token.FOR, token.IF, token.RETURN, token.SWITCH,
			token.VAR, token.DO, token.TRY, token.WITH,
			token.WHILE, token.THROW, token.CATCH, token.FINALLY:
			// Return only if parser made some progress since last
			// sync or if it has not reached 10 next calls without
			// progress. Otherwise consume at least one token to
			// avoid an endless parser loop
			if p.idx == p.recover.idx && p.recover.count < 10 {
				p.recover.count++
				return
			}
			if p.idx > p.recover.idx {
				p.recover.idx = p.idx
				p.recover.count = 0
				return
			}
			// Reaching here indicates a parser bug, likely an
			// incorrect token list in this function, but it only
			// leads to skipping of possibly correct code if a
			// previous error is present, and thus is preferred
			// over a non-terminating parse.
		case token.EOF:
			return
		}
		p.next()
	}
}
