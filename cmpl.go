package otto

import (
	"github.com/incident-io/otto/ast"
	"github.com/incident-io/otto/file"
)

type compiler struct {
	file    *file.File
	program *ast.Program
}
