package pipelinecheck

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
)

// DefaultAllowFlapDirective exempts a function from the flap rule
// when followed by a non-empty reason.
const DefaultAllowFlapDirective = "fp-go:allow-flap"

// EndomorphismPkgPath is the import path of the fp-go endomorphism
// package. A Flap that feeds an endomorphism step is allowed.
const EndomorphismPkgPath = "github.com/IBM/fp-go/v2/endomorphism"

const flapMessage = "point-free Flap or Flip: write a named plain " +
	"function that calls f(x) or takes its arguments in the order " +
	"you need"

// flapNames are the fp-go combinators that apply a function held as
// data to a value, or swap the arguments of a curried function.
var flapNames = map[string]bool{
	"Flap":      true,
	"MonadFlap": true,
}

// checkNoFlap flags each use of Flap or MonadFlap of any fp-go v2
// package, and each use of function.Flip, called or passed as a
// value. A Flap use is allowed when
// an enclosing call also takes an endomorphism-package step, as in
// F.Pipe2(edits, A.Flap[En.Endomorphism[T]](x), En.Reduce[T]).
func checkNoFlap(
	fset *token.FileSet,
	f *ast.File,
	allow string,
) []Violation {
	fpAliases := fpGoV2Aliases(f)
	if len(fpAliases) == 0 {
		return nil
	}
	fnAliases := functionAliases(f)
	enAliases := importAliasesFor(f, EndomorphismPkgPath)
	var violations []Violation
	reported := make(map[string]bool)
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		isFlap := flapNames[sel.Sel.Name] &&
			isFunctionAlias(sel.X, fpAliases) &&
			!feedsEndomorphism(stack, enAliases)
		isFlip := sel.Sel.Name == "Flip" &&
			isFunctionAlias(sel.X, fnAliases)
		if !isFlap && !isFlip {
			return true
		}
		fnName := enclosingFuncName(f, sel.Pos())
		if exemptionFor(&violations, fset, f, fnName,
			allow, reported) == exempt {
			return true
		}
		violations = append(violations, Violation{
			Position: fset.Position(sel.Pos()),
			Function: fnName,
			Message:  flapMessage,
		})
		return true
	})
	return violations
}

// fpGoV2Aliases returns the import aliases of fp-go v2 packages only.
// A loom package can export its own Flap, which this rule ignores.
func fpGoV2Aliases(f *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if !strings.HasPrefix(path, fpGoPathPrefix) {
			continue
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name != "_" {
			aliases[name] = true
		}
	}
	return aliases
}

// feedsEndomorphism reports whether an enclosing call on the stack
// has an argument that refers to the endomorphism package, such as
// En.Reduce or En.Endomorphism[T].
func feedsEndomorphism(
	stack []ast.Node,
	enAliases map[string]bool,
) bool {
	if len(enAliases) == 0 {
		return false
	}
	for i := len(stack) - 2; i >= 0; i-- {
		call, ok := stack[i].(*ast.CallExpr)
		if !ok {
			continue
		}
		for _, arg := range call.Args {
			if refersTo(arg, enAliases) {
				return true
			}
		}
	}
	return false
}

// refersTo reports whether expr contains a selector on one of the
// given package aliases.
func refersTo(expr ast.Expr, aliases map[string]bool) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if ok && isFunctionAlias(sel.X, aliases) {
			found = true
		}
		return !found
	})
	return found
}

// CheckNoFlap scans cfg.Roots and returns every point-free Flap or
// Flip use. cfg.AllowFlapDirective exempts a function (defaults to
// DefaultAllowFlapDirective).
func CheckNoFlap(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	return scanFiles(cfg, func(
		fset *token.FileSet,
		f *ast.File,
	) []Violation {
		return checkNoFlap(fset, f, cfg.AllowFlapDirective)
	})
}

// RequireNoFlap runs CheckNoFlap and fails r on every violation or
// scan error.
func RequireNoFlap(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoFlap(cfg)
	reportAll(r, vs, err)
}
