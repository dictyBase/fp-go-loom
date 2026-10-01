package pipelinecheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	wantDelegateMsg      = "only delegates to"
	fetchWorkByShapeName = "fetchWorkByShape"
)

var trivialDelegateCases = []checkCase{
	{
		name: "pass-through wrapper in IOE.Chain flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	return fetchByShape(st)
}
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWorkByShape)(fetchByShape(st))
}`,
		wantCount: 1,
		wantMsg:   wantDelegateMsg,
		wantFunc:  fetchWorkByShapeName,
	},
	{
		name: "wrapper referenced from F.Pipe2 flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	return fetchByShape(st)
}
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}
func runPipe(st State) IOE.IOEither[error, State] {
	return F.Pipe2(
		fetchByShape(st),
		fetchWorkByShape,
	)
}`,
		wantCount: 1,
		wantMsg:   wantDelegateMsg,
		wantFunc:  fetchWorkByShapeName,
	},
	{
		name: "multi-statement body clean",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	err := precheck(st)
	if err != nil {
		return IOE.Left[error](err)
	}
	return fetchByShape(st)
}
func precheck(State) error { return nil }
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWorkByShape)(fetchByShape(st))
}`,
	},
	{
		name: "call wrapped in error transform clean",
		src: `package testpkg
import (
	"fmt"
	F "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
type State struct{ Work int }
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	return F.Pipe2(
		fetchByShape(st),
		IOE.MapLeft[error](func(e error) error {
			return fmt.Errorf("fetch: %w", e)
		}),
	)
}
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWorkByShape)(fetchByShape(st))
}`,
	},
	{
		name: "unreferenced wrapper clean",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	return fetchByShape(st)
}
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}`,
	},
	{
		name: "method-on-param argument flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }
func (State) Fetch() IOE.IOEither[error, State] {
	return IOE.Of[error](State{})
}
func fetchWork(st State) IOE.IOEither[error, State] {
	return st.Fetch()
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWork)(st.Fetch())
}`,
		wantCount: 1,
		wantMsg:   wantDelegateMsg,
		wantFunc:  "fetchWork",
	},
	{
		name: "extra constant arg alongside param flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }
const pageSize = 25
func fetchWork(st State) IOE.IOEither[error, State] {
	return fetchPage(st, pageSize)
}
func fetchPage(State, int) IOE.IOEither[error, State] {
	return IOE.Of[error](State{})
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWork)(fetchPage(st, pageSize))
}`,
		wantCount: 1,
		wantMsg:   wantDelegateMsg,
		wantFunc:  "fetchWork",
	},
	{
		name: "arg not rooted at param clean",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }
func fetchWork(st State) IOE.IOEither[error, State] {
	return fetchByShape(other())
}
func other() State { return State{} }
func fetchByShape(State) IOE.IOEither[error, State] {
	return IOE.Of[error](State{})
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWork)(fetchByShape(st))
}`,
	},
	{
		name: "directive exempts wrapper",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }

// fp-go:allow-trivial-delegate kept for API stability
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	return fetchByShape(st)
}
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWorkByShape)(fetchByShape(st))
}`,
	},
	{
		name: "directive without reason is itself flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }

// fp-go:allow-trivial-delegate
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	return fetchByShape(st)
}
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWorkByShape)(fetchByShape(st))
}`,
		wantCount: 1,
		wantMsg:   "requires a non-empty reason",
		wantFunc:  fetchWorkByShapeName,
	},
}

func TestTrivialDelegate_Table(t *testing.T) {
	for _, tc := range trivialDelegateCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeFixture(t, tc.src)
			vs, err := CheckTrivialDelegate(Config{
				Roots: []string{dir},
			})
			require.NoError(t, err)
			require.Len(t, vs, tc.wantCount)
			for _, v := range vs {
				require.Contains(t, v.Message, tc.wantMsg)
				if tc.wantFunc != "" {
					require.Equal(
						t,
						tc.wantFunc,
						v.Function,
					)
				}
			}
		})
	}
}

func TestTrivialDelegate_CustomDirective(t *testing.T) {
	src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }

// custom:allow-delegate kept
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	return fetchByShape(st)
}
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWorkByShape)(fetchByShape(st))
}`
	dir := writeFixture(t, src)
	vs, err := CheckTrivialDelegate(Config{
		Roots:                         []string{dir},
		AllowTrivialDelegateDirective: "custom:allow-delegate",
	})
	require.NoError(t, err)
	require.Empty(t, vs)
}

func TestTrivialDelegate_CrossFileSamePackage(t *testing.T) {
	dir := t.TempDir()
	wrapper := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	return fetchByShape(st)
}`
	pipeline := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWorkByShape)(fetchByShape(st))
}`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "wrapper.go"),
		[]byte(wrapper),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "pipeline.go"),
		[]byte(pipeline),
		0o600,
	))
	vs, err := CheckTrivialDelegate(Config{
		Roots: []string{dir},
	})
	require.NoError(t, err)
	require.Len(t, vs, 1)
	require.Equal(t, fetchWorkByShapeName, vs[0].Function)
}

func TestTrivialDelegate_CheckIntegration(t *testing.T) {
	src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Work int }
func fetchWorkByShape(st State) IOE.IOEither[error, State] {
	return fetchByShape(st)
}
func fetchByShape(st State) IOE.IOEither[error, State] {
	return IOE.Of[error](st)
}
func runPipe(st State) IOE.IOEither[error, State] {
	return IOE.Chain(fetchWorkByShape)(fetchByShape(st))
}`
	dir := writeFixture(t, src)
	// Off by default.
	vs, err := Check(Config{Roots: []string{dir}})
	require.NoError(t, err)
	require.Empty(t, vs)
	// On via Config.
	vs, err = Check(Config{
		Roots:                  []string{dir},
		RequireTrivialDelegate: true,
	})
	require.NoError(t, err)
	require.Len(t, vs, 1)
}
