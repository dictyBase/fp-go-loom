package pipelinecheck

import (
	"go/ast"
	"go/token"
)

// DefaultAllowDirectApplyDirective exempts a function from the
// direct-apply rule when followed by a non-empty reason.
const DefaultAllowDirectApplyDirective = "fp-go:allow-direct-apply"

const directApplyMessage = "combinator applied directly: " +
	"comb(args)(value) — use F.Pipe1(value, comb(args)) " +
	"so the data flows left to right"

// unparen strips parentheses from expr.
func unparen(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.X
	}
}

// isFpGoCall reports whether call invokes a function of an fp-go or
// fp-go-loom package, for example IOE.Map[error](f) or A.Map(f).
func isFpGoCall(
	call *ast.CallExpr,
	aliases map[string]bool,
) bool {
	sel := pipeSelector(call.Fun)
	return sel != nil && isFunctionAlias(sel.X, aliases)
}

// checkNoDirectApply flags comb(args)(value) in files that import
// fp-go: a call whose callee is itself a call to an fp-go function.
// A.Map(f)(xs) reads inside-out. F.Pipe1(xs, A.Map(f)) reads left to
// right. Only this shape is reported. A call such as
// P.Or(hasCode(x)) is a valid partial application, and syntax alone
// cannot tell it from data nesting, so it is not reported.
func checkNoDirectApply(
	fset *token.FileSet,
	f *ast.File,
	allow string,
) []Violation {
	aliases := fpGoAliases(f)
	if len(aliases) == 0 {
		return nil
	}
	var violations []Violation
	reported := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		inner, ok := unparen(call.Fun).(*ast.CallExpr)
		if !ok || !isFpGoCall(inner, aliases) {
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
			Message:  directApplyMessage,
		})
		return true
	})
	return violations
}

// CheckNoDirectApply scans cfg.Roots and returns every
// comb(args)(value) call in files that import fp-go. It replaces the
// manual ripgrep gate for this shape. cfg.AllowDirectApplyDirective
// exempts a function (defaults to
// DefaultAllowDirectApplyDirective).
func CheckNoDirectApply(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	return scanFiles(cfg, func(
		fset *token.FileSet,
		f *ast.File,
	) []Violation {
		return checkNoDirectApply(
			fset, f, cfg.AllowDirectApplyDirective)
	})
}

// RequireNoDirectApply runs CheckNoDirectApply and fails r on every
// violation or scan error.
func RequireNoDirectApply(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoDirectApply(cfg)
	reportAll(r, vs, err)
}
