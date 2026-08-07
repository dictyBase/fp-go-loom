package pipelinecheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const wantBareIfMsg = "bare if inside fp-go pipeline closure"

var bareIfCases = []struct {
	name      string
	src       string
	wantCount int
	wantMsg   string
}{
	{
		name: "Pipe callback flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) int {
	return F.Pipe2(s, func(v int) int {
		if v > 0 { return v }
		return 0
	}, func(v int) int { return v })
}`,
		wantCount: 1,
		wantMsg:   wantBareIfMsg,
	},
	{
		name: "Flow callback flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
var pipeline = F.Flow1(func(v int) int {
	if v > 0 { return v }
	return 0
})`,
		wantCount: 1,
		wantMsg:   wantBareIfMsg,
	},
	{
		name: "IOEither Chain callback flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func step(v int) IOE.IOEither[error, int] {
	return IOE.Chain(func(n int) IOE.IOEither[error, int] {
		if n > 0 { return IOE.Of[error](n) }
		return IOE.Of[error](0)
	})(IOE.Of[error](v))
}`,
		wantCount: 1,
		wantMsg:   wantBareIfMsg,
	},
	{
		name: "IOEither Map callback flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func step(v int) IOE.IOEither[error, int] {
	return IOE.Map[error](func(n int) int {
		if n > 0 { return n }
		return 0
	})(IOE.Of[error](v))
}`,
		wantCount: 1,
		wantMsg:   wantBareIfMsg,
	},
	{
		name: "alternate aliases flagged",
		src: `package testpkg
import (
	fn "github.com/IBM/fp-go/v2/function"
	E "github.com/IBM/fp-go/v2/either"
)
func step(v int) E.Either[error, int] {
	return E.Map[error](func(n int) int {
		if n > 0 { return n }
		return 0
	})(E.Of[error](v))
}
var pipeline = fn.Flow1(func(v int) int {
	if v > 0 { return v }
	return 0
})`,
		wantCount: 2,
		wantMsg:   wantBareIfMsg,
	},
	{
		name: "default aliases across supported packages",
		src: `package testpkg
import (
	"github.com/IBM/fp-go/v2/either"
	"github.com/IBM/fp-go/v2/function"
	"github.com/IBM/fp-go/v2/io"
	"github.com/IBM/fp-go/v2/ioeither"
	"github.com/IBM/fp-go/v2/option"
)
var e = either.Map[error](func(n int) int {
	if n > 0 { return n }
	return 0
})
var i = io.Map(func(n int) int {
	if n > 0 { return n }
	return 0
})
var ioe = ioeither.Map[error](func(n int) int {
	if n > 0 { return n }
	return 0
})
var o = option.Map(func(n int) int {
	if n > 0 { return n }
	return 0
})
var pipeline = function.Flow1(func(n int) int {
	if n > 0 { return n }
	return 0
})`,
		wantCount: 5,
		wantMsg:   wantBareIfMsg,
	},
	{
		name: "Map Chain ChainFirst across supported packages",
		src: `package testpkg
import (
	E "github.com/IBM/fp-go/v2/either"
	IO "github.com/IBM/fp-go/v2/io"
	IOE "github.com/IBM/fp-go/v2/ioeither"
	O "github.com/IBM/fp-go/v2/option"
)
var eMap = E.Map[error](func(n int) int {
	if n > 0 { return n }
	return 0
})
var eChain = E.Chain(func(n int) E.Either[error, int] {
	if n > 0 { return E.Of[error](n) }
	return E.Of[error](0)
})
var eChainFirst = E.ChainFirst(func(n int) E.Either[error, int] {
	if n > 0 { return E.Of[error](n) }
	return E.Of[error](0)
})
var ioMap = IO.Map(func(n int) int {
	if n > 0 { return n }
	return 0
})
var ioChain = IO.Chain(func(n int) IO.IO[int] {
	if n > 0 { return IO.Of(n) }
	return IO.Of(0)
})
var ioChainFirst = IO.ChainFirst(func(n int) IO.IO[int] {
	if n > 0 { return IO.Of(n) }
	return IO.Of(0)
})
var ioeMap = IOE.Map[error](func(n int) int {
	if n > 0 { return n }
	return 0
})
var ioeChain = IOE.Chain(func(n int) IOE.IOEither[error, int] {
	if n > 0 { return IOE.Of[error](n) }
	return IOE.Of[error](0)
})
var ioeChainFirst = IOE.ChainFirst(func(n int) IOE.IOEither[error, int] {
	if n > 0 { return IOE.Of[error](n) }
	return IOE.Of[error](0)
})
var oMap = O.Map(func(n int) int {
	if n > 0 { return n }
	return 0
})
var oChain = O.Chain(func(n int) O.Option[int] {
	if n > 0 { return O.Some(n) }
	return O.Some(0)
})
var oChainFirst = O.ChainFirst(func(n int) O.Option[int] {
	if n > 0 { return O.Some(n) }
	return O.Some(0)
})`,
		wantCount: 12,
		wantMsg:   wantBareIfMsg,
	},
	{
		name: "ordinary closure ignored",
		src: `package testpkg
func helper() func(int) int {
	return func(v int) int {
		if v > 0 { return v }
		return 0
	}
}`,
		wantCount: 0,
	},
	{
		name: "top-level function if ignored",
		src: `package testpkg
func helper(v int) int {
	if v > 0 { return v }
	return 0
}`,
		wantCount: 0,
	},
	{
		name: "TryCatch callback ignored",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func step() IOE.IOEither[error, int] {
	return IOE.TryCatchError(func() (int, error) {
		if true { return 1, nil }
		return 0, nil
	})
}`,
		wantCount: 0,
	},
	{
		name: "non fp-go package ignored",
		src: `package testpkg
import F "example.com/other/function"
var pipeline = F.Flow1(func(v int) int {
	if v > 0 { return v }
	return 0
})`,
		wantCount: 0,
	},
	{
		name: "nested if statements each flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
var pipeline = F.Flow1(func(v int) int {
	if v > 0 {
		if v > 1 { return v }
	}
	return 0
})`,
		wantCount: 2,
		wantMsg:   wantBareIfMsg,
	},
	{
		name: "unrelated Pipe-like selector ignored",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
var pipeline = F.Pipeline(func(v int) int {
	if v > 0 { return v }
	return 0
})`,
		wantCount: 0,
	},
	{
		name: "pre-bound Fold callback clean",
		src: `package testpkg
import (
	F "github.com/IBM/fp-go/v2/function"
	P "github.com/IBM/fp-go/v2/predicate"
)
var branch = P.Fold(
	func(v int) int { return 0 },
	func(v int) int { return v },
)
var pipeline = F.Flow1(func(v int) int {
	return branch(v > 0)(v)
})`,
		wantCount: 0,
	},
}

func TestBareIf_Table(t *testing.T) {
	for _, tc := range bareIfCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			vs := checkBareIf(
				fset,
				f,
				DefaultAllowBareIfDirective,
			)
			require.Len(t, vs, tc.wantCount)
			if tc.wantMsg != "" {
				require.Contains(t, vs[0].Message, tc.wantMsg)
			}
		})
	}
}

func TestBareIf_AllowDirective(t *testing.T) {
	src := `package testpkg
import F "github.com/IBM/fp-go/v2/function"
// fp-go:allow-bare-if branch is required for compatibility
func runThing(v int) int {
	return F.Pipe1(v, func(n int) int {
		if n > 0 { return n }
		return 0
	})
}`
	fset, f := parse(t, src)
	vs := checkBareIf(fset, f, DefaultAllowBareIfDirective)
	require.Empty(t, vs)
}

func TestBareIf_CustomDirective(t *testing.T) {
	src := `package testpkg
import F "github.com/IBM/fp-go/v2/function"
// custom:allow-branch legacy behavior
func runThing(v int) int {
	return F.Pipe1(v, func(n int) int {
		if n > 0 { return n }
		return 0
	})
}`
	fset, f := parse(t, src)
	vs := checkBareIf(fset, f, "custom:allow-branch")
	require.Empty(t, vs)
}

func TestBareIf_DirectiveWithoutReason(t *testing.T) {
	src := `package testpkg
import F "github.com/IBM/fp-go/v2/function"
// fp-go:allow-bare-if
func runThing(v int) int {
	return F.Pipe1(v, func(n int) int {
		if n > 0 { return n }
		return 0
	})
}`
	fset, f := parse(t, src)
	vs := checkBareIf(fset, f, DefaultAllowBareIfDirective)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

func TestBareIf_NestedCallbackCheckedIndependently(
	t *testing.T,
) {
	src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func step(v int) IOE.IOEither[error, int] {
	return IOE.Map[error](func(n int) int {
		return n + 1
	})(IOE.Chain(func(n int) IOE.IOEither[error, int] {
		if n > 0 { return IOE.Of[error](n) }
		return IOE.Of[error](0)
	})(IOE.Of[error](v)))
}`
	fset, f := parse(t, src)
	vs := checkBareIf(fset, f, DefaultAllowBareIfDirective)
	require.Len(t, vs, 1)
}

func TestBareIf_PackageScope(t *testing.T) {
	src := `package testpkg
import F "github.com/IBM/fp-go/v2/function"
var _ = F.Flow1(func(v int) int {
	if v > 0 { return v }
	return 0
})`
	fset, f := parse(t, src)
	vs := checkBareIf(fset, f, DefaultAllowBareIfDirective)
	require.Len(t, vs, 1)
	require.Equal(t, "<package>", vs[0].Function)
}

func TestBareIf_CheckIntegration(t *testing.T) {
	src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func step(v int) IOE.IOEither[error, int] {
	return IOE.Chain(func(n int) IOE.IOEither[error, int] {
		if n > 0 { return IOE.Of[error](n) }
		return IOE.Of[error](0)
	})(IOE.Of[error](v))
}`
	fset, f := parse(t, src)
	cfg := withDefaults(Config{RequirePointFreeBranching: true})
	vs := checkFile(fset, f, cfg)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, wantBareIfMsg)
}

func TestBareIf_PublicAPI(t *testing.T) {
	dir := t.TempDir()
	src := `package fixture
import F "github.com/IBM/fp-go/v2/function"
var pipeline = F.Flow1(func(v int) int {
	if v > 0 { return v }
	return 0
})
`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "bad.go"),
		[]byte(src),
		0o600,
	))

	vs, err := CheckBareIf(Config{Roots: []string{dir}})
	require.NoError(t, err)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, wantBareIfMsg)

	entrypointOnly, err := Check(Config{
		Roots:                     []string{dir},
		RequirePointFreeBranching: true,
	})
	require.NoError(t, err)
	require.Len(t, entrypointOnly, 1)
}

func TestRequireBareIfReportsViolations(t *testing.T) {
	dir := writeFixture(t, `package fixture
import F "github.com/IBM/fp-go/v2/function"
var pipeline = F.Flow1(func(v int) int {
	if v > 0 { return v }
	return 0
})`)
	r := &fakeReporter{}
	RequireBareIf(r, Config{Roots: []string{dir}})
	require.Len(t, r.errors, 1)
	require.Contains(t, r.errors[0], wantBareIfMsg)
	require.Empty(t, r.fatal)
}

func TestBareIf_DisabledByDefault(t *testing.T) {
	src := `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(v int) int {
	return F.Pipe1(v, func(n int) int {
		if n > 0 { return n }
		return 0
	})
}`
	fset, f := parse(t, src)
	vs := checkFile(fset, f, withDefaults(Config{}))
	require.Empty(t, vs)
}
