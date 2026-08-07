package pipelinecheck

import (
	"go/ast"
	"go/token"
)

const handRolledFileJoinMessage = "hand-rolled path mapping inside fp-go " +
	"array.Map — use FILE.Join with one flipped root mapper: " +
	"root := F.Pipe1(base, F.Flip(FILE.Join)); A.Map(root)"

// checkNoHandRolledFileJoin flags filepath path construction in a
// function literal passed to fp-go array.Map. Import aliases are
// resolved by canonical package path.
func checkNoHandRolledFileJoin(
	fset *token.FileSet,
	f *ast.File,
	allow string,
) []Violation {
	arrayAliases := importAliasesFor(f, ArrayPkgPath)
	filepathAliases := importAliasesFor(f, FilepathPkgPath)
	if len(arrayAliases) == 0 || len(filepathAliases) == 0 {
		return nil
	}
	var violations []Violation
	reported := make(map[string]bool)
	reportedCalls := make(map[token.Pos]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || reportedCalls[call.Pos()] {
			return true
		}
		if violation, ok := fileJoinViolation(
			fset,
			f,
			call,
			arrayAliases,
			filepathAliases,
			allow,
			&violations,
			reported,
		); ok {
			reportedCalls[call.Pos()] = true
			violations = append(violations, violation)
		}
		return true
	})
	return violations
}

func fileJoinViolation(
	fset *token.FileSet,
	f *ast.File,
	call *ast.CallExpr,
	arrayAliases map[string]bool,
	filepathAliases map[string]bool,
	allow string,
	violations *[]Violation,
	reported map[string]bool,
) (Violation, bool) {
	if !isArrayMapMapper(call, arrayAliases) {
		return Violation{}, false
	}
	mapper := call.Args[0].(*ast.FuncLit)
	if !containsHandRolledJoin(mapper.Body, filepathAliases) {
		return Violation{}, false
	}
	fnName := enclosingFuncName(f, call.Pos())
	if exemptionFor(
		violations,
		fset,
		f,
		fnName,
		allow,
		reported,
	) == exempt {
		return Violation{}, false
	}
	return Violation{
		Position: fset.Position(call.Pos()),
		Function: fnName,
		Message:  handRolledFileJoinMessage,
	}, true
}

func isArrayMapMapper(
	call *ast.CallExpr,
	aliases map[string]bool,
) bool {
	sel := pipeSelector(call.Fun)
	if sel == nil || sel.Sel.Name != "Map" ||
		!aliases[selectorAlias(sel.X)] || len(call.Args) == 0 {
		return false
	}
	_, ok := call.Args[0].(*ast.FuncLit)
	return ok
}

func containsHandRolledJoin(
	body *ast.BlockStmt,
	filepathAliases map[string]bool,
) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel := pipeSelector(call.Fun)
		if sel == nil || !filepathAliases[selectorAlias(sel.X)] {
			return true
		}
		if sel.Sel.Name == "Join" ||
			sel.Sel.Name == "FromSlash" {
			found = true
			return false
		}
		return true
	})
	return found
}
