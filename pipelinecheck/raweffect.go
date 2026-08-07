package pipelinecheck

import (
	"go/ast"
	"go/token"
	"strings"
)

const nonRawTryCatchMessage = "non-raw work inside IOE.TryCatchError callback — " +
	"keep raw operation in TryCatchError; move wrapping to IOE.MapLeft " +
	"and success projection to IOE.Map"

// checkNoNonRawTryCatchCallback flags clear error wrapping, lens
// setter calls, and success projections inside TryCatchError callback
// literals. It intentionally does not flag arbitrary nested calls.
func checkNoNonRawTryCatchCallback(
	fset *token.FileSet,
	f *ast.File,
	aliases map[string]bool,
	allow string,
) []Violation {
	fmtAliases := importAliasesFor(f, FmtPkgPath)
	var violations []Violation
	reported := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel := pipeSelector(call.Fun)
		if sel == nil || sel.Sel.Name != "TryCatchError" ||
			!isFunctionAlias(sel.X, aliases) {
			return true
		}
		callback := tryCatchCallback(call)
		if callback == nil {
			return true
		}
		fnName := enclosingFuncName(f, call.Pos())
		if exemptionFor(
			&violations,
			fset,
			f,
			fnName,
			allow,
			reported,
		) == exempt {
			return true
		}
		if nonRawNode := findNonRawNode(
			callback.Body,
			fmtAliases,
		); nonRawNode != nil {
			violations = append(violations, Violation{
				Position: fset.Position(nonRawNode.Pos()),
				Function: fnName,
				Message:  nonRawTryCatchMessage,
			})
		}
		return true
	})
	return violations
}

func findNonRawNode(
	body *ast.BlockStmt,
	fmtAliases map[string]bool,
) ast.Node {
	var found ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil || found != nil {
			return found == nil
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if isFmtErrorCall(n, fmtAliases) || isLensSetCall(n) {
			found = n
			return false
		}
		if isSuccessProjection(n) {
			found = n
			return false
		}
		return true
	})
	return found
}

func isFmtErrorCall(
	n ast.Node,
	fmtAliases map[string]bool,
) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel := pipeSelector(call.Fun)
	return sel != nil && sel.Sel.Name == "Errorf" &&
		fmtAliases[selectorAlias(sel.X)]
}

func isLensSetCall(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Set" {
		return false
	}
	return looksLikeLens(sel.X)
}

func looksLikeLens(expr ast.Expr) bool {
	switch value := expr.(type) {
	case *ast.Ident:
		name := strings.ToLower(value.Name)
		return strings.Contains(name, "lens") ||
			strings.HasSuffix(name, "optic")
	case *ast.SelectorExpr:
		return looksLikeLens(value.X) ||
			looksLikeLens(value.Sel)
	default:
		return false
	}
}

func isSuccessProjection(n ast.Node) bool {
	ret, ok := n.(*ast.ReturnStmt)
	if !ok || len(ret.Results) < 2 {
		return false
	}
	switch ret.Results[0].(type) {
	case *ast.SelectorExpr,
		*ast.IndexExpr,
		*ast.SliceExpr,
		*ast.TypeAssertExpr,
		*ast.CallExpr:
		return true
	default:
		return false
	}
}
