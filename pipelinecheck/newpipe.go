package pipelinecheck

import (
	"go/ast"
	"go/token"
)

// DefaultAllowNewPipeDirective exempts a function from the new-pipe
// rule when followed by a non-empty reason.
const DefaultAllowNewPipeDirective = "fp-go:allow-new-pipe"

const newPipeMessage = "new() wraps a pipeline: return the value " +
	"from the pipeline and take its address (&v) only at the API " +
	"call that needs a pointer"

// checkNoNewPipe flags new(F.PipeN(...)) and new(F.FlowN(...)) in
// files that import fp-go. A pipeline builds a value. A pointer to
// it belongs at the call that needs the pointer, not in the builder.
func checkNoNewPipe(
	fset *token.FileSet,
	f *ast.File,
	allow string,
) []Violation {
	aliases := functionAliases(f)
	if len(aliases) == 0 {
		return nil
	}
	var violations []Violation
	reported := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isBuiltinNew(call) {
			return true
		}
		inner, ok := unparen(call.Args[0]).(*ast.CallExpr)
		if !ok || !isPipeOrFlowCall(inner, aliases) {
			return true
		}
		fnName := enclosingFuncName(f, call.Pos())
		if exemptionFor(&violations, fset, f, fnName,
			allow, reported) == exempt {
			return true
		}
		violations = append(violations, Violation{
			Position: fset.Position(call.Pos()),
			Function: fnName,
			Message:  newPipeMessage,
		})
		return true
	})
	return violations
}

// isBuiltinNew reports whether call is new(x) with one argument.
func isBuiltinNew(call *ast.CallExpr) bool {
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == "new" && len(call.Args) == 1
}

// CheckNoNewPipe scans cfg.Roots and returns every new(F.Pipe*(...))
// or new(F.Flow*(...)) call. cfg.AllowNewPipeDirective exempts a
// function (defaults to DefaultAllowNewPipeDirective).
func CheckNoNewPipe(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	return scanFiles(cfg, func(
		fset *token.FileSet,
		f *ast.File,
	) []Violation {
		return checkNoNewPipe(fset, f, cfg.AllowNewPipeDirective)
	})
}

// RequireNoNewPipe runs CheckNoNewPipe and fails r on every
// violation or scan error.
func RequireNoNewPipe(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoNewPipe(cfg)
	reportAll(r, vs, err)
}
