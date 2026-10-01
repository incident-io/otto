package otto

import (
	"github.com/incident-io/otto/v2/ast"
	"github.com/incident-io/otto/v2/file"
)

type compiler struct {
	file    *file.File
	program *ast.Program
}
