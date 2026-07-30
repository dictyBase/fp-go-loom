package pipelinecheck

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

// checkSafePrintf flags IO.Printf calls whose literal or package-local
// constant format string has no real formatting verb. A `%`-free
// format dumps the value via `%!(EXTRA type=value)` and can leak
// secrets such as API keys (durable rule 5); `%%` is an escaped
// percent, not a verb. Package-local string constants (and
// concatenations of them) are resolved so `IO.Printf(constFmt)` is
// inspected; cross-package or computed constants are dynamic and not
// flagged.
func checkSafePrintf(
	fset *token.FileSet,
	f *ast.File,
	aliases map[string]bool,
	allow string,
	consts map[string]string,
) []Violation {
	var violations []Violation
	reported := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel := pipeSelector(call.Fun)
		if sel == nil || sel.Sel.Name != "Printf" {
			return true
		}
		if !isFunctionAlias(sel.X, aliases) {
			return true
		}
		format, ok := resolveFormat(call.Args, consts)
		if !ok {
			return true // dynamic format — not inspected
		}
		if hasRealVerb(format) {
			return true
		}
		fnName := enclosingFuncName(f, call.Pos())
		if exemptionFor(&violations, fset, f, fnName,
			allow, reported) == exempt {
			return true
		}
		violations = append(violations, Violation{
			Position: fset.Position(call.Pos()),
			Function: fnName,
			Message: "IO.Printf format has no " +
				"formatting verb — a bare value " +
				"dump can leak secrets; use a verb " +
				"or IO.PrintGo (durable rule 5)",
		})
		return true
	})
	return violations
}

// resolveFormat returns the unquoted string when the first call
// argument is a string literal, a package-local string constant, or a
// concatenation of those. Constants from other packages or computed
// values are dynamic (ok=false) and not flagged.
func resolveFormat(
	args []ast.Expr,
	consts map[string]string,
) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	return foldLiteral(args[0], consts)
}

// foldLiteral unquotes a string literal, resolves a package-local
// string constant identifier, or concatenates a chain of literal/
// constant additions left-to-right. Returns ok=false for any
// non-constant operand.
func foldLiteral(
	e ast.Expr,
	consts map[string]string,
) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return unquoteString(v.Value)
		}
		return "", false
	case *ast.Ident:
		if s, ok := consts[v.Name]; ok {
			return s, true
		}
		return "", false
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		ls, ok := foldLiteral(v.X, consts)
		if !ok {
			return "", false
		}
		rs, ok := foldLiteral(v.Y, consts)
		if !ok {
			return "", false
		}
		return ls + rs, true
	case *ast.ParenExpr:
		return foldLiteral(v.X, consts)
	}
	return "", false
}

func unquoteString(s string) (string, bool) {
	out, err := strconv.Unquote(s)
	if err != nil {
		return "", false
	}
	return out, true
}

// pkgConsts builds a name -> resolved string value map for all
// package-local string constants in a package (a directory of non-test
// .go files sharing a package). It is a two-pass resolver:
//
//  1. Gather every `const name = <expr>` (grouped `const a, b = "x",
//     "y"` and omitted-RHS inheritance `const ( a = "x"; b )`) across
//     all files in the package, mapping each name to its raw
//     initializer expression.
//  2. Resolve each initializer recursively: string literals unquote,
//     identifiers look up other package consts, `+` concatenates,
//     and parentheses unwrap. Non-string or computed constants
//     resolve to ok=false and are dropped.
//
// Cross-package constants are not resolved (they are dynamic from
// this syntax-only view); a `IO.Printf(otherpkg.Fmt)` is therefore not
// flagged.
func pkgConsts(files []*ast.File) map[string]string {
	raw := gatherConstExprs(files)
	resolved := make(map[string]string, len(raw))
	for name := range raw {
		resolveConst(name, raw, resolved, map[string]bool{})
	}
	return resolved
}

// gatherConstExprs collects every file-local string-const initializer
// across the package into name -> raw expression, honouring grouped,
// multi-name, and omitted-RHS inheritance.
func gatherConstExprs(
	files []*ast.File,
) map[string]ast.Expr {
	raw := make(map[string]ast.Expr)
	for _, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			var lastValues []ast.Expr
			for _, spec := range gd.Specs {
				lastValues = gatherSpec(raw, spec, lastValues)
			}
		}
	}
	return raw
}

// gatherSpec folds one ValueSpec into raw, returning the updated
// lastValues cursor for omitted-RHS inheritance.
func gatherSpec(
	raw map[string]ast.Expr,
	spec ast.Spec,
	lastValues []ast.Expr,
) []ast.Expr {
	vs, ok := spec.(*ast.ValueSpec)
	if !ok {
		return lastValues
	}
	switch {
	case len(vs.Values) == len(vs.Names):
		lastValues = vs.Values
		for i, name := range vs.Names {
			raw[name.Name] = vs.Values[i]
		}
	case len(vs.Values) == 0 && len(lastValues) == len(vs.Names):
		for i, name := range vs.Names {
			raw[name.Name] = lastValues[i]
		}
	default:
		lastValues = nil
	}
	return lastValues
}

// resolveConst resolves a single const name, memoising into resolved.
// The visiting set breaks reference cycles (Go disallows them, but
// the resolver is defensive).
func resolveConst(
	name string,
	raw map[string]ast.Expr,
	resolved map[string]string,
	visiting map[string]bool,
) {
	if _, ok := resolved[name]; ok {
		return
	}
	if visiting[name] {
		return
	}
	visiting[name] = true
	defer delete(visiting, name)
	expr, ok := raw[name]
	if !ok {
		return
	}
	if v, ok := foldConstExpr(
		expr, raw, resolved, visiting,
	); ok {
		resolved[name] = v
	}
}

// foldConstExpr folds an initializer expression to a string value,
// resolving identifiers against the package const set.
func foldConstExpr(
	e ast.Expr,
	raw map[string]ast.Expr,
	resolved map[string]string,
	visiting map[string]bool,
) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return unquoteString(v.Value)
		}
		return "", false
	case *ast.Ident:
		return foldConstIdent(v, raw, resolved, visiting)
	case *ast.BinaryExpr:
		return foldConstBinary(v, raw, resolved, visiting)
	case *ast.ParenExpr:
		return foldConstExpr(v.X, raw, resolved, visiting)
	}
	return "", false
}

// foldConstIdent resolves an identifier against the package const set.
func foldConstIdent(
	v *ast.Ident,
	raw map[string]ast.Expr,
	resolved map[string]string,
	visiting map[string]bool,
) (string, bool) {
	if rv, ok := resolved[v.Name]; ok {
		return rv, true
	}
	if _, has := raw[v.Name]; has {
		resolveConst(v.Name, raw, resolved, visiting)
		if rv, ok := resolved[v.Name]; ok {
			return rv, true
		}
	}
	return "", false
}

// foldConstBinary folds a `+` concatenation of two constant exprs.
func foldConstBinary(
	v *ast.BinaryExpr,
	raw map[string]ast.Expr,
	resolved map[string]string,
	visiting map[string]bool,
) (string, bool) {
	if v.Op != token.ADD {
		return "", false
	}
	ls, ok := foldConstExpr(v.X, raw, resolved, visiting)
	if !ok {
		return "", false
	}
	rs, ok := foldConstExpr(v.Y, raw, resolved, visiting)
	if !ok {
		return "", false
	}
	return ls + rs, true
}

// pkgKey returns a package identity key (directory + package name)
// so constants are grouped per package, not per directory alone.
// Build-tag or platform files in the same directory but a different
// package do not leak constants into each other.
func pkgKey(fset *token.FileSet, f *ast.File) string {
	pos := fset.Position(f.Pos())
	return filepath.Join(filepath.Dir(pos.Filename), f.Name.Name)
}

// hasRealVerb reports whether format contains a real printf verb. See
// scanDirective for the per-`%` parsing.
func hasRealVerb(format string) bool {
	runes := []rune(format)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '%' {
			continue
		}
		if next, verb := scanDirective(runes, i); verb {
			return true
		} else if next > i {
			i = next - 1
		}
	}
	return false
}

// scanDirective parses one printf directive starting at runes[i] == '%'.
// It returns the index past the directive and whether a real verb was
// found. `%%` advances past the escaped percent with verb=false.
func scanDirective(runes []rune, i int) (int, bool) {
	j := i + 1
	if j < len(runes) && runes[j] == '%' {
		return j + 1, false // %%
	}
	j = consumeDirectivePrefix(runes, j)
	if j < len(runes) && isVerbRune(runes[j]) {
		return j + 1, true
	}
	return i + 1, false
}

// consumeDirectivePrefix advances past the optional flags, width,
// precision, and trailing argument index that precede the verb.
func consumeDirectivePrefix(runes []rune, j int) int {
	for j < len(runes) && strings.ContainsRune("+- 0#", runes[j]) {
		j++
	}
	j = consumeWidth(runes, j)
	if j < len(runes) && runes[j] == '.' {
		j++
		j = consumeWidth(runes, j)
	}
	if j < len(runes) && runes[j] == '[' {
		j = consumeIndex(runes, j)
	}
	return j
}

// consumeIndex advances past '[n]' and returns the index after ']'.
func consumeIndex(runes []rune, j int) int {
	for j < len(runes) && runes[j] != ']' {
		j++
	}
	if j < len(runes) {
		j++
	}
	return j
}

// consumeWidth advances past one width/precision token: a run of
// digits, a single '*', or an indexed star '[n]*'.
func consumeWidth(runes []rune, j int) int {
	if j >= len(runes) {
		return j
	}
	if runes[j] == '*' {
		return j + 1
	}
	if runes[j] == '[' {
		return consumeIndexedStar(runes, j)
	}
	for j < len(runes) && runes[j] >= '0' && runes[j] <= '9' {
		j++
	}
	return j
}

// consumeIndexedStar advances past '[n]*' and returns the index after
// the star, or after ']' when no star follows.
func consumeIndexedStar(runes []rune, j int) int {
	for j < len(runes) && runes[j] != ']' {
		j++
	}
	if j < len(runes) {
		j++
	}
	if j < len(runes) && runes[j] == '*' {
		j++
	}
	return j
}

// isVerbRune reports whether r is a recognised fmt verb rune usable in
// Printf: v, d, s, q, c, b, o, O, x, X, T, t, f, F, e, E, g, G, p, U.
// `%w` is intentionally excluded — it is only meaningful to
// fmt.Errorf. Flags/width/precision and index brackets are consumed
// before the terminal verb and are not verbs themselves.
func isVerbRune(r rune) bool {
	return strings.ContainsRune("vdsqcboOxXTtfFeEgGpU", r)
}
