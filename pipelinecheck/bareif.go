package pipelinecheck

import (
	"go/ast"
	"go/token"
	"strings"
)

const bareIfMessage = "bare if inside fp-go pipeline closure — " +
	"extract branches to a pre-bound P.Fold(onFalse, onTrue) " +
	"and apply the predicate at the pipeline call site " +
	"(fp-go-pipe-flow Point-Free Branching)"

const (
	bareIfMap        = "Map"
	bareIfChain      = "Chain"
	bareIfChainFirst = "ChainFirst"
)

// fp-go combinator packages whose callbacks are expected to stay
// point-free. Package paths are resolved from imports, not aliases.
var bareIfCombinatorPkgs = map[string]map[string]bool{
	IOEitherPkgPath: {
		bareIfMap:        true,
		bareIfChain:      true,
		bareIfChainFirst: true,
	},
	EitherPkgPath: {
		bareIfMap:        true,
		bareIfChain:      true,
		bareIfChainFirst: true,
	},
	OptionPkgPath: {
		bareIfMap:        true,
		bareIfChain:      true,
		bareIfChainFirst: true,
	},
	IOPkgPath: {
		bareIfMap:        true,
		bareIfChain:      true,
		bareIfChainFirst: true,
	},
}

// checkBareIf flags if statements in function literals passed directly
// to fp-go Pipe/Flow or selected transformation combinators. Ordinary
// closures and raw TryCatchError callbacks are deliberately ignored.
func checkBareIf(
	fset *token.FileSet,
	f *ast.File,
	allow string,
) []Violation {
	aliases := make(map[string]map[string]bool)
	for path := range bareIfCombinatorPkgs {
		aliases[path] = importAliasesFor(f, path)
	}
	functionAliasesForBareIf := functionAliases(f)
	var violations []Violation
	reported := make(map[string]bool)
	reportedIfs := make(map[token.Pos]bool)

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if !isBareIfCombinator(
			call.Fun,
			functionAliasesForBareIf,
			aliases,
		) {
			return true
		}
		for _, arg := range call.Args {
			fn, ok := arg.(*ast.FuncLit)
			if !ok {
				continue
			}
			inspectBareIfBody(
				fset,
				f,
				fn.Body,
				allow,
				&violations,
				reported,
				reportedIfs,
			)
		}
		return true
	})
	return violations
}

func isBareIfCombinator(
	fun ast.Expr,
	functionAliasesForBareIf map[string]bool,
	aliases map[string]map[string]bool,
) bool {
	sel := pipeSelector(fun)
	if sel == nil {
		return false
	}
	if functionAliasesForBareIf[selectorAlias(sel.X)] {
		return isPipeOrFlow(sel.Sel.Name)
	}
	for path, names := range bareIfCombinatorPkgs {
		if aliases[path][selectorAlias(sel.X)] &&
			names[sel.Sel.Name] {
			return true
		}
	}
	return false
}

func isPipeOrFlow(name string) bool {
	for _, prefix := range []string{"Pipe", "Flow"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, prefix)
		if suffix == "" {
			continue
		}
		for _, r := range suffix {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	return false
}

func selectorAlias(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

func inspectBareIfBody(
	fset *token.FileSet,
	f *ast.File,
	body *ast.BlockStmt,
	allow string,
	violations *[]Violation,
	reported map[string]bool,
	reportedIfs map[token.Pos]bool,
) {
	if body == nil {
		return
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		ifs, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		fnName := enclosingFuncName(f, ifs.Pos())
		if reportedIfs[ifs.Pos()] {
			return true
		}
		reportedIfs[ifs.Pos()] = true
		position := fset.Position(ifs.Pos())
		if exemptionFor(
			violations,
			fset,
			f,
			fnName,
			allow,
			reported,
		) == exempt {
			return true
		}
		*violations = append(*violations, Violation{
			Position: position,
			Function: fnName,
			Message:  bareIfMessage,
		})
		return true
	})
}
