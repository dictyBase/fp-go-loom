package pipelinecheck

import (
	"go/ast"
	"go/token"
	"strings"
)

// DefaultAllowLocalPipeClosureDirective exempts a function from
// the local-pipe-closure rule when followed by a non-empty
// reason.
const DefaultAllowLocalPipeClosureDirective = "fp-go:allow-local-pipe-closure"

const localPipeClosureMessage = "local closure or combinator " +
	"assigned inside function — use a standalone function " +
	"(with F.Bind1st/F.Bind2nd), a package-level var, or " +
	"one flat F.PipeN"

// higherOrderCombinatorNames are fp-go combinators whose first
// argument is a function/predicate/thunk rather than plain data.
var higherOrderCombinatorNames = map[string]bool{
	bareIfMap:        true,
	"MapLeft":        true,
	"BiMap":          true,
	bareIfChain:      true,
	"ChainLeft":      true,
	"ChainEitherK":   true,
	"ChainIOK":       true,
	"ChainOptionK":   true,
	bareIfChainFirst: true,
	"ChainFirstIOK":  true,
	"ChainFirstLeft": true,
	"Filter":         true,
	"FilterMap":      true,
	"FromPredicate":  true,
	"FromOption":     true,
	"FromValidation": true,
	"TraverseArray":  true,
	"ContraMap":      true,
	"GetOrElse":      true,
	"OrElse":         true,
}

func isNamedPipeOrFlow(
	call *ast.CallExpr,
	fnAliases map[string]bool,
	prefix string,
) bool {
	sel := pipeSelector(call.Fun)
	if sel == nil || !isFunctionAlias(sel.X, fnAliases) {
		return false
	}
	if !strings.HasPrefix(sel.Sel.Name, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(sel.Sel.Name, prefix)
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func addIdentArgs(args []ast.Expr, dest map[string]bool) {
	for _, arg := range args {
		if id, ok := unparen(arg).(*ast.Ident); ok {
			dest[id.Name] = true
		}
	}
}

func isHigherOrderFpGoCall(
	call *ast.CallExpr,
	allFpAliases map[string]bool,
) bool {
	sel := pipeSelector(call.Fun)
	return sel != nil &&
		isFunctionAlias(sel.X, allFpAliases) &&
		higherOrderCombinatorNames[sel.Sel.Name]
}

// pipeTargetIdents collects identifiers used as (1) the seed of an
// F.Pipe* call or (2) a step of F.Pipe*/F.Flow* or the function
// argument of a higher-order fp-go combinator inside body.
func pipeTargetIdents(
	body *ast.BlockStmt,
	fnAliases map[string]bool,
	allFpAliases map[string]bool,
) (map[string]bool, map[string]bool) {
	seeds := make(map[string]bool)
	steps := make(map[string]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch {
		case isNamedPipeOrFlow(call, fnAliases, "Pipe"):
			if len(call.Args) > 0 {
				addIdentArgs(call.Args[:1], seeds)
				addIdentArgs(call.Args[1:], steps)
			}
		case isNamedPipeOrFlow(call, fnAliases, "Flow"):
			addIdentArgs(call.Args, steps)
		case isHigherOrderFpGoCall(call, allFpAliases):
			addIdentArgs(call.Args, steps)
		}
		return true
	})
	return seeds, steps
}

// isFlaggedLocalRHS reports whether assigning rhs to lhsName
// inside a function violates the local-pipe-closure rule.
func isFlaggedLocalRHS(
	lhsName string,
	rhs ast.Expr,
	fnAliases map[string]bool,
	allFpAliases map[string]bool,
	seeds map[string]bool,
	steps map[string]bool,
) bool {
	expr := unparen(rhs)
	if _, ok := expr.(*ast.FuncLit); ok {
		return true
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	if isNamedPipeOrFlow(call, fnAliases, "Flow") {
		return true
	}
	if isNamedPipeOrFlow(call, fnAliases, "Pipe") {
		return seeds[lhsName] || steps[lhsName]
	}
	if isFpGoCall(call, allFpAliases) {
		return steps[lhsName]
	}
	return false
}

func checkFuncLocalClosures(
	fset *token.FileSet,
	f *ast.File,
	fn *ast.FuncDecl,
	fnAliases map[string]bool,
	allFpAliases map[string]bool,
	raw map[*ast.FuncLit]bool,
	allow string,
	reported map[string]bool,
	violations *[]Violation,
) {
	seeds, steps := pipeTargetIdents(
		fn.Body,
		fnAliases,
		allFpAliases,
	)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok && raw[lit] {
			return false
		}
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			checkAssignPairs(
				fset,
				f,
				fn.Name.Name,
				stmt.Lhs,
				stmt.Rhs,
				fnAliases,
				allFpAliases,
				seeds,
				steps,
				allow,
				reported,
				violations,
			)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, len(stmt.Names))
			for i, name := range stmt.Names {
				lhs[i] = name
			}
			checkAssignPairs(
				fset,
				f,
				fn.Name.Name,
				lhs,
				stmt.Values,
				fnAliases,
				allFpAliases,
				seeds,
				steps,
				allow,
				reported,
				violations,
			)
		}
		return true
	})
}

func checkAssignPairs(
	fset *token.FileSet,
	f *ast.File,
	fnName string,
	lhs []ast.Expr,
	rhs []ast.Expr,
	fnAliases map[string]bool,
	allFpAliases map[string]bool,
	seeds map[string]bool,
	steps map[string]bool,
	allow string,
	reported map[string]bool,
	violations *[]Violation,
) {
	for i, r := range rhs {
		var name string
		if i < len(lhs) {
			if id, ok := lhs[i].(*ast.Ident); ok {
				name = id.Name
			}
		}
		if !isFlaggedLocalRHS(
			name,
			r,
			fnAliases,
			allFpAliases,
			seeds,
			steps,
		) {
			continue
		}
		if exemptionFor(
			violations,
			fset,
			f,
			fnName,
			allow,
			reported,
		) == exempt {
			return
		}
		*violations = append(*violations, Violation{
			Position: fset.Position(r.Pos()),
			Function: fnName,
			Message:  localPipeClosureMessage,
		})
	}
}

// checkNoLocalPipeClosure scans f for local closures, local
// F.Flow* pipelines, split F.Pipe* seeds, and local fp-go
// combinator assignments inside functions.
func checkNoLocalPipeClosure(
	fset *token.FileSet,
	f *ast.File,
	allow string,
) []Violation {
	allFpAliases := fpGoAliases(f)
	if len(allFpAliases) == 0 {
		return nil
	}
	fnAliases := functionAliases(f)
	raw := rawEffectCallbacks(f, allFpAliases)
	var violations []Violation
	reported := make(map[string]bool)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		checkFuncLocalClosures(
			fset,
			f,
			fn,
			fnAliases,
			allFpAliases,
			raw,
			allow,
			reported,
			&violations,
		)
	}
	return violations
}

// CheckNoLocalPipeClosure scans cfg.Roots and returns every local
// closure, local F.Flow*, split F.Pipe* seed, or local combinator
// assignment inside a function in files that import fp-go or
// fp-go-loom. cfg.AllowLocalPipeClosureDirective exempts a
// function (defaults to DefaultAllowLocalPipeClosureDirective).
func CheckNoLocalPipeClosure(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	return scanFiles(cfg, func(
		fset *token.FileSet,
		f *ast.File,
	) []Violation {
		return checkNoLocalPipeClosure(
			fset,
			f,
			cfg.AllowLocalPipeClosureDirective,
		)
	})
}

// RequireNoLocalPipeClosure runs CheckNoLocalPipeClosure and
// fails r on every violation or scan error.
func RequireNoLocalPipeClosure(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoLocalPipeClosure(cfg)
	reportAll(r, vs, err)
}
