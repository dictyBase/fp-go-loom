package pipelinecheck

import (
	"go/ast"
	"go/token"
)

// DefaultAllowBoolExprDirective exempts a function from the
// bool-expression rule when followed by a non-empty reason.
const DefaultAllowBoolExprDirective = "fp-go:allow-bool-expr"

const boolExprMessage = "hand-written boolean expression in a " +
	"predicate: build it from predicates (P.And, P.Or, P.Not, " +
	"predord, F.Flow2(getter, pred)) instead of ==, !=, <, >, " +
	"&&, ||, or !"

// boolOperators are the operators that make a hand-written
// predicate body.
var boolOperators = map[token.Token]bool{
	token.EQL: true, token.NEQ: true,
	token.LSS: true, token.LEQ: true,
	token.GTR: true, token.GEQ: true,
	token.LAND: true, token.LOR: true,
}

// checkNoBoolExpr flags a named, receiver-free function with one
// parameter and one bool result whose final return expression uses
// a comparison, &&, ||, or !. Such a function is a predicate. In
// files that import fp-go it must be composed from predicates.
// Functions of two or more parameters are not reported: they are
// bound with F.Bind1st at the call site.
func checkNoBoolExpr(
	fset *token.FileSet,
	f *ast.File,
	allow string,
) []Violation {
	if len(fpGoAliases(f)) == 0 {
		return nil
	}
	var violations []Violation
	reported := make(map[string]bool)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !isUnaryBoolFunc(fn) {
			continue
		}
		ret := finalReturn(fn)
		if ret == nil || !hasBoolOperator(ret) {
			continue
		}
		if exemptionFor(&violations, fset, f, fn.Name.Name,
			allow, reported) == exempt {
			continue
		}
		violations = append(violations, Violation{
			Position: fset.Position(fn.Pos()),
			Function: fn.Name.Name,
			Message:  boolExprMessage,
		})
	}
	return violations
}

// isUnaryBoolFunc reports whether fn is a plain function with one
// parameter and a single bool result.
func isUnaryBoolFunc(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Body == nil {
		return false
	}
	params, results := fn.Type.Params, fn.Type.Results
	if params == nil || results == nil ||
		len(results.List) != 1 {
		return false
	}
	count := 0
	for _, field := range params.List {
		count += max(len(field.Names), 1)
	}
	id, ok := results.List[0].Type.(*ast.Ident)
	return count == 1 && ok && id.Name == "bool" &&
		len(results.List[0].Names) <= 1
}

// finalReturn returns the expression of the last statement of fn
// when that statement is a single-value return.
func finalReturn(fn *ast.FuncDecl) ast.Expr {
	list := fn.Body.List
	if len(list) == 0 {
		return nil
	}
	ret, ok := list[len(list)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil
	}
	return ret.Results[0]
}

// hasBoolOperator reports whether expr, outside any nested call or
// function literal, uses a comparison, &&, ||, or unary !.
func hasBoolOperator(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr, *ast.FuncLit:
			return false
		case *ast.BinaryExpr:
			found = found || boolOperators[node.Op]
		case *ast.UnaryExpr:
			found = found || node.Op == token.NOT
		}
		return !found
	})
	return found
}

// CheckNoBoolExpr scans cfg.Roots and returns every hand-written
// unary predicate. cfg.AllowBoolExprDirective exempts a function
// (defaults to DefaultAllowBoolExprDirective).
func CheckNoBoolExpr(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	return scanFiles(cfg, func(
		fset *token.FileSet,
		f *ast.File,
	) []Violation {
		return checkNoBoolExpr(
			fset,
			f,
			cfg.AllowBoolExprDirective,
		)
	})
}

// RequireNoBoolExpr runs CheckNoBoolExpr and fails r on every
// violation or scan error.
func RequireNoBoolExpr(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoBoolExpr(cfg)
	reportAll(r, vs, err)
}
