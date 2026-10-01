package pipelinecheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

// DefaultAllowTrivialDelegateDirective is the doc-comment marker that
// exempts a function from the trivial-delegate rule when followed by a
// non-empty reason.
const DefaultAllowTrivialDelegateDirective = "fp-go:allow-trivial-delegate"

// trivialDelegateMessage formats the failure message for a wrapper
// that only delegates to target.
func trivialDelegateMessage(
	name string,
	target string,
) string {
	return fmt.Sprintf(
		"wrapper %s only delegates to %s — call %s at the "+
			"pipeline call site",
		name,
		target,
		target,
	)
}

// delegateCandidate records a function whose entire body is a single
// pass-through call built from its own parameters.
type delegateCandidate struct {
	fn     *ast.FuncDecl
	target string
}

// CheckTrivialDelegate scans cfg.Roots and flags functions whose
// entire body is a single pass-through call on their own parameters
// (`func f(x State) IOE.X { return g(x) }`) when those functions are
// referenced from an F.Pipe/F.F.Flow chain or an IOE.* combinator
// argument in the same package. Unreferenced wrappers are out of
// scope; the unused linter owns dead code.
// cfg.AllowTrivialDelegateDirective exempts a function (defaults to
// DefaultAllowTrivialDelegateDirective).
func CheckTrivialDelegate(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	parsed, err := parseAll(cfg)
	if err != nil {
		return nil, err
	}
	return checkTrivialDelegate(parsed, cfg), nil
}

// RequireTrivialDelegate runs CheckTrivialDelegate and fails r on
// every violation or scan error.
func RequireTrivialDelegate(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckTrivialDelegate(cfg)
	if err != nil {
		r.Fatalf("pipelinecheck: %v", err)
	}
	for _, v := range vs {
		r.Errorf("%s", v)
	}
}

// checkTrivialDelegate groups parsed files by package and reports
// trivial delegate wrappers referenced from pipeline combinators.
func checkTrivialDelegate(
	parsed []parsedFile,
	cfg Config,
) []Violation {
	var violations []Violation
	for _, files := range groupByPackage(parsed) {
		violations = append(
			violations,
			packageDelegateViolations(files, cfg)...)
	}
	return violations
}

// groupByPackage maps a package identity to its parsed files.
func groupByPackage(
	parsed []parsedFile,
) map[string][]parsedFile {
	groups := make(map[string][]parsedFile)
	for _, p := range parsed {
		key := pkgKey(p.fset, p.f)
		groups[key] = append(groups[key], p)
	}
	return groups
}

// packageDelegateViolations scans one package: candidates first,
// then pipeline reference sites, then directive handling.
func packageDelegateViolations(
	files []parsedFile,
	cfg Config,
) []Violation {
	candidates := make(map[string]delegateCandidate)
	var violations []Violation
	for _, p := range files {
		collectDelegateCandidates(
			p.fset,
			p.f,
			cfg.AllowTrivialDelegateDirective,
			candidates,
			&violations,
		)
	}
	if len(candidates) == 0 {
		return violations
	}
	referenced := pipelineReferenceNames(files)
	reported := make(map[string]bool)
	for _, p := range files {
		reportPackageViolations(
			p.fset,
			p.f,
			candidates,
			referenced,
			reported,
			&violations,
		)
	}
	return violations
}

// collectDelegateCandidates records single-statement pass-through
// functions and reports directive-only-reason violations. Exempt
// wrappers are skipped entirely.
func collectDelegateCandidates(
	fset *token.FileSet,
	f *ast.File,
	directive string,
	candidates map[string]delegateCandidate,
	out *[]Violation,
) {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		status := exemptionStatus(fn, directive)
		if status == directiveWithoutReason {
			appendDirectiveErrors(
				out,
				fset,
				fn,
				status,
				directive,
			)
			continue
		}
		if status == exempt {
			continue
		}
		target, ok := delegateTarget(fn)
		if !ok {
			continue
		}
		candidates[fn.Name.Name] = delegateCandidate{
			fn:     fn,
			target: target,
		}
	}
}

// delegateTarget reports whether fn's body is exactly one return of a
// call whose arguments only forward the wrapper's own parameters, and
// returns the delegated target's textual form.
func delegateTarget(fn *ast.FuncDecl) (string, bool) {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return "", false
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return "", false
	}
	call, ok := ret.Results[0].(*ast.CallExpr)
	if !ok {
		return "", false
	}
	params := paramNames(fn)
	for _, arg := range call.Args {
		if !forwardsArg(arg, params) {
			return "", false
		}
	}
	// At least one argument (or the Fun receiver) must reference a
	// wrapper parameter, so `return g()` stays out of scope.
	if !forwardsParam(call.Fun, params) &&
		!anyForwardsParam(call.Args, params) {
		return "", false
	}
	return types.ExprString(call.Fun), true
}

// anyForwardsParam reports whether any expression references a
// parameter via forwardsParam.
func anyForwardsParam(
	exprs []ast.Expr,
	params map[string]bool,
) bool {
	for _, e := range exprs {
		if forwardsParam(e, params) {
			return true
		}
	}
	return false
}

// forwardsArg reports whether arg is a pass-through argument: a
// parameter-derived expression, a literal, or a plain identifier
// (a package constant or value). Closures and computed calls add
// work, so they disqualify the wrapper.

// paramNames returns fn's named parameter identifiers.
func paramNames(fn *ast.FuncDecl) map[string]bool {
	params := make(map[string]bool)
	if fn.Type.Params == nil {
		return params
	}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if name.Name != "" && name.Name != "_" {
				params[name.Name] = true
			}
		}
	}
	return params
}

// forwardsParam reports whether expr is the bare parameter identifier
// itself or a call/selector rooted at it, with no closure literal
// introducing extra work.
func forwardsParam(expr ast.Expr, params map[string]bool) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return params[e.Name]
	case *ast.ParenExpr:
		return forwardsParam(e.X, params)
	case *ast.SelectorExpr:
		return forwardsParam(e.X, params)
	case *ast.CallExpr:
		for _, inner := range append(
			[]ast.Expr{e.Fun},
			e.Args...,
		) {
			if forwardsParam(inner, params) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// pipelineReferenceNames collects identifiers referenced as arguments
// of F.PipeN/F.FlowN calls or any IOE.* combinator call across the
// package's files.
func pipelineReferenceNames(
	files []parsedFile,
) map[string]bool {
	names := make(map[string]bool)
	for _, p := range files {
		fnAliases := functionAliases(p.f)
		ioeAliases := ioeitherAliases(p.f)
		ast.Inspect(p.f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !isPipelineCombinator(
				call.Fun,
				fnAliases,
				ioeAliases,
			) {
				return true
			}
			for _, arg := range call.Args {
				collectReferenceName(arg, names)
			}
			return true
		})
	}
	return names
}

// isPipelineCombinator reports whether fun is an F.Pipe*/F.Flow*
// selector call or any selector call on the fp-go IOEither package.
func isPipelineCombinator(
	fun ast.Expr,
	fnAliases map[string]bool,
	ioeAliases map[string]bool,
) bool {
	sel := pipeSelector(fun)
	if sel == nil {
		return false
	}
	if isFunctionAlias(sel.X, fnAliases) &&
		isPipeOrFlow(sel.Sel.Name) {
		return true
	}
	return isFunctionAlias(sel.X, ioeAliases)
}

// collectReferenceName records the bare identifier or selector text
// of a named-continuation argument.
func collectReferenceName(arg ast.Expr, names map[string]bool) {
	switch e := arg.(type) {
	case *ast.Ident:
		names[e.Name] = true
	case *ast.SelectorExpr:
		names[types.ExprString(e)] = true
	case *ast.IndexExpr:
		names[types.ExprString(e)] = true
	case *ast.IndexListExpr:
		names[types.ExprString(e)] = true
	}
}

// reportPackageViolations flags each candidate whose name appears in
// a pipeline reference set, once per wrapper.
func reportPackageViolations(
	fset *token.FileSet,
	f *ast.File,
	candidates map[string]delegateCandidate,
	referenced map[string]bool,
	reported map[string]bool,
	out *[]Violation,
) {
	for name, cand := range candidates {
		if reported[name] || !referenced[name] {
			continue
		}
		if funcDeclFor(f, name) == nil {
			continue
		}
		reported[name] = true
		*out = append(*out, Violation{
			Position: fset.Position(cand.fn.Pos()),
			Function: name,
			Message: trivialDelegateMessage(
				name,
				cand.target,
			),
		})
	}
}

// forwardsArg reports whether expr is acceptable in a delegating
// call: a parameter-derived expression, a literal, or a plain
// identifier (a package constant or value). Composite expressions
// such as closures or computed calls add work and disqualify.
func forwardsArg(expr ast.Expr, params map[string]bool) bool {
	if forwardsParam(expr, params) {
		return true
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return true
	case *ast.UnaryExpr:
		return forwardsArg(e.X, params)
	default:
		return false
	}
}
