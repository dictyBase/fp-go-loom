// Package pipelinecheck enforces fp-go style rules via syntax-only
// AST checks. Eight rules are available via dedicated entry points:
//
//  1. Handoff wrapper (Check/Require, default on for entrypoints): a
//     single-step F.Pipe1(seed, namedFn) that delegates the whole
//     continuation to a single-use wrapper.
//  2. Applied seed (Check/Require with Config.RequirePointFreeSeed):
//     the Pipe's first argument is a call that pre-applies entrypoint
//     parameters instead of treating a boundary value as the seed.
//  3. TryCatch raw-effect (CheckNoIfErrInTryCatch): `if err != nil` /
//     `if nil != err` inside an IOE.TryCatchError callback literal.
//     TryCatchError should hold the raw SDK effect; projection and
//     wrapping belong in IOE.MapLeft / IOE.Map.
//  4. Safe Printf (CheckSafePrintf): an IO.Printf call whose literal
//     or package-local constant format string has no real verb,
//     which dumps the value via %!(EXTRA ...) and can leak secrets.
//  5. Redundant Fold (CheckRedundantFold): an E.Fold/E.Match call
//     whose Right (success) arm discards its argument by returning
//     nil. Such a Fold is equivalent to E.ToError; use E.ToError[A]
//     (or F.Flow2(E.ToError[A], leftTransform)) instead of the
//     discard arm.
//  6. Point-Free Branching (CheckBareIf): a bare if inside an fp-go
//     Pipe/Flow or transformation callback. Extract branches to a
//     pre-bound P.Fold and apply the predicate at the pipeline call
//     site. Config.RequirePointFreeBranching also enables it through
//     Check/Require.
//  7. Fixed-root file mapping (CheckNoHandRolledFileJoinInArrayMap):
//     filepath path construction inside an fp-go array.Map mapper.
//     Use one flipped FILE.Join mapper instead.
//  8. Non-raw TryCatch work (CheckNoNonRawTryCatchCallback): clear
//     error wrapping, lens setters, or success projection inside an
//     IOE.TryCatchError callback. Keep TryCatchError raw.
//
// Rules 1 and 2 are entrypoint-scoped (a function matching
// Config.IsEntrypoint, "run*" by default). Rules 3 through 8 apply to
// all functions, since TryCatch, Printf, Fold, and callbacks appear
// anywhere.
//
// Opt a function out of a rule with the matching doc-comment directive,
// each requiring a non-empty reason:
//
//	// fp-go:allow-pipe1-handoff <reason>
//	// fp-go:allow-applied-seed <reason>
//	// fp-go:allow-trycatch-iferr <reason>
//	// fp-go:allow-unsafe-printf <reason>
//	// fp-go:allow-redundant-fold <reason>
//	// fp-go:allow-bare-if <reason>
//	// fp-go:allow-hand-rolled-file-join <reason>
//	// fp-go:allow-non-raw-trycatch <reason>
//
// Limitations: analysis is syntax-only (no go/types), so a shadowed
// alias of a target package may false-positive and a dot import (import
// . ".../pkg") may cause false negatives. Assign-then-return shapes (p
// := F.Pipe1(...); return p) are not inspected; only the returned
// expression is checked. Safe-Printf resolves package-local string
// constants (grouped, inherited, concatenated, and cross-file within
// the same package); cross-package or computed constants are dynamic
// and not flagged. The printf directive parser supports the documented
// fmt verbs, flags, width/precision, argument indexes, and `*` stars
// tested in stylecheck_test.go; `%w` is excluded (it is only
// meaningful to fmt.Errorf). Redundant-Fold inspects the Right arm of
// E.Fold/E.Match (Either only, not IOE.Fold); a Constant* combinator
// wrapping a non-nil value is a legitimate fold and is not flagged.
//
// Fixed-root mapping resolves github.com/IBM/fp-go/v2/array and
// path/filepath imports by canonical path. It only inspects function
// literals passed directly to array.Map, and does not require a
// particular root variable name or Pipe arity.
//
// Non-raw TryCatch analysis resolves github.com/IBM/fp-go/v2/ioeither
// by canonical path. It flags only fmt.Errorf, selector calls named
// Set, and return expressions whose first result is a selector. It
// does not flag arbitrary nested SDK/helper calls, named callbacks,
// or require later MapLeft/Map stages. Its setter heuristic recognizes
// receiver names containing lens or ending in optic; arbitrary SDK
// Set methods remain outside this narrow syntax-only rule.
// Both rules are syntax-only;
// dot imports and shadowed identifiers retain normal syntax-only
// limitations.
//
// Point-Free Branching recognizes canonical function Pipe*/Flow* calls
// plus Map, Chain, and ChainFirst from fp-go Either, IO, IOEither, and
// Option. It does not inspect TryCatchError callbacks, which may contain
// legitimate imperative raw-effect control flow. Its public entry points
// are CheckBareIf and RequireBareIf.
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
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// FunctionPkgPath is the canonical fp-go function-package import path.
const FunctionPkgPath = "github.com/IBM/fp-go/v2/function"

// ArrayPkgPath is the canonical fp-go array-package import path.
const ArrayPkgPath = "github.com/IBM/fp-go/v2/array"

// FilepathPkgPath is the standard-library filepath import path.
const FilepathPkgPath = "path/filepath"

// FmtPkgPath is the standard-library fmt import path.
const FmtPkgPath = "fmt"

// DefaultAllowDirective is the doc-comment marker that exempts a
// function from the handoff rule when followed by a non-empty reason.
const DefaultAllowDirective = "fp-go:allow-pipe1-handoff"

// DefaultAllowAppliedSeedDirective is the doc-comment marker that
// exempts a function from the applied-seed rule when followed by a
// non-empty reason.
const DefaultAllowAppliedSeedDirective = "fp-go:allow-applied-seed"

// IOEitherPkgPath is the canonical fp-go IOEither import path, used
// by the TryCatch raw-effect rule.
const IOEitherPkgPath = "github.com/IBM/fp-go/v2/ioeither"

// IOPkgPath is the canonical fp-go IO import path, used by the
// safe-Printf rule.
const IOPkgPath = "github.com/IBM/fp-go/v2/io"

// EitherPkgPath is the canonical fp-go either-package import path,
// used by the redundant-Fold rule and point-free branching rule.
const EitherPkgPath = "github.com/IBM/fp-go/v2/either"

// OptionPkgPath is the canonical fp-go option-package import path,
// used by the point-free branching rule.
const OptionPkgPath = "github.com/IBM/fp-go/v2/option"

// DefaultAllowTryCatchIfErrDirective exempts a function from the
// TryCatch-if-err rule when followed by a non-empty reason.
const DefaultAllowTryCatchIfErrDirective = "fp-go:allow-trycatch-iferr"

// DefaultAllowHandRolledFileJoinDirective exempts a function from the
// fixed-root file-join rule when followed by a non-empty reason.
const DefaultAllowHandRolledFileJoinDirective = "fp-go:allow-hand-rolled-file-join"

// DefaultAllowNonRawTryCatchDirective exempts a function from the
// non-raw TryCatch rule when followed by a non-empty reason.
const DefaultAllowNonRawTryCatchDirective = "fp-go:allow-non-raw-trycatch"

// DefaultAllowUnsafePrintfDirective exempts a function from the
// safe-Printf rule when followed by a non-empty reason.
const DefaultAllowUnsafePrintfDirective = "fp-go:allow-unsafe-printf"

// DefaultAllowRedundantFoldDirective exempts a function from the
// redundant-Fold rule when followed by a non-empty reason.
const DefaultAllowRedundantFoldDirective = "fp-go:allow-redundant-fold"

// DefaultAllowBareIfDirective exempts a function from the point-free
// branching rule when followed by a non-empty reason.
const DefaultAllowBareIfDirective = "fp-go:allow-bare-if"

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
	// files. Defaults to []string{"."} when empty. Duplicate paths
	// are deduplicated.
	Roots []string

	// IsEntrypoint reports whether a function name is a CLI entrypoint
	// worth checking. Defaults to "has prefix run" when nil.
	IsEntrypoint func(name string) bool

	// AllowDirective exempts a function from the handoff rule. The
	// directive must be followed by a non-empty reason. Defaults to
	// DefaultAllowDirective when empty.
	AllowDirective string

	// AllowAppliedSeedDirective exempts a function from the
	// applied-seed rule. Defaults to DefaultAllowAppliedSeedDirective
	// when empty.
	AllowAppliedSeedDirective string

	// RequirePointFreeSeed enables the applied-seed rule. When false
	// (the default), only the handoff rule runs.
	RequirePointFreeSeed bool

	// AllowTryCatchIfErrDirective exempts a function from the
	// TryCatch-if-err rule (used by CheckNoIfErrInTryCatch).
	// Defaults to DefaultAllowTryCatchIfErrDirective when empty.
	AllowTryCatchIfErrDirective string

	// AllowHandRolledFileJoinDirective exempts a function from the
	// fixed-root file-join rule. Defaults to
	// DefaultAllowHandRolledFileJoinDirective.
	AllowHandRolledFileJoinDirective string

	// AllowNonRawTryCatchDirective exempts a function from the
	// non-raw TryCatch rule. Defaults to
	// DefaultAllowNonRawTryCatchDirective.
	AllowNonRawTryCatchDirective string

	// AllowUnsafePrintfDirective exempts a function from the
	// safe-Printf rule (used by CheckSafePrintf).
	// Defaults to DefaultAllowUnsafePrintfDirective when empty.
	AllowUnsafePrintfDirective string

	// AllowRedundantFoldDirective exempts a function from the
	// redundant-Fold rule (used by CheckRedundantFold).
	// Defaults to DefaultAllowRedundantFoldDirective when empty.
	AllowRedundantFoldDirective string

	// RequirePointFreeBranching enables the bare-if rule for fp-go
	// pipeline callbacks. Defaults to false.
	RequirePointFreeBranching bool

	// AllowBareIfDirective exempts a function from the bare-if rule.
	// Defaults to DefaultAllowBareIfDirective when empty.
	AllowBareIfDirective string
}

// Violation is a single style-rule failure.
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

// Check scans cfg.Roots and returns every entrypoint-rule violation
// (handoff and, when RequirePointFreeSeed is set, applied-seed) that
// lacks a valid exemption. When RequirePointFreeBranching is set, it
// also scans fp-go callbacks for bare if statements. It does NOT run
// the TryCatch or Printf rules; use CheckNoIfErrInTryCatch /
// CheckSafePrintf for those.
func Check(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	parsed, err := parseAll(cfg)
	if err != nil {
		return nil, err
	}
	var violations []Violation
	for _, p := range parsed {
		violations = append(violations,
			checkFile(p.fset, p.f, cfg)...)
	}
	return violations, nil
}

// CheckBareIf scans cfg.Roots and returns every bare if statement
// inside a recognized fp-go pipeline or transformation callback.
// cfg.AllowBareIfDirective exempts a function (defaults to
// DefaultAllowBareIfDirective). TryCatchError callbacks are excluded.
func CheckBareIf(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	parsed, err := parseAll(cfg)
	if err != nil {
		return nil, err
	}
	var violations []Violation
	for _, p := range parsed {
		violations = append(violations, checkBareIf(
			p.fset,
			p.f,
			cfg.AllowBareIfDirective,
		)...)
	}
	return violations, nil
}

// RequireBareIf runs CheckBareIf and fails r on every violation or
// scan error.
func RequireBareIf(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckBareIf(cfg)
	if err != nil {
		r.Fatalf("pipelinecheck: %v", err)
	}
	for _, v := range vs {
		r.Errorf("%s", v)
	}
}

// CheckNoIfErrInTryCatch scans cfg.Roots and returns every
// CheckNoHandRolledFileJoinInArrayMap scans direct fp-go array.Map
// function-literal mappers for filepath.Join or filepath.FromSlash.
// Imports are resolved by canonical path, and
// cfg.AllowHandRolledFileJoinDirective accepts a reasoned function
// opt-out. Use one flipped FILE.Join mapper instead.
func CheckNoHandRolledFileJoinInArrayMap(
	cfg Config,
) ([]Violation, error) {
	cfg = withDefaults(cfg)
	parsed, err := parseAll(cfg)
	if err != nil {
		return nil, err
	}
	var violations []Violation
	for _, p := range parsed {
		violations = append(
			violations,
			checkNoHandRolledFileJoin(
				p.fset,
				p.f,
				cfg.AllowHandRolledFileJoinDirective,
			)...)
	}
	return violations, nil
}

// RequireNoHandRolledFileJoinInArrayMap runs the fixed-root gate.
func RequireNoHandRolledFileJoinInArrayMap(
	r Reporter,
	cfg Config,
) {
	r.Helper()
	vs, err := CheckNoHandRolledFileJoinInArrayMap(cfg)
	if err != nil {
		r.Fatalf("pipelinecheck: %v", err)
	}
	for _, v := range vs {
		r.Errorf("%s", v)
	}
}

// CheckNoIfErrInTryCatch scans cfg.Roots and returns every
// `if err != nil` / `if nil != err` inside an IOE.TryCatchError
// callback literal. cfg.AllowTryCatchIfErrDirective exempts a
// function (defaults to DefaultAllowTryCatchIfErrDirective).
func CheckNoIfErrInTryCatch(
	cfg Config,
) ([]Violation, error) {
	cfg = withDefaults(cfg)
	parsed, err := parseAll(cfg)
	if err != nil {
		return nil, err
	}
	var violations []Violation
	for _, p := range parsed {
		aliases := ioeitherAliases(p.f)
		if len(aliases) == 0 {
			continue
		}
		violations = append(violations,
			checkNoIfErrInTryCatch(
				p.fset, p.f, aliases,
				cfg.AllowTryCatchIfErrDirective, nil,
			)...)
	}
	return violations, nil
}

// CheckNoNonRawTryCatchCallback scans IOE.TryCatchError callback
// literals for clear wrapping, lens-setter, or success-projection work.
func CheckNoNonRawTryCatchCallback(
	cfg Config,
) ([]Violation, error) {
	cfg = withDefaults(cfg)
	parsed, err := parseAll(cfg)
	if err != nil {
		return nil, err
	}
	var violations []Violation
	for _, p := range parsed {
		aliases := ioeitherAliases(p.f)
		if len(aliases) == 0 {
			continue
		}
		violations = append(violations,
			checkNoNonRawTryCatchCallback(
				p.fset,
				p.f,
				aliases,
				cfg.AllowNonRawTryCatchDirective,
			)...)
	}
	return violations, nil
}

// RequireNoNonRawTryCatchCallback runs the non-raw TryCatch gate.
func RequireNoNonRawTryCatchCallback(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoNonRawTryCatchCallback(cfg)
	if err != nil {
		r.Fatalf("pipelinecheck: %v", err)
	}
	for _, v := range vs {
		r.Errorf("%s", v)
	}
}

// RequireNoIfErrInTryCatch runs CheckNoIfErrInTryCatch and fails r on
// every violation or scan error.
func RequireNoIfErrInTryCatch(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoIfErrInTryCatch(cfg)
	if err != nil {
		r.Fatalf("pipelinecheck: %v", err)
	}
	for _, v := range vs {
		r.Errorf("%s", v)
	}
}

// CheckSafePrintf scans cfg.Roots and returns every IO.Printf call
// whose literal or package-local constant format string has no real
// verb. cfg.AllowUnsafePrintfDirective exempts a function (defaults
// to DefaultAllowUnsafePrintfDirective).
func CheckSafePrintf(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	parsed, err := parseAll(cfg)
	if err != nil {
		return nil, err
	}
	pkgConst := buildPkgConsts(parsed)
	var violations []Violation
	for _, p := range parsed {
		aliases := ioAliases(p.f)
		if len(aliases) == 0 {
			continue
		}
		consts := pkgConst[pkgKey(p.fset, p.f)]
		violations = append(violations,
			checkSafePrintf(
				p.fset, p.f, aliases,
				cfg.AllowUnsafePrintfDirective, consts,
			)...)
	}
	return violations, nil
}

// RequireSafePrintf runs CheckSafePrintf and fails r on every
// violation or scan error.
func RequireSafePrintf(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckSafePrintf(cfg)
	if err != nil {
		r.Fatalf("pipelinecheck: %v", err)
	}
	for _, v := range vs {
		r.Errorf("%s", v)
	}
}

// CheckRedundantFold scans cfg.Roots and returns every E.Fold/E.Match
// call whose Right (success) arm discards its argument by returning
// nil. Such a Fold is equivalent to E.ToError; use E.ToError[A] (or
// F.Flow2(E.ToError[A], leftTransform)) instead of spelling the
// discard arm. cfg.AllowRedundantFoldDirective exempts a function
// (defaults to DefaultAllowRedundantFoldDirective).
func CheckRedundantFold(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	parsed, err := parseAll(cfg)
	if err != nil {
		return nil, err
	}
	var violations []Violation
	for _, p := range parsed {
		aliases := eitherAliases(p.f)
		if len(aliases) == 0 {
			continue
		}
		violations = append(violations,
			checkRedundantFold(
				p.fset, p.f, aliases,
				cfg.AllowRedundantFoldDirective,
			)...)
	}
	return violations, nil
}

// RequireNoRedundantFold runs CheckRedundantFold and fails r on every
// violation or scan error.
func RequireNoRedundantFold(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckRedundantFold(cfg)
	if err != nil {
		r.Fatalf("pipelinecheck: %v", err)
	}
	for _, v := range vs {
		r.Errorf("%s", v)
	}
}

// ModuleRoot walks up from the test working directory until it finds
// a directory containing go.mod, returning that path. It fatals r when
// go.mod cannot be found, so a wired gate test fails fast instead of
// silently scanning the wrong tree. Use it to make a gate's Roots
// resolve the whole module regardless of which package the test lives
// in. Custom Reporter implementations that do not terminate on
// Fatalf get an empty-string return after the loop.
func ModuleRoot(r Reporter) string {
	r.Helper()
	dir, err := os.Getwd()
	if err != nil {
		r.Fatalf("pipelinecheck: getwd: %v", err)
		return ""
	}
	for {
		if _, err := os.Stat(
			filepath.Join(dir, "go.mod"),
		); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			r.Fatalf(
				"pipelinecheck: go.mod not found walking up from %s",
				dir,
			)
			return ""
		}
		dir = parent
	}
}

// Require runs Check and fails r on every violation or scan error.
// Set Config.RequirePointFreeBranching to include the bare-if rule.
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
	if cfg.AllowAppliedSeedDirective == "" {
		cfg.AllowAppliedSeedDirective = DefaultAllowAppliedSeedDirective
	}
	applyDefault(&cfg.AllowTryCatchIfErrDirective,
		DefaultAllowTryCatchIfErrDirective)
	applyDefault(&cfg.AllowHandRolledFileJoinDirective,
		DefaultAllowHandRolledFileJoinDirective)
	applyDefault(&cfg.AllowNonRawTryCatchDirective,
		DefaultAllowNonRawTryCatchDirective)
	applyDefault(&cfg.AllowUnsafePrintfDirective,
		DefaultAllowUnsafePrintfDirective)
	applyDefault(&cfg.AllowRedundantFoldDirective,
		DefaultAllowRedundantFoldDirective)
	applyDefault(&cfg.AllowBareIfDirective,
		DefaultAllowBareIfDirective)
	return cfg
}

func applyDefault(target *string, value string) {
	if *target == "" {
		*target = value
	}
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
// pruned. The walk roots "." and ".." are NOT pruned (".." starts
// with "." but is a legitimate parent-dir root).
func isSkippedDir(name string) bool {
	if name == "." || name == ".." {
		return false
	}
	if strings.HasPrefix(name, ".") || name == "vendor" {
		return true
	}
	return name == "testdata" || strings.HasPrefix(name, "_")
}

// parsedFile is a parsed non-test .go file with its token.FileSet.
type parsedFile struct {
	fset *token.FileSet
	f    *ast.File
}

// parseAll parses every non-test .go file under cfg.Roots once and
// returns them in scan order. It deduplicates paths across roots.
func parseAll(cfg Config) ([]parsedFile, error) {
	seen := make(map[string]bool)
	var out []parsedFile
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
			fset := token.NewFileSet()
			f, err := parser.ParseFile(
				fset,
				file,
				nil,
				parser.ParseComments,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"parse %s: %w", file, err,
				)
			}
			out = append(out, parsedFile{fset, f})
		}
	}
	return out, nil
}

// buildPkgConsts groups parsed files by package identity and
// resolves each package's string constants once, returning pkgKey ->
// name -> value.
func buildPkgConsts(
	parsed []parsedFile,
) map[string]map[string]string {
	pkgFiles := make(map[string][]*ast.File)
	for _, p := range parsed {
		k := pkgKey(p.fset, p.f)
		pkgFiles[k] = append(pkgFiles[k], p.f)
	}
	out := make(map[string]map[string]string, len(pkgFiles))
	for k, fs := range pkgFiles {
		out[k] = pkgConsts(fs)
	}
	return out
}

// checkFile inspects a parsed file and returns entrypoint-rule
// violations (handoff and, when RequirePointFreeSeed is set,
// applied-seed). Tests call it directly with in-memory parse results.
func checkFile(
	fset *token.FileSet,
	f *ast.File,
	cfg Config,
) []Violation {
	var violations []Violation
	if cfg.RequirePointFreeBranching {
		violations = append(violations, checkBareIf(
			fset,
			f,
			cfg.AllowBareIfDirective,
		)...)
	}
	fnAliases := functionAliases(f)
	if len(fnAliases) == 0 {
		return violations
	}
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil ||
			!cfg.IsEntrypoint(fn.Name.Name) {
			return true
		}
		appendFuncViolations(
			&violations,
			fset,
			fn,
			fnAliases,
			cfg,
		)
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
	handoffStatus := exemptionStatus(fn, cfg.AllowDirective)
	appendDirectiveErrors(
		out,
		fset,
		fn,
		handoffStatus,
		cfg.AllowDirective,
	)
	var seedStatus exemption
	if cfg.RequirePointFreeSeed {
		seedStatus = exemptionStatus(
			fn,
			cfg.AllowAppliedSeedDirective,
		)
		appendDirectiveErrors(
			out,
			fset,
			fn,
			seedStatus,
			cfg.AllowAppliedSeedDirective,
		)
	}
	params := entrypointParams(fn)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, expr := range ret.Results {
			call, ok := expr.(*ast.CallExpr)
			if !ok {
				continue
			}
			if handoffStatus == notExempt &&
				isPipe1Handoff(call, aliases) {
				*out = append(*out, Violation{
					Position: fset.Position(call.Pos()),
					Function: fn.Name.Name,
					Message: "returns F.Pipe1(seed, namedFn) — " +
						"inline the wrapper's steps into the " +
						"entrypoint F.PipeN " +
						"(fp-go-pipe-flow Anti-patterns)",
				})
			}
			if cfg.RequirePointFreeSeed &&
				seedStatus == notExempt &&
				isAppliedSeedPipe(call, aliases, params) {
				*out = append(*out, Violation{
					Position: fset.Position(call.Pos()),
					Function: fn.Name.Name,
					Message: "Pipe seed is an applied call " +
						"referencing entrypoint params — " +
						"construct a boundary-input value and " +
						"move the constructor into the Pipe " +
						"(fp-go-pipe-flow Anti-patterns)",
				})
			}
		}
		return true
	})
}

func appendDirectiveErrors(
	out *[]Violation,
	fset *token.FileSet,
	fn *ast.FuncDecl,
	status exemption,
	directive string,
) {
	if status != directiveWithoutReason {
		return
	}
	*out = append(*out, Violation{
		Position: fset.Position(fn.Pos()),
		Function: fn.Name.Name,
		Message: directive +
			" directive requires a non-empty reason",
	})
}

type exemption int

const (
	notExempt exemption = iota
	exempt
	directiveWithoutReason
)

// exemptionStatus inspects fn's doc comment for directive. A directive
// followed by a word boundary and a non-empty reason exempts the
// function. A directive with no reason is a separate violation so the
// author knows why the exemption did not apply. A typo'd directive
// (e.g. allow-pipe1-handoffx) is not matched.
func exemptionStatus(
	fn *ast.FuncDecl,
	directive string,
) exemption {
	if fn == nil || fn.Doc == nil {
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
// importAliasesFor returns the set of import aliases that resolve to
// pkgPath in f. A file may import a package under a custom alias or
// the default package name; a blank import (_) contributes nothing.
func importAliasesFor(
	f *ast.File,
	pkgPath string,
) map[string]bool {
	aliases := make(map[string]bool)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path != pkgPath {
			continue
		}
		if imp.Name == nil {
			aliases[filepath.Base(path)] = true
			continue
		}
		if imp.Name.Name == "_" {
			continue
		}
		aliases[imp.Name.Name] = true
	}
	return aliases
}

// functionAliases returns aliases for the fp-go function package.
func functionAliases(f *ast.File) map[string]bool {
	return importAliasesFor(f, FunctionPkgPath)
}

// ioeitherAliases returns aliases for the fp-go IOEither package.
func ioeitherAliases(f *ast.File) map[string]bool {
	return importAliasesFor(f, IOEitherPkgPath)
}

// ioAliases returns aliases for the fp-go IO package.
func ioAliases(f *ast.File) map[string]bool {
	return importAliasesFor(f, IOPkgPath)
}

// eitherAliases returns aliases for the fp-go either package.
func eitherAliases(f *ast.File) map[string]bool {
	return importAliasesFor(f, EitherPkgPath)
}

// enclosingFuncName returns the name of the FuncDecl containing pos.
// When pos is at package (top-level) scope, it returns "<package>" so
// package-scope violations are attributed consistently.
func enclosingFuncName(f *ast.File, pos token.Pos) string {
	var match string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		if fn.Pos() <= pos && pos <= fn.End() {
			match = fn.Name.Name
		}
		return true
	})
	if match == "" {
		return "<package>"
	}
	return match
}

// funcDeclFor returns the *ast.FuncDecl named name in f, or nil.
func funcDeclFor(f *ast.File, name string) *ast.FuncDecl {
	if name == "" {
		return nil
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// exemptionFor resolves the exemption status for the function enclosing
// pos. When the directive is present but lacks a reason it appends a
// "directive requires a non-empty reason" violation to out (once per
// function, tracked in reported) and returns exempt so the caller
// suppresses the underlying rule violation.
func exemptionFor(
	out *[]Violation,
	fset *token.FileSet,
	f *ast.File,
	fnName string,
	directive string,
	reported map[string]bool,
) exemption {
	status := exemptionStatus(funcDeclFor(f, fnName), directive)
	if status != directiveWithoutReason {
		return status
	}
	if !reported[fnName] {
		reported[fnName] = true
		if fn := funcDeclFor(f, fnName); fn != nil {
			*out = append(*out, Violation{
				Position: fset.Position(fn.Pos()),
				Function: fnName,
				Message: directive +
					" directive requires a non-empty reason",
			})
		}
	}
	return exempt // suppress the underlying violation
}

// entrypointParams returns the set of named parameter identifiers of
// fn, used to detect applied-seed calls that reference them.
func entrypointParams(fn *ast.FuncDecl) map[string]bool {
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

func isPipe1Handoff(
	call *ast.CallExpr,
	aliases map[string]bool,
) bool {
	sel := pipeSelector(call.Fun)
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

// isAppliedSeedPipe flags an F.PipeN(...) call whose first argument
// (the seed) is itself a call that references one or more entrypoint
// parameters. The fix is to make the seed a boundary-input value and
// move the constructor into the Pipe as a unary step.
func isAppliedSeedPipe(
	call *ast.CallExpr,
	aliases map[string]bool,
	params map[string]bool,
) bool {
	sel := pipeSelector(call.Fun)
	if sel == nil || !strings.HasPrefix(sel.Sel.Name, "Pipe") {
		return false
	}
	if !isFunctionAlias(sel.X, aliases) {
		return false
	}
	if len(call.Args) < 2 {
		return false
	}
	seed, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	return referencesParams(seed, params)
}

// referencesParams reports whether expr contains any identifier
// matching a name in params.
func referencesParams(
	expr ast.Expr,
	params map[string]bool,
) bool {
	if len(params) == 0 {
		return false
	}
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if ok && params[ident.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

// pipeSelector unwraps generic instantiation
// (F.Pipe1[int, error](...)) to reach the underlying SelectorExpr.
func pipeSelector(fun ast.Expr) *ast.SelectorExpr {
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
