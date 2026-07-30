package pipelinecheck

import (
	"go/ast"
	"go/token"
)

// checkNoIfErrInTryCatch flags `if err != nil` and `if nil != err`
// statements inside IOE.TryCatchError callback literals. Such a guard
// buries error wrapping inside the effect constructor; TryCatchError
// should return the raw SDK result, and projection/wrapping belongs in
// IOE.Map / IOE.MapLeft (durable rule 3). Only the not-equal form is
// flagged: an `if err == nil` success-branch is a different shape and
// is out of scope. Named callback references (not func literals) are
// not inspected.
func checkNoIfErrInTryCatch(
	fset *token.FileSet,
	f *ast.File,
	aliases map[string]bool,
	allow string,
	_ map[string]string,
) []Violation {
	var violations []Violation
	reported := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel := pipeSelector(call.Fun)
		if sel == nil || sel.Sel.Name != "TryCatchError" {
			return true
		}
		if !isFunctionAlias(sel.X, aliases) {
			return true
		}
		fn := tryCatchCallback(call)
		if fn == nil {
			return true
		}
		fnName := enclosingFuncName(f, call.Pos())
		if exemptionFor(&violations, fset, f, fnName,
			allow, reported) == exempt {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ifs, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			if !isErrNilCompare(ifs.Cond) {
				return true
			}
			violations = append(violations, Violation{
				Position: fset.Position(ifs.Pos()),
				Function: fnName,
				Message: "if err != nil inside " +
					"IOE.TryCatchError — return the raw " +
					"effect and wrap with IOE.MapLeft " +
					"(durable rule 3)",
			})
			return true
		})
		return true
	})
	return violations
}

// tryCatchCallback returns the first FuncLit argument of a
// TryCatchError call, or nil when the callback is a named reference
// (which is not inspected across the file).
func tryCatchCallback(call *ast.CallExpr) *ast.FuncLit {
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.FuncLit); ok {
			return lit
		}
	}
	return nil
}

// isErrNilCompare reports whether cond is `err != nil` or
// `nil != err`, where err is any identifier literally named "err".
// Only the not-equal form is flagged (the durable rule targets the
// wrap-and-return pattern); it does not chase shadowing.
func isErrNilCompare(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	if bin.Op != token.NEQ {
		return false
	}
	return isErrIdentNil(bin.X, bin.Y) ||
		isErrIdentNil(bin.Y, bin.X)
}

// isErrIdentNil reports whether lhs is an identifier named "err" and
// rhs is the untyped nil.
func isErrIdentNil(lhs, rhs ast.Expr) bool {
	id, ok := lhs.(*ast.Ident)
	if !ok || id.Name != "err" {
		return false
	}
	r, ok := rhs.(*ast.Ident)
	return ok && r.Name == "nil"
}
