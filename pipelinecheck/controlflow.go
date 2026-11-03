package pipelinecheck

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
)

// DefaultAllowLoopDirective exempts a function from the
// imperative-loop rule when followed by a non-empty reason.
const DefaultAllowLoopDirective = "fp-go:allow-loop"

// DefaultAllowBranchDirective exempts a function from the
// imperative-branch rule when followed by a non-empty reason.
const DefaultAllowBranchDirective = "fp-go:allow-branch"

const (
	fpGoPathPrefix = "github.com/IBM/fp-go/v2"
	loomPathPrefix = "github.com/dictyBase/fp-go-loom"
	tryCatchPrefix = "TryCatch"

	loopMessage = "imperative loop — use A.Map, A.Reduce, " +
		"A.FilterMap, or A.Traverse* from fp-go. Wrap a " +
		"mutable-API loop in one named adapter with " +
		"// " + DefaultAllowLoopDirective + " <reason>"

	branchMessage = "imperative branch outside a raw " +
		"effect — use O.FromPredicate + O.Fold, P.Fold, " +
		"E.FromPredicate, or matchopt. Wrap an adapter " +
		"for a mutable API in one named function with " +
		"// " + DefaultAllowBranchDirective + " <reason>"
)

// isLoopStmt reports whether n is a for or range statement.
func isLoopStmt(n ast.Node) bool {
	switch n.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return true
	}
	return false
}

// isBranchStmt reports whether n is an if, switch, or type switch.
func isBranchStmt(n ast.Node) bool {
	switch n.(type) {
	case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return true
	}
	return false
}

// isFpGoFamilyPath reports whether path is an fp-go v2 package or an
// fp-go-loom package. Loom combinators build fp-go values, so a
// call to one counts as an fp-go call.
func isFpGoFamilyPath(path string) bool {
	return strings.HasPrefix(path, fpGoPathPrefix) ||
		strings.HasPrefix(path, loomPathPrefix)
}

// fpGoAliases returns every import alias in f that resolves to an
// fp-go v2 or fp-go-loom package. A blank import contributes nothing.
func fpGoAliases(f *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if !isFpGoFamilyPath(path) {
			continue
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name != "_" {
			aliases[name] = true
		}
	}
	return aliases
}

// rawEffectCallbacks returns the function literals passed to an
// fp-go TryCatch* call. Such a callback is the imperative shell
// that wraps a raw effect, so control flow inside it is allowed.
func rawEffectCallbacks(
	f *ast.File,
	aliases map[string]bool,
) map[*ast.FuncLit]bool {
	callbacks := make(map[*ast.FuncLit]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel := pipeSelector(call.Fun)
		if sel == nil ||
			!strings.HasPrefix(sel.Sel.Name, tryCatchPrefix) ||
			!isFunctionAlias(sel.X, aliases) {
			return true
		}
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.FuncLit); ok {
				callbacks[lit] = true
			}
		}
		return true
	})
	return callbacks
}

// checkControlFlow reports every statement that match selects,
// outside fp-go TryCatch* callbacks and exempt functions.
func checkControlFlow(
	fset *token.FileSet,
	f *ast.File,
	match func(ast.Node) bool,
	message string,
	allow string,
) []Violation {
	var violations []Violation
	reported := make(map[string]bool)
	raw := rawEffectCallbacks(f, fpGoAliases(f))
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok && raw[lit] {
			return false
		}
		if n == nil || !match(n) {
			return true
		}
		fnName := enclosingFuncName(f, n.Pos())
		if exemptionFor(&violations, fset, f, fnName,
			allow, reported) == exempt {
			return true
		}
		violations = append(violations, Violation{
			Position: fset.Position(n.Pos()),
			Function: fnName,
			Message:  message,
		})
		return true
	})
	return violations
}

// scanFiles runs check over every non-test file under cfg.Roots.
func scanFiles(
	cfg Config,
	check func(*token.FileSet, *ast.File) []Violation,
) ([]Violation, error) {
	parsed, err := parseAll(cfg)
	if err != nil {
		return nil, err
	}
	var violations []Violation
	for _, p := range parsed {
		violations = append(violations, check(p.fset, p.f)...)
	}
	return violations, nil
}

// reportAll fails r on a scan error or on every violation.
func reportAll(r Reporter, vs []Violation, err error) {
	r.Helper()
	if err != nil {
		r.Fatalf("pipelinecheck: %v", err)
	}
	for _, v := range vs {
		r.Errorf("%s", v)
	}
}

// CheckNoImperativeLoop scans cfg.Roots and returns every for or
// range statement outside an fp-go TryCatch* callback. Use
// A.Map, A.Reduce, A.FilterMap, or A.Traverse* instead.
// cfg.AllowLoopDirective exempts a function (defaults to
// DefaultAllowLoopDirective). Use it for one named adapter around
// a mutable API, for example a tree builder that only offers
// AppendChild.
func CheckNoImperativeLoop(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	return scanFiles(cfg, func(
		fset *token.FileSet,
		f *ast.File,
	) []Violation {
		return checkControlFlow(fset, f, isLoopStmt,
			loopMessage, cfg.AllowLoopDirective)
	})
}

// RequireNoImperativeLoop runs CheckNoImperativeLoop and fails r on
// every violation or scan error.
func RequireNoImperativeLoop(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoImperativeLoop(cfg)
	reportAll(r, vs, err)
}

// CheckNoImperativeBranch scans cfg.Roots and returns every if,
// switch, or type switch statement outside an fp-go TryCatch*
// callback. Use O.FromPredicate + O.Fold, P.Fold, E.FromPredicate,
// or matchopt instead. cfg.AllowBranchDirective exempts a function
// (defaults to DefaultAllowBranchDirective). This rule is stricter
// than CheckBareIf, which flags only an if inside a pipeline
// callback.
func CheckNoImperativeBranch(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	return scanFiles(cfg, func(
		fset *token.FileSet,
		f *ast.File,
	) []Violation {
		return checkControlFlow(fset, f, isBranchStmt,
			branchMessage, cfg.AllowBranchDirective)
	})
}

// RequireNoImperativeBranch runs CheckNoImperativeBranch and fails
// r on every violation or scan error.
func RequireNoImperativeBranch(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoImperativeBranch(cfg)
	reportAll(r, vs, err)
}
