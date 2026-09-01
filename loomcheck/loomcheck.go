// Package loomcheck enforces "import the loom combinator instead of
// hand-rolling it" through a syntax-only AST check.
//
// The failure mode it targets is subtle: an author loads the fp-go
// style skills, writes a small `func(x T) bool { return ... }` inline,
// and ships code that looks perfectly idiomatic while silently
// reinventing a combinator this module already exports. Nothing in
// the compiler, the formatter, or golangci-lint objects, so the
// duplication survives review.
//
// The rule flags a function literal that
//
//  1. takes one parameter and returns a single bool, and
//  2. has a body of exactly one return statement whose expression
//     matches a shape loom already covers.
//
// Condition 2 keeps the signal high: by default only recognized
// shapes are reported, each with the exact loom replacement named in
// the message. Set Config.StrictAllPredicates to also flag
// single-return bool literals whose shape is not recognized, which
// surfaces candidates for new loom combinators.
//
// Opt out per function with a doc comment carrying the directive and
// a non-empty reason:
//
//	// fp-go:allow-hand-rolled-predicate SDK fixes this callback shape
//	func adapter() { ... }
//
// Usage mirrors pipelinecheck — drop a thin test into any package:
//
//	func TestNoHandRolledPredicates(t *testing.T) {
//		loomcheck.Require(t, loomcheck.Config{Roots: []string{".."}})
//	}
package loomcheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// DefaultAllowDirective exempts a function from the hand-rolled
// predicate rule. It must be followed by a non-empty reason.
const DefaultAllowDirective = "fp-go:allow-hand-rolled-predicate"

// defaultSkipDirs are never scanned.
var defaultSkipDirs = []string{
	".git",
	"testdata",
	"vendor",
	"node_modules",
}

// Reporter is the subset of *testing.T that Require needs.
type Reporter interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Config configures a Check run.
type Config struct {
	// Roots are directories scanned recursively for non-test .go
	// files. Defaults to []string{"."} when empty.
	Roots []string

	// AllowDirective exempts a function from the rule. Defaults to
	// DefaultAllowDirective when empty.
	AllowDirective string

	// SkipDirs are directory base names skipped during the walk, in
	// addition to .git, testdata, vendor and node_modules.
	SkipDirs []string

	// StrictAllPredicates also reports single-return bool literals
	// whose shape has no known loom replacement. Off by default.
	StrictAllPredicates bool
}

// Violation is a single rule failure.
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

// Check scans cfg.Roots and returns every hand-rolled predicate that
// lacks a valid exemption.
func Check(cfg Config) ([]Violation, error) {
	roots := cfg.Roots
	if len(roots) == 0 {
		roots = []string{"."}
	}
	allow := cfg.AllowDirective
	if allow == "" {
		allow = DefaultAllowDirective
	}
	files, err := collectFiles(roots, cfg.SkipDirs)
	if err != nil {
		return nil, err
	}
	var out []Violation
	for _, path := range files {
		vs, err := checkFile(
			path,
			allow,
			cfg.StrictAllPredicates,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, vs...)
	}
	return out, nil
}

// Require fails r when Check reports any violation.
func Require(r Reporter, cfg Config) {
	r.Helper()
	violations, err := Check(cfg)
	if err != nil {
		r.Fatalf("loomcheck: %v", err)
		return
	}
	for _, v := range violations {
		r.Errorf("%s", v.String())
	}
}

// checkFile parses one file and runs the rule over it.
func checkFile(
	path string,
	allow string,
	strict bool,
) ([]Violation, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(
		fset,
		path,
		nil,
		parser.ParseComments,
	)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if isGenerated(f) {
		return nil, nil
	}
	return checkHandRolledPredicates(fset, f, allow, strict), nil
}

// collectFiles walks roots and returns non-test .go files.
func collectFiles(
	roots []string,
	skip []string,
) ([]string, error) {
	skipSet := make(map[string]bool)
	for _, d := range defaultSkipDirs {
		skipSet[d] = true
	}
	for _, d := range skip {
		skipSet[d] = true
	}
	seen := make(map[string]bool)
	var out []string
	for _, root := range roots {
		err := filepath.Walk(
			root,
			walkFn(skipSet, seen, &out),
		)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// walkFn builds the filepath.Walk callback for collectFiles.
func walkFn(
	skipSet map[string]bool,
	seen map[string]bool,
	out *[]string,
) filepath.WalkFunc {
	return func(
		path string,
		info os.FileInfo,
		err error,
	) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipSet[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !isTargetFile(info.Name()) {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if seen[abs] {
			return nil
		}
		seen[abs] = true
		*out = append(*out, path)
		return nil
	}
}

// isTargetFile reports whether name is a non-test Go source file.
func isTargetFile(name string) bool {
	if !strings.HasSuffix(name, ".go") {
		return false
	}
	return !strings.HasSuffix(name, "_test.go")
}

// isGenerated reports whether f carries a generated-code marker.
func isGenerated(f *ast.File) bool {
	for _, group := range f.Comments {
		for _, c := range group.List {
			if strings.Contains(c.Text, "Code generated") &&
				strings.Contains(c.Text, "DO NOT EDIT") {
				return true
			}
		}
	}
	return false
}

// enclosingFuncName returns the name of the FuncDecl containing pos,
// or "<file>" when the node sits at package level.
func enclosingFuncName(f *ast.File, pos token.Pos) string {
	name := "<file>"
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		if pos >= fn.Pos() && pos <= fn.End() {
			name = fn.Name.Name
		}
		return true
	})
	return name
}

// isExempt reports whether pos sits inside a declaration carrying the
// allow directive followed by a non-empty reason.
func isExempt(
	f *ast.File,
	pos token.Pos,
	directive string,
) bool {
	for _, group := range f.Comments {
		if group.End() > pos {
			continue
		}
		if !hasReasonedDirective(group, directive) {
			continue
		}
		if coversPos(f, group, pos) {
			return true
		}
	}
	return false
}

// hasReasonedDirective reports whether group carries the directive
// with a non-empty reason after it.
func hasReasonedDirective(
	group *ast.CommentGroup,
	directive string,
) bool {
	for _, c := range group.List {
		text := strings.TrimSpace(
			strings.TrimPrefix(c.Text, "//"),
		)
		if !strings.HasPrefix(text, directive) {
			continue
		}
		reason := strings.TrimSpace(
			strings.TrimPrefix(text, directive),
		)
		if reason != "" {
			return true
		}
	}
	return false
}

// coversPos reports whether group is the doc comment of a declaration
// that contains pos.
func coversPos(
	f *ast.File,
	group *ast.CommentGroup,
	pos token.Pos,
) bool {
	covered := false
	for _, decl := range f.Decls {
		if pos < decl.Pos() || pos > decl.End() {
			continue
		}
		if declDoc(decl) == group {
			covered = true
		}
	}
	return covered
}

// declDoc returns the doc comment group attached to decl.
func declDoc(decl ast.Decl) *ast.CommentGroup {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Doc
	case *ast.GenDecl:
		return d.Doc
	default:
		return nil
	}
}
