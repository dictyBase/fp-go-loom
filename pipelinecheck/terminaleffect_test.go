package pipelinecheck

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/require"
)

const wantDuplicateMsg = "both fold arms"

const wantRecoveryMsg = "recovery step placed after"

var sendTerminals = []string{"send"}

const terminalFixtureRoot = "testdata/terminaleffect"

// --- Duplicate terminal fold ---

var duplicateFoldCases = []struct {
	name      string
	src       string
	wantCount int
}{
	{
		name: "both arms call terminal directly",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type M struct{}
func send(m M) int { return 0 }
func bad(e E.Either[error, M]) int {
	return E.Fold(
		func(err error) int { return send(M{}) },
		func(m M) int { return send(m) },
	)(e)
}`,
		wantCount: 1,
	},
	{
		name: "both arms reach terminal through a wrapper",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type M struct{}
func send(m M) int { return 0 }
func sendOK(m M) int { return send(m) }
func sendDLQ(err error) int { return send(M{}) }
func bad(e E.Either[error, M]) int {
	return E.Fold(sendDLQ, sendOK)(e)
}`,
		wantCount: 1,
	},
	{
		name: "IOEither Fold alias flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
import IO "github.com/IBM/fp-go/v2/io"
type M struct{}
func send(m M) IO.IO[int] { return nil }
func bad(e IOE.IOEither[error, M]) IO.IO[int] {
	return IOE.Fold(
		func(err error) IO.IO[int] { return send(M{}) },
		func(m M) IO.IO[int] { return send(m) },
	)(e)
}`,
		wantCount: 1,
	},
	{
		name: "Fold in a package without either or ioeither",
		src: `package testpkg
type M struct{}
type folder struct{}
func send(m M) int { return 0 }
func (folder) Fold(a, b func() int) int { return 0 }
func bad(fd folder) int {
	return fd.Fold(
		func() int { return send(M{}) },
		func() int { return send(M{}) },
	)
}`,
		wantCount: 0,
	},
	{
		name: "only one arm reaches terminal",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type M struct{}
func send(m M) int { return 0 }
func bad(e E.Either[error, M]) int {
	return E.Fold(
		func(err error) int { return 0 },
		func(m M) int { return send(m) },
	)(e)
}`,
		wantCount: 0,
	},
	{
		name: "arms produce a description, not an effect",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type M struct{}
func send(m M) int { return 0 }
func good(e E.Either[error, M]) M {
	return E.Fold(
		func(err error) M { return M{} },
		func(m M) M { return m },
	)(e)
}`,
		wantCount: 0,
	},
}

func TestCheckDuplicateTerminalFold(t *testing.T) {
	for _, tc := range duplicateFoldCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			vs := checkDuplicateTerminalFold(
				fset,
				f,
				foldPackageAliases(f),
				newTerminalSet(sendTerminals),
				packageFuncs([]*ast.File{f}),
				DefaultAllowDuplicateTerminalDirective,
			)
			require.Len(t, vs, tc.wantCount)
			if tc.wantCount > 0 {
				require.Contains(
					t, vs[0].Message, wantDuplicateMsg,
				)
			}
		})
	}
}

func TestDuplicateTerminalFold_Directive(t *testing.T) {
	src := `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type M struct{}
func send(m M) int { return 0 }

// fp-go:allow-duplicate-terminal legacy handler, split in #42
func bad(e E.Either[error, M]) int {
	return E.Fold(
		func(err error) int { return send(M{}) },
		func(m M) int { return send(m) },
	)(e)
}`
	fset, f := parse(t, src)
	vs := checkDuplicateTerminalFold(
		fset, f, foldPackageAliases(f),
		newTerminalSet(sendTerminals),
		packageFuncs([]*ast.File{f}),
		DefaultAllowDuplicateTerminalDirective,
	)
	require.Empty(t, vs)
}

func TestDuplicateTerminalFold_DirectiveNeedsReason(
	t *testing.T,
) {
	src := `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type M struct{}
func send(m M) int { return 0 }

// fp-go:allow-duplicate-terminal
func bad(e E.Either[error, M]) int {
	return E.Fold(
		func(err error) int { return send(M{}) },
		func(m M) int { return send(m) },
	)(e)
}`
	fset, f := parse(t, src)
	vs := checkDuplicateTerminalFold(
		fset, f, foldPackageAliases(f),
		newTerminalSet(sendTerminals),
		packageFuncs([]*ast.File{f}),
		DefaultAllowDuplicateTerminalDirective,
	)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

// --- Recovery after terminal ---

var recoveryCases = []struct {
	name      string
	src       string
	wantCount int
}{
	{
		name: "OrElse after a terminal step flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
import IOE "github.com/IBM/fp-go/v2/ioeither"
type M struct{}
type S struct{}
func send(m M) IOE.IOEither[error, int] { return nil }
func recover1(err error) IOE.IOEither[error, int] { return nil }
func bad(s S) IOE.IOEither[error, int] {
	return F.Pipe2(
		IOE.Of[error](M{}),
		IOE.Chain(send),
		IOE.OrElse(recover1),
	)
}`,
		wantCount: 1,
	},
	{
		name: "terminal reached through a wrapper flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
import IOE "github.com/IBM/fp-go/v2/ioeither"
type M struct{}
func send(m M) IOE.IOEither[error, int] { return nil }
func respond(m M) IOE.IOEither[error, int] { return send(m) }
func recover1(err error) IOE.IOEither[error, int] { return nil }
func bad() IOE.IOEither[error, int] {
	return F.Pipe2(
		IOE.Of[error](M{}),
		IOE.Chain(respond),
		IOE.OrElse(recover1),
	)
}`,
		wantCount: 1,
	},
	{
		name: "OrElse before the terminal step is clean",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
import IOE "github.com/IBM/fp-go/v2/ioeither"
type M struct{}
func send(m M) IOE.IOEither[error, int] { return nil }
func encode(m M) IOE.IOEither[error, M] { return nil }
func recover1(err error) IOE.IOEither[error, M] { return nil }
func good() IOE.IOEither[error, int] {
	return F.Pipe3(
		IOE.Of[error](M{}),
		IOE.Chain(encode),
		IOE.OrElse(recover1),
		IOE.Chain(send),
	)
}`,
		wantCount: 0,
	},
	{
		name: "pipe without recovery is clean",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
import IOE "github.com/IBM/fp-go/v2/ioeither"
type M struct{}
func send(m M) IOE.IOEither[error, int] { return nil }
func good() IOE.IOEither[error, int] {
	return F.Pipe1(
		IOE.Of[error](M{}),
		IOE.Chain(send),
	)
}`,
		wantCount: 0,
	},
	{
		name: "recovery arm alone is not a terminal step",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
import IOE "github.com/IBM/fp-go/v2/ioeither"
type M struct{}
func send(m M) IOE.IOEither[error, int] { return nil }
func encode(m M) IOE.IOEither[error, M] { return nil }
func good() IOE.IOEither[error, M] {
	return F.Pipe2(
		IOE.Of[error](M{}),
		IOE.Chain(encode),
		IOE.OrElse(func(err error) IOE.IOEither[error, M] {
			return nil
		}),
	)
}`,
		wantCount: 0,
	},
}

func TestCheckRecoveryAfterTerminal(t *testing.T) {
	for _, tc := range recoveryCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			vs := checkRecoveryAfterTerminal(
				fset,
				f,
				functionAliases(f),
				newTerminalSet(sendTerminals),
				packageFuncs([]*ast.File{f}),
				DefaultAllowRecoveryAfterTerminalDirective,
			)
			require.Len(t, vs, tc.wantCount)
			if tc.wantCount > 0 {
				require.Contains(
					t, vs[0].Message, wantRecoveryMsg,
				)
			}
		})
	}
}

func TestRecoveryAfterTerminal_Directive(t *testing.T) {
	src := `package testpkg
import F "github.com/IBM/fp-go/v2/function"
import IOE "github.com/IBM/fp-go/v2/ioeither"
type M struct{}
func send(m M) IOE.IOEither[error, int] { return nil }
func recover1(err error) IOE.IOEither[error, int] { return nil }

// fp-go:allow-recovery-after-terminal send is idempotent here
func bad() IOE.IOEither[error, int] {
	return F.Pipe2(
		IOE.Of[error](M{}),
		IOE.Chain(send),
		IOE.OrElse(recover1),
	)
}`
	fset, f := parse(t, src)
	vs := checkRecoveryAfterTerminal(
		fset, f, functionAliases(f),
		newTerminalSet(sendTerminals),
		packageFuncs([]*ast.File{f}),
		DefaultAllowRecoveryAfterTerminalDirective,
	)
	require.Empty(t, vs)
}

// --- Terminal set ---

func TestNewTerminalSet_TrimsAndDropsEmpty(t *testing.T) {
	set := newTerminalSet([]string{" send ", "", "  ", "commit"})
	require.Len(t, set, 2)
	require.True(t, set["send"])
	require.True(t, set["commit"])
}

func TestReachesTerminal_StopsAtDepthLimit(t *testing.T) {
	src := `package testpkg
type M struct{}
func send(m M) int { return 0 }
func level3(m M) int { return send(m) }
func level2(m M) int { return level3(m) }
func level1(m M) int { return level2(m) }
func caller(m M) int { return level1(m) }`
	_, f := parse(t, src)
	funcs := packageFuncs([]*ast.File{f})
	require.False(t, stepReachesTerminal(
		funcs["caller"].Body,
		newTerminalSet(sendTerminals),
		funcs,
	))
	require.True(t, stepReachesTerminal(
		funcs["level2"].Body,
		newTerminalSet(sendTerminals),
		funcs,
	))
}

func TestReachesTerminal_SurvivesRecursion(t *testing.T) {
	src := `package testpkg
type M struct{}
func loop(m M) int { return loop(m) }
func caller(m M) int { return loop(m) }`
	_, f := parse(t, src)
	funcs := packageFuncs([]*ast.File{f})
	require.False(t, stepReachesTerminal(
		funcs["caller"].Body,
		newTerminalSet(sendTerminals),
		funcs,
	))
}

// --- Public API ---

func TestTerminalEffect_PublicAPI(t *testing.T) {
	cfg := Config{
		Roots:             []string{terminalFixtureRoot},
		TerminalFunctions: sendTerminals,
	}
	vs, err := CheckTerminalEffect(cfg)
	require.NoError(t, err)
	require.Len(t, vs, 2)

	msgs := []string{vs[0].Message, vs[1].Message}
	require.Contains(t, joinMessages(msgs), wantDuplicateMsg)
	require.Contains(t, joinMessages(msgs), wantRecoveryMsg)
}

func TestTerminalEffect_InertWithoutConfig(t *testing.T) {
	cfg := Config{Roots: []string{terminalFixtureRoot}}
	vs, err := CheckTerminalEffect(cfg)
	require.NoError(t, err)
	require.Empty(t, vs)
}

func TestTerminalEffect_NotRunByCheck(t *testing.T) {
	cfg := Config{
		Roots:             []string{terminalFixtureRoot},
		TerminalFunctions: sendTerminals,
	}
	vs, err := Check(cfg)
	require.NoError(t, err)
	require.Empty(t, vs)
}

func joinMessages(msgs []string) string {
	out := ""
	for _, m := range msgs {
		out += m + "\n"
	}
	return out
}
