package pipelinecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	wantNewPipeMsg = "new() wraps a pipeline"

	newPipePreamble = `package testpkg
import F "github.com/IBM/fp-go/v2/function"
type opts struct{ A, B bool }
func setA(o opts) opts { o.A = true; return o }
func setB(o opts) opts { o.B = true; return o }
`
)

var newPipeCases = []struct {
	name      string
	body      string
	wantCount int
}{
	{
		name: "new around F.Pipe flagged",
		body: `func bad() *opts {
	return new(F.Pipe2(opts{}, setA, setB))
}`,
		wantCount: 1,
	},
	{
		name:      "applied Flow inside new left to direct-apply rule",
		body:      `var bad = new(F.Flow2(setA, setB)(opts{}))`,
		wantCount: 0,
	},
	{
		name:      "new around a pipeline in var flagged",
		body:      `var bad = new(F.Pipe1(opts{}, setA))`,
		wantCount: 1,
	},
	{
		name: "new of a simple value clean",
		body: `func good(name string) *string { return new(name) }`,
	},
	{
		name: "pipeline returning a value clean",
		body: `func good() opts { return F.Pipe2(opts{}, setA, setB) }`,
	},
}

func TestNewPipe_Table(t *testing.T) {
	for _, tc := range newPipeCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, newPipePreamble+tc.body)
			vs := checkNoNewPipe(
				fset,
				f,
				DefaultAllowNewPipeDirective,
			)
			require.Len(t, vs, tc.wantCount)
			if tc.wantCount > 0 {
				require.Contains(
					t,
					vs[0].Message,
					wantNewPipeMsg,
				)
			}
		})
	}
}

func TestNewPipe_AllowDirective(t *testing.T) {
	fset, f := parse(t, newPipePreamble+`
// fp-go:allow-new-pipe the API stores the pointer
func ok() *opts {
	return new(F.Pipe1(opts{}, setA))
}`)
	require.Empty(
		t,
		checkNoNewPipe(fset, f, DefaultAllowNewPipeDirective),
	)
}
