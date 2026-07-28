// Package pipelinecheck enforces fp-go pipeline continuity for CLI
// entrypoints. A function matching the entrypoint predicate (run* by
// default) must expose a complete F.PipeN expression; a single-step
// F.Pipe1(seed, namedFn) handoff to a single-use wrapper is flagged.
//
// Opt a function out with a doc comment of the form:
//
//	// fp-go:allow-pipe1-handoff <reason>
//
// The directive must be followed by a non-empty reason. Use it only when
// the wrapper is genuinely reused (2+ call sites) or is an independently
// meaningful domain operation — not for a single-use force/fold
// continuation.
//
// Limitations: analysis is syntax-only (no go/types), so a shadowed alias
// of the function package may false-positive and a dot import
// (import . ".../function") false-negatives. Assign-then-return shapes
// (p := F.Pipe1(...); return p) are not inspected; only the returned
// expression is checked. See the fp-go-pipe-flow skill Anti-patterns
// section.
//
// This package walks Go ASTs imperatively. ast.Inspect's callback model
// does not fit fp-go pipe composition, so the walk layers are imperative
// by design; the public API remains in the house style.
package pipelinecheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode"
)

// FunctionPkgPath is the canonical fp-go function-package import path.
const FunctionPkgPath = "github.com/IBM/fp-go/v2/function"

// DefaultAllowDirective is the doc-comment marker that exempts a
// function when followed by a non-empty reason.
const DefaultAllowDirective = "fp-go:allow-pipe1-handoff"

// Reporter is the minimal subset of testing.TB that Require needs.
// *testing.T and *testing.B satisfy it implicitly; tests may pass a
// custom stub to assert failure paths.
type Reporter interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Config configures a Check run.
type Config struct {
	// Roots are directories scanned recursively for non-test .go
	// files. Defaults to []string{"."} when empty. Duplicate paths are
	// deduplicated.
	Roots []string

	// IsEntrypoint reports whether a function name is a CLI entrypoint
	// worth checking. Defaults to "has prefix run" when nil.
	IsEntrypoint func(name string) bool

	// AllowDirective is the doc-comment marker that exempts a function.
	// The directive must be followed by a non-empty reason. Defaults to
	// DefaultAllowDirective when empty.
	AllowDirective string
}

// Violation is a single pipeline-continuity failure.
type Violation struct {
	Position token.Position
	Function string
	Message  string
}

// String formats a violation as "position: function: message".
func (v Violation) String() string {
	return fmt.Sprintf(
		"%s: %s: %s",
		v.Position,
		v.Function,
		v.Message,
	)
}

// Check scans cfg.Roots and returns every F.Pipe1(seed, namedFn) handoff
// in an entrypoint that lacks a valid exemption.
func Check(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	seen := make(map[string]bool)
	var violations []Violation
	for _, root := range dedupeRoots(cfg.Roots) {
		files, err := nonTestGoFiles(root)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if seen[file] {
				continue
			}
			seen[file] = true
			vs, err := checkFileOnDisk(cfg, file)
			if err != nil {
				return nil, err
			}
			violations = append(violations, vs...)
		}
	}
	return violations, nil
}

// Require runs Check and fails r on every violation or scan error.
func Require(r Reporter, cfg Config) {
	r.Helper()
	violations, err := Check(cfg)
	if err != nil {
		r.Fatalf("pipelinecheck: %v", err)
	}
	for _, v := range violations {
		r.Errorf("%s", v)
	}
}

// withDefaults fills zero Config fields with the package defaults.
func withDefaults(cfg Config) Config {
	if len(cfg.Roots) == 0 {
		cfg.Roots = []string{"."}
	}
	if cfg.IsEntrypoint == nil {
		cfg.IsEntrypoint = func(name string) bool {
			return strings.HasPrefix(name, "run")
		}
	}
	if cfg.AllowDirective == "" {
		cfg.AllowDirective = DefaultAllowDirective
	}
	return cfg
}

func dedupeRoots(roots []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		clean := filepath.Clean(r)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out
}

func nonTestGoFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(
		path string,
		d fs.DirEntry,
		err error,
	) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if isSkippedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

// isSkippedDir reports whether to prune a directory during the walk.
// Dot-dirs (.git, .github), vendor, testdata, and _-prefixed dirs are
// pruned; the walk root "." is not.
func isSkippedDir(name string) bool {
	if name == "." {
		return false
	}
	if strings.HasPrefix(name, ".") || name == "vendor" {
		return true
	}
	return name == "testdata" || strings.HasPrefix(name, "_")
}

func checkFileOnDisk(
	cfg Config,
	file string,
) ([]Violation, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(
		fset,
		file,
		nil,
		parser.ParseComments,
	)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	return checkFile(fset, f, cfg), nil
}

// checkFile inspects a parsed file and returns violations. Tests call
// it directly with in-memory parse results.
func checkFile(
	fset *token.FileSet,
	f *ast.File,
	cfg Config,
) []Violation {
	aliases := functionAliases(f)
	if len(aliases) == 0 {
		return nil
	}
	var violations []Violation
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil ||
			!cfg.IsEntrypoint(fn.Name.Name) {
			return true
		}
		appendFuncViolations(&violations, fset, fn, aliases, cfg)
		return true
	})
	return violations
}

func appendFuncViolations(
	out *[]Violation,
	fset *token.FileSet,
	fn *ast.FuncDecl,
	aliases map[string]bool,
	cfg Config,
) {
	switch exemptionStatus(fn, cfg.AllowDirective) {
	case exempt:
		return
	case directiveWithoutReason:
		*out = append(*out, Violation{
			Position: fset.Position(fn.Pos()),
			Function: fn.Name.Name,
			Message: cfg.AllowDirective +
				" directive requires a non-empty reason",
		})
		return
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, expr := range ret.Results {
			call, ok := expr.(*ast.CallExpr)
			if !ok || !isPipe1Handoff(call, aliases) {
				continue
			}
			*out = append(*out, Violation{
				Position: fset.Position(call.Pos()),
				Function: fn.Name.Name,
				Message: "returns F.Pipe1(seed, namedFn) — " +
					"inline the wrapper's steps into the " +
					"entrypoint F.PipeN " +
					"(fp-go-pipe-flow Anti-patterns)",
			})
		}
		return true
	})
}

type exemption int

const (
	notExempt exemption = iota
	exempt
	directiveWithoutReason
)

// exemptionStatus inspects fn's doc comment for the allow directive.
// A directive followed by a word boundary and a non-empty reason
// exempts the function. A directive with no reason is a separate
// violation so the author knows why the exemption did not apply. A
// typo'd directive (e.g. allow-pipe1-handoffx) is not matched.
func exemptionStatus(
	fn *ast.FuncDecl,
	directive string,
) exemption {
	if fn.Doc == nil {
		return notExempt
	}
	for _, c := range fn.Doc.List {
		text := stripComment(c.Text)
		if !strings.HasPrefix(text, directive) {
			continue
		}
		rest := strings.TrimPrefix(text, directive)
		switch {
		case rest == "":
			return directiveWithoutReason
		case !unicode.IsSpace(rune(rest[0])):
			// No word boundary: a typo like "allow-pipe1-handoffx".
			return notExempt
		}
		reason := strings.TrimSpace(rest)
		if reason == "" {
			return directiveWithoutReason
		}
		return exempt
	}
	return notExempt
}

func stripComment(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "//"):
		s = strings.TrimPrefix(s, "//")
	case strings.HasPrefix(s, "/*"):
		s = strings.TrimPrefix(s, "/*")
		s = strings.TrimSuffix(s, "*/")
	}
	return strings.TrimSpace(s)
}

// functionAliases returns the set of import aliases that resolve to the
// fp-go function package in this file. A file may import it under a
// custom alias or the default package name "function". A blank import
// (_) contributes nothing.
func functionAliases(f *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path != FunctionPkgPath {
			continue
		}
		if imp.Name == nil {
			aliases["function"] = true
			continue
		}
		if imp.Name.Name == "_" {
			continue
		}
		aliases[imp.Name.Name] = true
	}
	return aliases
}

func isPipe1Handoff(
	call *ast.CallExpr,
	aliases map[string]bool,
) bool {
	sel := pipe1Selector(call.Fun)
	if sel == nil || sel.Sel.Name != "Pipe1" {
		return false
	}
	if !isFunctionAlias(sel.X, aliases) {
		return false
	}
	if len(call.Args) != 2 {
		return false
	}
	return isNamedContinuation(call.Args[1])
}

// pipe1Selector unwraps generic instantiation
// (F.Pipe1[int, error](...)) to reach the underlying SelectorExpr.
func pipe1Selector(fun ast.Expr) *ast.SelectorExpr {
	switch e := fun.(type) {
	case *ast.SelectorExpr:
		return e
	case *ast.IndexExpr:
		if sel, ok := e.X.(*ast.SelectorExpr); ok {
			return sel
		}
	case *ast.IndexListExpr:
		if sel, ok := e.X.(*ast.SelectorExpr); ok {
			return sel
		}
	}
	return nil
}

func isFunctionAlias(x ast.Expr, aliases map[string]bool) bool {
	ident, ok := x.(*ast.Ident)
	return ok && aliases[ident.Name]
}

// isNamedContinuation reports whether expr is a named function reference
// (identifier, selector, or generic instantiation) rather than an
// inline func literal or a combinator call. Parenthesized expressions
// are unwrapped first.
func isNamedContinuation(expr ast.Expr) bool {
	if p, ok := expr.(*ast.ParenExpr); ok {
		expr = p.X
	}
	switch expr.(type) {
	case *ast.Ident,
		*ast.SelectorExpr,
		*ast.IndexExpr,
		*ast.IndexListExpr:
		return true
	}
	return false
}
