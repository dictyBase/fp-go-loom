package loomcheck

import (
	"go/ast"
	"go/token"
)

// genericMessage is used when StrictAllPredicates is on and the shape
// has no known replacement.
const genericMessage = "hand-rolled predicate literal — check the " +
	"fp-go-loom catalog (predord / predarrays / predbytes / " +
	"predstrings / predfs) before writing one by hand"

// prefix prepended to every specific suggestion.
const suggestPrefix = "hand-rolled predicate — use "

// checkHandRolledPredicates flags bool-returning function literals
// whose single return expression duplicates a loom combinator.
func checkHandRolledPredicates(
	fset *token.FileSet,
	f *ast.File,
	allow string,
	strict bool,
) []Violation {
	var out []Violation
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.FuncLit)
		if !ok || !isPredicateLit(lit) {
			return true
		}
		expr := soleReturnExpr(lit)
		if expr == nil {
			return true
		}
		msg, known := suggestFor(expr)
		if !known {
			if !strict {
				return true
			}
			msg = genericMessage
		}
		if isExempt(f, lit.Pos(), allow) {
			return true
		}
		out = append(out, Violation{
			Position: fset.Position(lit.Pos()),
			Function: enclosingFuncName(f, lit.Pos()),
			Message:  msg,
		})
		return true
	})
	return out
}

// isPredicateLit reports whether lit has the shape func(T) bool.
func isPredicateLit(lit *ast.FuncLit) bool {
	t := lit.Type
	if t.Params == nil || len(t.Params.List) != 1 {
		return false
	}
	if len(t.Params.List[0].Names) > 1 {
		return false
	}
	if t.Results == nil || len(t.Results.List) != 1 {
		return false
	}
	ident, ok := t.Results.List[0].Type.(*ast.Ident)
	return ok && ident.Name == "bool"
}

// soleReturnExpr returns the expression of a body consisting of one
// return statement with one result, else nil.
func soleReturnExpr(lit *ast.FuncLit) ast.Expr {
	if lit.Body == nil || len(lit.Body.List) != 1 {
		return nil
	}
	ret, ok := lit.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil
	}
	return ret.Results[0]
}

// suggestFor maps a predicate expression onto its loom replacement.
func suggestFor(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		return suggestUnary(e)
	case *ast.BinaryExpr:
		return suggestBinary(e)
	case *ast.CallExpr:
		return suggestCall(e)
	case *ast.SelectorExpr:
		return "", false
	default:
		return "", false
	}
}

// suggestUnary handles negation.
func suggestUnary(e *ast.UnaryExpr) (string, bool) {
	if e.Op != token.NOT {
		return "", false
	}
	return suggestPrefix +
		"P.Not(<predicate>) rather than a `!` literal", true
}

// suggestBinary handles comparisons and range conjunctions.
func suggestBinary(e *ast.BinaryExpr) (string, bool) {
	if e.Op == token.LAND {
		return suggestRange(e)
	}
	if msg, ok := suggestLenCompare(e); ok {
		return msg, true
	}
	return suggestEquality(e)
}

// suggestLenCompare handles len(x) <op> n.
func suggestLenCompare(e *ast.BinaryExpr) (string, bool) {
	if !isLenCall(e.X) || !isIntLiteral(e.Y) {
		return "", false
	}
	zero := literalIsZero(e.Y)
	switch {
	case e.Op == token.GTR && zero,
		e.Op == token.NEQ && zero:
		return suggestPrefix +
			"predarrays.IsNonEmpty[T]() (or " +
			"predbytes.HasPositiveLen / S.IsNonEmpty)", true
	case e.Op == token.GEQ:
		return suggestPrefix +
			"predarrays.MinLen[T](n) or predord.MinStrLen(n)", true
	case e.Op == token.LEQ:
		return suggestPrefix +
			"predarrays.MaxLen[T](n) or predord.MaxStrLen(n)", true
	case e.Op == token.EQL:
		return suggestPrefix +
			"predarrays.LenEq[T](n) or predord.StrLenEq(n)", true
	default:
		return "", false
	}
}

// suggestEquality handles x == lit and x != lit.
func suggestEquality(e *ast.BinaryExpr) (string, bool) {
	if !isBasicLiteral(e.Y) {
		return "", false
	}
	switch e.Op {
	case token.EQL:
		return suggestPrefix +
			"predord.StrEq(v) or EQ.Equals(predord.IntEq)(v)", true
	case token.NEQ:
		return suggestPrefix +
			"predord.NotEqualStr / NotEqualInt / NotEqualF64", true
	default:
		return "", false
	}
}

// suggestRange handles lo <= x && x < hi style conjunctions.
func suggestRange(e *ast.BinaryExpr) (string, bool) {
	left, okL := e.X.(*ast.BinaryExpr)
	right, okR := e.Y.(*ast.BinaryExpr)
	if !okL || !okR {
		return "", false
	}
	if !isOrderOp(left.Op) || !isOrderOp(right.Op) {
		return "", false
	}
	if right.Op == token.LEQ || right.Op == token.GEQ {
		return suggestPrefix +
			"predord.IntBetweenInclusive(lo, hi)", true
	}
	return suggestPrefix + "predord.IntBetween(lo, hi)", true
}

// suggestCall handles stdlib and method-call predicates.
func suggestCall(e *ast.CallExpr) (string, bool) {
	sel, ok := e.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	if msg, found := stdlibSuggestions[selectorPath(sel)]; found {
		return suggestPrefix + msg, true
	}
	return suggestMethod(sel)
}

// suggestMethod handles fi.IsDir() and fi.Mode().IsRegular().
func suggestMethod(sel *ast.SelectorExpr) (string, bool) {
	switch sel.Sel.Name {
	case "IsDir":
		return suggestPrefix +
			"predfs.IsDirInfo (or predfs.IsDir[S](get))", true
	case "IsRegular":
		return suggestPrefix +
				"predfs.IsRegularInfo (or predfs.IsRegular[S](get))",
			true
	default:
		return "", false
	}
}

// stdlibSuggestions maps a qualified stdlib call onto its loom
// replacement.
var stdlibSuggestions = map[string]string{
	"strings.HasSuffix":    "predstrings.HasSuffix(suffix)",
	"strings.ContainsFunc": "predstrings.ContainsRuneClass(pred)",
	"strings.ContainsRune": "predstrings.ContainsRuneClass(pred)",
	"strings.EqualFold":    "predord.StrEq after normalization",
	"utf8.ValidString":     "a predord/predstrings combinator",
}

// selectorPath renders pkg.Name for a qualified selector.
func selectorPath(sel *ast.SelectorExpr) string {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name + "." + sel.Sel.Name
}

// isOrderOp reports whether op is an ordering comparison.
func isOrderOp(op token.Token) bool {
	switch op {
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	default:
		return false
	}
}

// isLenCall reports whether expr is a call to the builtin len.
func isLenCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "len"
}

// isBasicLiteral reports whether expr is a literal constant.
func isBasicLiteral(expr ast.Expr) bool {
	_, ok := expr.(*ast.BasicLit)
	return ok
}

// isIntLiteral reports whether expr is an integer literal.
func isIntLiteral(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.INT
}

// literalIsZero reports whether expr is the integer literal 0.
func literalIsZero(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == "0"
}
