package pipelinecheck

import (
	"go/ast"
	"go/token"
)

// DefaultAllowPopulatedStructDirective exempts a function from
// the populated-struct-literal rule when followed by a non-empty
// reason.
const DefaultAllowPopulatedStructDirective = "fp-go:allow-struct-literal"

const populatedStructMessage = "populated struct literal: " +
	"T{field: value} — construct from T{} with lens.Set in " +
	"F.PipeN"

// hasIdentKeyedElement reports whether elts contains a
// KeyValueExpr whose key is a field identifier.
func hasIdentKeyedElement(elts []ast.Expr) bool {
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if _, isIdent := kv.Key.(*ast.Ident); isIdent {
			return true
		}
	}
	return false
}

// isPopulatedStructLit reports whether lit is a non-empty struct
// composite literal. Empty zero literals (T{}), slice/array
// literals, and map literals are allowed.
func isPopulatedStructLit(lit *ast.CompositeLit) bool {
	if len(lit.Elts) == 0 {
		return false
	}
	switch lit.Type.(type) {
	case *ast.ArrayType, *ast.MapType:
		return false
	case *ast.StructType:
		return true
	default:
		return hasIdentKeyedElement(lit.Elts)
	}
}

// checkNoPopulatedStruct flags populated struct composite
// literals in files that import fp-go or fp-go-loom. Start from
// the zero value T{} and populate fields through lens.Set in
// F.PipeN instead.
func checkNoPopulatedStruct(
	fset *token.FileSet,
	f *ast.File,
	allow string,
) []Violation {
	aliases := fpGoAliases(f)
	if len(aliases) == 0 {
		return nil
	}
	var violations []Violation
	reported := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isPopulatedStructLit(lit) {
			return true
		}
		fnName := enclosingFuncName(f, lit.Pos())
		if exemptionFor(
			&violations,
			fset,
			f,
			fnName,
			allow,
			reported,
		) == exempt {
			return true
		}
		violations = append(violations, Violation{
			Position: fset.Position(lit.Pos()),
			Function: fnName,
			Message:  populatedStructMessage,
		})
		return true
	})
	return violations
}

// CheckNoPopulatedStruct scans cfg.Roots and returns every
// populated struct literal T{field: value} in files that import
// fp-go or fp-go-loom. cfg.AllowPopulatedStructDirective exempts
// a function (defaults to DefaultAllowPopulatedStructDirective).
func CheckNoPopulatedStruct(cfg Config) ([]Violation, error) {
	cfg = withDefaults(cfg)
	return scanFiles(cfg, func(
		fset *token.FileSet,
		f *ast.File,
	) []Violation {
		return checkNoPopulatedStruct(
			fset,
			f,
			cfg.AllowPopulatedStructDirective,
		)
	})
}

// RequireNoPopulatedStruct runs CheckNoPopulatedStruct and fails
// r on every violation or scan error.
func RequireNoPopulatedStruct(r Reporter, cfg Config) {
	r.Helper()
	vs, err := CheckNoPopulatedStruct(cfg)
	reportAll(r, vs, err)
}
