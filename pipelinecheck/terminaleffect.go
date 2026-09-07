package pipelinecheck

import (
	"go/ast"
	"go/token"
	"strings"
)

const duplicateTerminalMessage = "terminal effect reached from " +
	"both fold arms — converge both branches on one description " +
	"value, then execute the effect once after the fold"

const recoveryAfterTerminalMessage = "recovery step placed after a " +
	"terminal effect — a failure raised inside the effect re-enters " +
	"the recovery arm and executes the effect twice; recover before " +
	"the terminal step"

// terminalDepth bounds how many package-local hops the reachability
// walk follows from a pipeline step to a terminal function. One hop
// covers the common `IOE.Chain(respondJSON)` wrapper around
// `writeResponse`; deeper chains are out of scope for a syntax-only
// rule.
const terminalDepth = 2

// terminalSet is the configured set of terminal function names.
type terminalSet map[string]bool

// newTerminalSet indexes cfg.TerminalFunctions by bare name. Both
// `writeResponse` and a method name such as `Send` are matched on the
// identifier alone, so a receiver expression does not need resolving.
func newTerminalSet(names []string) terminalSet {
	set := make(terminalSet, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			set[trimmed] = true
		}
	}
	return set
}

// packageFuncs indexes the top-level function declarations of one
// package by name, enabling package-local reachability without type
// information.
func packageFuncs(files []*ast.File) map[string]*ast.FuncDecl {
	out := make(map[string]*ast.FuncDecl)
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil {
				continue
			}
			out[fn.Name.Name] = fn
		}
	}
	return out
}

// referencedName returns the identifier a node contributes to the
// reachability walk: the selected name for a selector, the identifier
// name otherwise.
func referencedName(n ast.Node) string {
	switch e := n.(type) {
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.Ident:
		return e.Name
	}
	return ""
}

// reachesTerminal reports whether node mentions a terminal function,
// either directly or through at most depth package-local function
// declarations. seen guards against recursive declarations.
func reachesTerminal(
	node ast.Node,
	terms terminalSet,
	funcs map[string]*ast.FuncDecl,
	depth int,
	seen map[string]bool,
) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		name := referencedName(n)
		if name == "" {
			return true
		}
		if terms[name] {
			found = true
			return false
		}
		if depth <= 0 || seen[name] {
			return true
		}
		decl, ok := funcs[name]
		if !ok {
			return true
		}
		seen[name] = true
		found = reachesTerminal(
			decl.Body, terms, funcs, depth-1, seen,
		)
		return !found
	})
	return found
}

// stepReachesTerminal is reachesTerminal with a fresh visited set.
func stepReachesTerminal(
	node ast.Node,
	terms terminalSet,
	funcs map[string]*ast.FuncDecl,
) bool {
	return reachesTerminal(
		node, terms, funcs, terminalDepth, map[string]bool{},
	)
}

// checkDuplicateTerminalFold flags Fold / Match calls over Either or
// IOEither whose two arms both reach a terminal function. Such a fold
// writes, sends or commits once per branch, so the two call sites
// drift and a late failure cannot be recovered without emitting the
// effect twice. Converge both arms on a description value and execute
// once after the fold.
func checkDuplicateTerminalFold(
	fset *token.FileSet,
	f *ast.File,
	aliases map[string]bool,
	terms terminalSet,
	funcs map[string]*ast.FuncDecl,
	allow string,
) []Violation {
	var violations []Violation
	reported := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if !isTerminalFold(call, aliases) {
			return true
		}
		if !bothArmsReachTerminal(call, terms, funcs) {
			return true
		}
		fnName := enclosingFuncName(f, call.Pos())
		if exemptionFor(
			&violations, fset, f, fnName, allow, reported,
		) == exempt {
			return true
		}
		violations = append(violations, Violation{
			Position: fset.Position(call.Pos()),
			Function: fnName,
			Message:  duplicateTerminalMessage,
		})
		return true
	})
	return violations
}

// isTerminalFold reports whether call is a two-arm Fold or Match on
// an Either or IOEither alias.
func isTerminalFold(
	call *ast.CallExpr,
	aliases map[string]bool,
) bool {
	sel := pipeSelector(call.Fun)
	if sel == nil {
		return false
	}
	if sel.Sel.Name != "Fold" && sel.Sel.Name != "Match" {
		return false
	}
	if !isFunctionAlias(sel.X, aliases) {
		return false
	}
	return len(call.Args) >= 2
}

// bothArmsReachTerminal reports whether each of the first two fold
// arms reaches a terminal function.
func bothArmsReachTerminal(
	call *ast.CallExpr,
	terms terminalSet,
	funcs map[string]*ast.FuncDecl,
) bool {
	return stepReachesTerminal(call.Args[0], terms, funcs) &&
		stepReachesTerminal(call.Args[1], terms, funcs)
}

// checkRecoveryAfterTerminal flags an OrElse step positioned after a
// terminal step inside one F.PipeN call. Recovery must wrap only the
// fallible work that precedes the effect; wrapping the effect itself
// means a failure inside it re-enters the recovery arm and the effect
// runs a second time.
func checkRecoveryAfterTerminal(
	fset *token.FileSet,
	f *ast.File,
	fnAliases map[string]bool,
	terms terminalSet,
	funcs map[string]*ast.FuncDecl,
	allow string,
) []Violation {
	var violations []Violation
	reported := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isPipeCall(call, fnAliases) {
			return true
		}
		recovery, terminal := recoveryAndTerminalIndex(
			call, terms, funcs,
		)
		if recovery < 0 || terminal < 0 ||
			terminal > recovery {
			return true
		}
		fnName := enclosingFuncName(f, call.Pos())
		if exemptionFor(
			&violations, fset, f, fnName, allow, reported,
		) == exempt {
			return true
		}
		violations = append(violations, Violation{
			Position: fset.Position(
				call.Args[recovery].Pos(),
			),
			Function: fnName,
			Message:  recoveryAfterTerminalMessage,
		})
		return true
	})
	return violations
}

// isPipeCall reports whether call is F.Pipe / F.PipeN on the fp-go
// function-package alias.
func isPipeCall(
	call *ast.CallExpr,
	fnAliases map[string]bool,
) bool {
	sel := pipeSelector(call.Fun)
	if sel == nil || !isFunctionAlias(sel.X, fnAliases) {
		return false
	}
	return strings.HasPrefix(sel.Sel.Name, "Pipe")
}

// recoveryAndTerminalIndex returns the argument index of the first
// OrElse step and of the first terminal step, or -1 when absent. The
// recovery step itself is never counted as terminal: its arm is the
// second emission the rule exists to prevent, and reporting it as the
// cause would point at the wrong step.
func recoveryAndTerminalIndex(
	call *ast.CallExpr,
	terms terminalSet,
	funcs map[string]*ast.FuncDecl,
) (int, int) {
	recovery, terminal := -1, -1
	for i, arg := range call.Args {
		if isRecoveryStep(arg) {
			if recovery < 0 {
				recovery = i
			}
			continue
		}
		if terminal < 0 &&
			stepReachesTerminal(arg, terms, funcs) {
			terminal = i
		}
	}
	return recovery, terminal
}

// isRecoveryStep reports whether a pipeline step is an OrElse call.
func isRecoveryStep(arg ast.Expr) bool {
	call, ok := arg.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel := pipeSelector(call.Fun)
	return sel != nil && sel.Sel.Name == "OrElse"
}
