package pipelinecheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
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
// pass-through call or single-use Pipe wrapper built from its own
// parameters.
type delegateCandidate struct {
	fn        *ast.FuncDecl
	target    string
	singleUse bool
}

// CheckTrivialDelegate scans cfg.Roots and flags functions
// referenced from an F.Pipe/F.Flow chain, an IOE.* combinator
// argument, or lens.Modify in the same package when they (1) return
// a single pass-through call on their own parameters, (2) take only
// ignored parameters (`_ State`) and return a single non-closure
// call, or (3) are unexported single-use F.PipeN wrappers with no
// TryCatch* or MapLeft stage. Unreferenced wrappers are out of
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
	referenced, linear := pipelineReferenceSets(files)
	counts := packageIdentCounts(files)
	reported := make(map[string]bool)
	for _, p := range files {
		reportPackageViolations(
			p.fset,
			p.f,
			candidates,
			referenced,
			linear,
			counts,
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
	fnAliases := functionAliases(f)
	ioeAliases := ioeitherAliases(f)
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
		cand, ok := delegateCandidateFor(
			fn,
			fnAliases,
			ioeAliases,
		)
		if !ok {
			continue
		}
		candidates[fn.Name.Name] = cand
	}
}

// delegateCandidateFor inspects fn and returns a delegateCandidate
// when fn is a pass-through call wrapper, an ignored-parameter
// pipeline adapter, or a single-use Pipe wrapper.
func delegateCandidateFor(
	fn *ast.FuncDecl,
	fnAliases map[string]bool,
	ioeAliases map[string]bool,
) (delegateCandidate, bool) {
	if target, ok := delegateTarget(fn, fnAliases); ok {
		return delegateCandidate{
			fn:     fn,
			target: target,
		}, true
	}
	if target, ok := ignoredParamTarget(fn); ok {
		return delegateCandidate{
			fn:     fn,
			target: target,
		}, true
	}
	if target, ok := singleUsePipeTarget(
		fn,
		fnAliases,
		ioeAliases,
	); ok {
		return delegateCandidate{
			fn:        fn,
			target:    target,
			singleUse: true,
		}, true
	}
	return delegateCandidate{}, false
}

// delegateTarget reports whether fn's body is exactly one return of a
// non-Pipe call whose arguments only forward the wrapper's own
// parameters, and returns the delegated target's textual form.
func delegateTarget(
	fn *ast.FuncDecl,
	fnAliases map[string]bool,
) (string, bool) {
	call, ok := singleReturnCall(fn)
	if !ok || isPipeOrFlowCall(call, fnAliases) {
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

// ignoredParamTarget flags an unexported package-level function that
// takes only ignored parameters (`_ State`) and returns a single call
// containing no function literals (e.g. `return NewClient(...)`
// instead of `F.Constant1[State](NewClient(...))`).
func ignoredParamTarget(fn *ast.FuncDecl) (string, bool) {
	if fn.Recv != nil || ast.IsExported(fn.Name.Name) {
		return "", false
	}
	if !hasOnlyIgnoredParams(fn) {
		return "", false
	}
	call, ok := singleReturnCall(fn)
	if !ok || containsFuncLit(call) {
		return "", false
	}
	return types.ExprString(call.Fun), true
}

// singleUsePipeTarget flags an unexported package-level function
// whose entire body is `return F.PipeN(seed, ...)` where seed
// forwards the function's parameter and no step is a TryCatch* or
// MapLeft effect/error-wrapping stage.
func singleUsePipeTarget(
	fn *ast.FuncDecl,
	fnAliases map[string]bool,
	ioeAliases map[string]bool,
) (string, bool) {
	if fn.Recv != nil || ast.IsExported(fn.Name.Name) {
		return "", false
	}
	call, ok := singleReturnCall(fn)
	if !ok || !isPipeOrFlowCall(call, fnAliases) {
		return "", false
	}
	if len(call.Args) < 2 {
		return "", false
	}
	params := paramNames(fn)
	if !forwardsParam(call.Args[0], params) {
		return "", false
	}
	if hasEffectOrErrorStep(call.Args[1:], ioeAliases) {
		return "", false
	}
	return types.ExprString(call.Fun), true
}

// singleReturnCall extracts the *ast.CallExpr from a function whose
// body is a single `return call(...)` statement.
func singleReturnCall(fn *ast.FuncDecl) (*ast.CallExpr, bool) {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return nil, false
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil, false
	}
	call, ok := ret.Results[0].(*ast.CallExpr)
	return call, ok
}

// isPipeOrFlowCall reports whether call is an F.Pipe* or F.Flow*
// invocation.
func isPipeOrFlowCall(
	call *ast.CallExpr,
	fnAliases map[string]bool,
) bool {
	sel := pipeSelector(call.Fun)
	if sel == nil {
		return false
	}
	return isFunctionAlias(sel.X, fnAliases) &&
		isPipeOrFlow(sel.Sel.Name)
}

// hasOnlyIgnoredParams reports whether fn declares at least one
// parameter and every parameter is unnamed or `_`.
func hasOnlyIgnoredParams(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return false
	}
	return len(paramNames(fn)) == 0
}

// containsFuncLit reports whether expr contains any function literal.
func containsFuncLit(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			found = true
			return false
		}
		return true
	})
	return found
}

// hasEffectOrErrorStep reports whether any pipe step is a TryCatch*
// or MapLeft combinator call, which legitimately encapsulates a raw
// effect or contextual error wrap.
func hasEffectOrErrorStep(
	steps []ast.Expr,
	ioeAliases map[string]bool,
) bool {
	for _, step := range steps {
		call, ok := step.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel := pipeSelector(call.Fun)
		if sel == nil || !isFunctionAlias(sel.X, ioeAliases) {
			continue
		}
		name := sel.Sel.Name
		if name == "MapLeft" ||
			strings.HasPrefix(name, "TryCatch") {
			return true
		}
	}
	return false
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

// pipelineReferenceSets collects (1) all identifiers referenced as
// arguments of F.PipeN/F.FlowN calls or IOE.* combinators, and (2)
// linear step references (F.PipeN/F.FlowN steps, IOE.Chain*,
// IOE.Map*, and lens.Modify arguments inside pipes) where a
// single-use F.PipeN wrapper can be inlined directly into the outer
// pipe.
func pipelineReferenceSets(
	files []parsedFile,
) (map[string]bool, map[string]bool) {
	all := make(map[string]bool)
	linear := make(map[string]bool)
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
			isLinear := isLinearCombinator(
				call.Fun,
				fnAliases,
				ioeAliases,
			)
			for _, arg := range call.Args {
				collectReferenceName(arg, all)
				if isLinear {
					collectLinearReferenceName(arg, linear)
				}
			}
			return true
		})
	}
	return all, linear
}

// isLinearCombinator reports whether fun is F.Pipe*, F.Flow*, or an
// IOE.Chain*/IOE.Map* combinator whose argument can be spliced into
// the enclosing pipe.
func isLinearCombinator(
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
	if !isFunctionAlias(sel.X, ioeAliases) {
		return false
	}
	switch sel.Sel.Name {
	case "Chain", "ChainEitherK", "ChainFirst", "Map":
		return true
	default:
		return false
	}
}

// collectLinearReferenceName records direct step identifiers and
// lens.Modify(fn) arguments inside a linear pipeline.
func collectLinearReferenceName(
	arg ast.Expr,
	names map[string]bool,
) {
	collectReferenceName(arg, names)
	call, ok := arg.(*ast.CallExpr)
	if !ok {
		return
	}
	sel := pipeSelector(call.Fun)
	if sel != nil && sel.Sel.Name == "Modify" {
		for _, mArg := range call.Args {
			collectReferenceName(mArg, names)
		}
	}
}

// packageIdentCounts counts non-declaration identifier uses across
// all parsed files of a package.
func packageIdentCounts(files []parsedFile) map[string]int {
	counts := make(map[string]int)
	for _, p := range files {
		declNames := make(map[*ast.Ident]bool)
		for _, decl := range p.f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				declNames[fn.Name] = true
			}
		}
		ast.Inspect(p.f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if ok && !declNames[ident] {
				counts[ident.Name]++
			}
			return true
		})
	}
	return counts
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
	case *ast.CallExpr:
		sel := pipeSelector(e.Fun)
		if sel != nil && sel.Sel.Name == "Modify" {
			for _, mArg := range e.Args {
				collectReferenceName(mArg, names)
			}
		}
	}
}

// reportPackageViolations flags each candidate whose name appears in
// a pipeline reference set, once per wrapper.
func reportPackageViolations(
	fset *token.FileSet,
	f *ast.File,
	candidates map[string]delegateCandidate,
	referenced map[string]bool,
	linear map[string]bool,
	counts map[string]int,
	reported map[string]bool,
	out *[]Violation,
) {
	for name, cand := range candidates {
		if reported[name] || !referenced[name] {
			continue
		}
		if cand.singleUse &&
			(!linear[name] || counts[name] != 1) {
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
