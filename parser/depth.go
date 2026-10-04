package parser

import (
	"github.com/robertkrimen/otto/ast"
)

// maxDepth bounds the nesting depth of parsed source. Deeper input is
// rejected with a syntax error rather than exhausting the Go stack while
// parsing, compiling or evaluating it.
const maxDepth = 3000

// maxTemplateDepth bounds how deeply template literals nest inside each
// other's substitutions. Each level re-scans its substitution's source, so
// unbounded nesting makes parsing quadratic in the source length.
const maxTemplateDepth = 32

// interruptEvery is how many tokens are scanned between calls to the
// interrupt callback.
const interruptEvery = 1024

// Option configures optional parser behaviour.
type Option func(*parser)

// WithInterrupt makes the parser call fn periodically while parsing. fn may
// panic to abandon a parse that is taking too long; the panic propagates to the
// caller.
func WithInterrupt(fn func()) Option {
	return func(p *parser) {
		p.interrupt = fn
	}
}

type tooDeepPanic struct{}

func (p *parser) enter() {
	p.depth++
	if p.depth > maxDepth {
		panic(tooDeepPanic{})
	}
}

func (p *parser) leave() {
	p.depth--
}

// enterChain records one more level of a left-associative chain such as
// a+b+c or a.b.c, which the parser builds iteratively but which nests in the
// resulting AST.
func (p *parser) enterChain(chain *int) {
	*chain++
	p.enter()
}

func (p *parser) leaveChain(chain *int) {
	p.depth -= *chain
}

// parseGuarded runs p.parse, turning an exceeded nesting depth into a syntax
// error.
func (p *parser) parseGuarded() (program *ast.Program, err error) { //nolint:nonamedreturns
	defer func() {
		if caught := recover(); caught != nil {
			if _, ok := caught.(tooDeepPanic); !ok {
				panic(caught)
			}
			p.errors = nil
			p.error(p.idx, "Maximum nesting depth exceeded")
			program, err = &ast.Program{}, p.errors.Err()
		}
	}()
	return p.parse()
}

func (p *parser) tick() {
	if p.interrupt == nil {
		return
	}
	p.tokens++
	if p.tokens%interruptEvery == 0 {
		p.interrupt()
	}
}

// tickBytes accounts for scanning n bytes of source without producing tokens,
// counting every 64 bytes as one token.
func (p *parser) tickBytes(n int) {
	if p.interrupt == nil {
		return
	}
	before := p.tokens / interruptEvery
	p.tokens += n/64 + 1
	if p.tokens/interruptEvery != before {
		p.interrupt()
	}
}
