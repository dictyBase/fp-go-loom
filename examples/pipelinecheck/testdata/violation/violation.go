// Package violation is a deliberate pipelinecheck fixture. It fails
// the trivial-delegate rule and exists only as gate test data. Do not
// copy this shape into production code.
package violation

import (
	F "github.com/IBM/fp-go/v2/function"
)

// wrap is the pass-through wrapper the gate must flag.
func wrap(s string) string {
	return helper(s)
}

// pipe references wrap from an F.Pipe site, which is what makes wrap
// reportable.
func pipe(s string) string {
	return F.Pipe1(s, wrap)
}

// helper is the pass-through target.
func helper(s string) string {
	return s + "!"
}
