package pipelinecheck

import (
	"go/ast"
	"go/token"
	"strings"
)

// checkRedundantFold flags E.Fold / E.Match calls whose Right (second)
// arm discards its argument by returning nil. Such a Fold is just
// "extract the Left error, return nil on Right", which is E.ToError:
//
//	E.Fold(F.Identity[error], func(A) error { return nil }) // -> E.ToError[A]
//	E.Fold(wrap, F.Constant1[A](error(nil)))               // -> F.Flow2(E.ToError[A], wrap)
//
// Two discard shapes are detected:
//   - an anonymous func whose body is a single `return nil` (the
//     parameter is definitionally unreferenced — a single bare return
//     of nil cannot mention it), or
//   - a function-package Constant* combinator (Constant, Constant1,
//     Constant2, ...) whose argument is nil or a T(nil) conversion.
//
// Reserve Fold for the case where both arms produce a real value from
// their input (durable rule 10). Only the Right (success) arm is
// inspected; a Left arm that ignores its argument is out of scope.
// IOE.Fold over IOEither is not inspected — E.ToError is Either-only.
func checkRedundantFold(
	fset *token.FileSet,
	f *ast.File,
	aliases map[string]bool,
	allow string,
) []Violation {
	var violations []Violation
	reported := make(map[string]bool)
	fnAliases := functionAliases(f)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel := pipeSelector(call.Fun)
		if sel == nil {
			return true
		}
		if sel.Sel.Name != "Fold" && sel.Sel.Name != "Match" {
			return true
		}
		if !isEitherAlias(sel.X, aliases) {
			return true
		}
		// Fold(onLeft, onRight): the success arm is the second
		// argument. A curried application E.Fold(a, b)(x) visits
		// the inner two-arg Fold call separately; the outer call's
		// Fun is a CallExpr, which pipeSelector skips, so there is
		// no double count.
		if len(call.Args) < 2 {
			return true
		}
		if !isDiscardRightArm(call.Args[1], fnAliases) {
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
			Message: "E.Fold/Match Right arm " +
				"returns nil — use E.ToError[A] (or " +
				"F.Flow2(E.ToError[A], leftTransform)) " +
				"instead of the discard arm " +
				"(fp-go-pipe-flow Redundant Fold)",
		})
		return true
	})
	return violations
}

// isEitherAlias reports whether x is an identifier resolving to the
// fp-go either package in this file.
func isEitherAlias(x ast.Expr, aliases map[string]bool) bool {
	ident, ok := x.(*ast.Ident)
	return ok && aliases[ident.Name]
}

// isDiscardRightArm reports whether arm is a Fold success arm that
// discards its argument by returning nil: either an anonymous func
// whose only statement is `return nil`, or a function-package
// Constant* combinator wrapping nil.
func isDiscardRightArm(
	arm ast.Expr,
	fnAliases map[string]bool,
) bool {
	switch v := arm.(type) {
	case *ast.FuncLit:
		return isDiscardFuncLit(v)
	case *ast.CallExpr:
		return isConstantNil(v, fnAliases)
	}
	return false
}

// isDiscardFuncLit reports whether fn is `func(A) error { return nil }`
// — a single bare return of the untyped nil. A body of exactly one
// `return nil` statement cannot reference its parameter, so no separate
// use-check is needed; a multi-statement body or a non-nil return is
// not a discard.
func isDiscardFuncLit(fn *ast.FuncLit) bool {
	if len(fn.Body.List) != 1 {
		return false
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok {
		return false
	}
	if len(ret.Results) != 1 {
		return false
	}
	return isNilIdent(ret.Results[0])
}

// isConstantNil reports whether call is a function-package Constant*
// combinator (Constant, Constant1, Constant2, …) whose single
// argument is nil or a T(nil) conversion. That wraps nil as a
// constant function — the combinator form of the discard arm. A
// Constant wrapping a real value is not a ToError equivalent and is
// not flagged.
func isConstantNil(
	call *ast.CallExpr,
	fnAliases map[string]bool,
) bool {
	sel := pipeSelector(call.Fun)
	if sel == nil {
		return false
	}
	if !strings.HasPrefix(sel.Sel.Name, "Constant") {
		return false
	}
	if !isFunctionAlias(sel.X, fnAliases) {
		return false
	}
	if len(call.Args) != 1 {
		return false
	}
	return isNilLiteral(call.Args[0])
}

// isNilIdent reports whether expr is the untyped nil identifier.
func isNilIdent(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "nil"
}

// isNilLiteral reports whether expr is nil, either as the bare
// identifier or a T(nil) conversion such as error(nil).
func isNilLiteral(expr ast.Expr) bool {
	if isNilIdent(expr) {
		return true
	}
	conv, ok := expr.(*ast.CallExpr)
	if !ok || len(conv.Args) != 1 {
		return false
	}
	if _, ok := conv.Fun.(*ast.Ident); !ok {
		return false
	}
	return isNilIdent(conv.Args[0])
}
