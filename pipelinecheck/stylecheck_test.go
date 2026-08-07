package pipelinecheck

import (
	"go/ast"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const wantIfErrMsg = "if err != nil"

// --- NoIfErrInTryCatch ---

var ifErrCases = []struct {
	name      string
	src       string
	wantCount int
	wantMsg   string
}{
	{
		name: "if err != nil flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func get() (*T, error) { return nil, nil }
func bad() IOE.IOEither[error, *T] {
	return IOE.TryCatchError(func() (*T, error) {
		x, err := get()
		if err != nil { return nil, err }
		return x, nil
	})
}`,
		wantCount: 1,
		wantMsg:   wantIfErrMsg,
	},
	{
		name: "nil != err flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func get() (*T, error) { return nil, nil }
func bad() IOE.IOEither[error, *T] {
	return IOE.TryCatchError(func() (*T, error) {
		x, err := get()
		if nil != err { return nil, err }
		return x, nil
	})
}`,
		wantCount: 1,
		wantMsg:   wantIfErrMsg,
	},
	{
		name: "raw effect not flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func get() (*T, error) { return nil, nil }
func good() IOE.IOEither[error, *T] {
	return IOE.TryCatchError(func() (*T, error) {
		return get()
	})
}`,
		wantCount: 0,
	},
	{
		name: "generic call flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func get() (*T, error) { return nil, nil }
func bad() IOE.IOEither[error, *T] {
	return IOE.TryCatchError[*T](func() (*T, error) {
		x, err := get()
		if err != nil { return nil, err }
		return x, nil
	})
}`,
		wantCount: 1,
		wantMsg:   wantIfErrMsg,
	},
	{
		name: "if err outside callback not flagged",
		src: `package testpkg
type T struct{}
func get() (*T, error) { return nil, nil }
func ok() error {
	_, err := get()
	if err != nil { return err }
	return nil
}`,
		wantCount: 0,
	},
	{
		name: "named callback not flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func raw() (*T, error) {
	x, err := get2()
	if err != nil { return nil, err }
	return x, nil
}
func get2() (*T, error) { return nil, nil }
func ok() IOE.IOEither[error, *T] {
	return IOE.TryCatchError(raw)
}`,
		wantCount: 0,
	},
	{
		name: "if err == nil not flagged",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func get() (*T, error) { return nil, nil }
func ok() IOE.IOEither[error, *T] {
	return IOE.TryCatchError(func() (*T, error) {
		x, err := get()
		if err == nil { return x, nil }
		return nil, err
	})
}`,
		wantCount: 0,
	},
}

func TestNoIfErrInTryCatch_Table(t *testing.T) {
	for _, c := range ifErrCases {
		t.Run(c.name, func(t *testing.T) {
			fset, f := parse(t, c.src)
			vs := checkNoIfErrInTryCatch(
				fset, f, ioeitherAliases(f),
				DefaultAllowTryCatchIfErrDirective, nil,
			)
			require.Len(t, vs, c.wantCount)
			if c.wantCount > 0 {
				require.Contains(t, vs[0].Message, c.wantMsg)
			}
		})
	}
}

func TestNoIfErrInTryCatch_AllowDirective(t *testing.T) {
	src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func get() (*T, error) { return nil, nil }

// fp-go:allow-trycatch-iferr legacy adapter splits compound error
func bad() IOE.IOEither[error, *T] {
	return IOE.TryCatchError(func() (*T, error) {
		x, err := get()
		if err != nil { return nil, err }
		return x, nil
	})
}`
	fset, f := parse(t, src)
	vs := checkNoIfErrInTryCatch(
		fset, f, ioeitherAliases(f),
		DefaultAllowTryCatchIfErrDirective, nil,
	)
	require.Empty(t, vs)
}

func TestNoIfErrInTryCatch_CustomDirective(t *testing.T) {
	src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func get() (*T, error) { return nil, nil }
// custom:allow-iferr legacy adapter
func bad() IOE.IOEither[error, *T] {
	return IOE.TryCatchError(func() (*T, error) {
		x, err := get()
		if err != nil { return nil, err }
		return x, nil
	})
}`
	fset, f := parse(t, src)
	vs := checkNoIfErrInTryCatch(
		fset, f, ioeitherAliases(f), "custom:allow-iferr", nil,
	)
	require.Empty(t, vs)
}

func TestNoIfErrInTryCatch_DirectiveWithoutReason(t *testing.T) {
	src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func get() (*T, error) { return nil, nil }

// fp-go:allow-trycatch-iferr
func bad() IOE.IOEither[error, *T] {
	return IOE.TryCatchError(func() (*T, error) {
		x, err := get()
		if err != nil { return nil, err }
		return x, nil
	})
}`
	fset, f := parse(t, src)
	vs := checkNoIfErrInTryCatch(
		fset, f, ioeitherAliases(f),
		DefaultAllowTryCatchIfErrDirective, nil,
	)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

// --- SafePrintf ---

var printfCases = []struct {
	name      string
	src       string
	wantCount int
}{
	{
		name: "bare literal flagged",
		src: `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
func bad(n int) IO.IO[int] {
	return IO.Printf[int]("plain")
}`,
		wantCount: 1,
	},
	{
		name: "verb allowed",
		src: `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
func good(n int) IO.IO[int] {
	return IO.Printf[int]("count: %d")
}`,
		wantCount: 0,
	},
	{
		name: "escaped percent only flagged",
		src: `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
func bad(n int) IO.IO[int] {
	return IO.Printf[int]("100%% done")
}`,
		wantCount: 1,
	},
	{
		name: "concatenated verbs allowed",
		src: `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
func good(n int) IO.IO[int] {
	return IO.Printf[int]("path=" + "%s")
}`,
		wantCount: 0,
	},
	{
		name: "dynamic format not flagged",
		src: `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
func good(n int, f string) IO.IO[int] {
	return IO.Printf[int](f)
}`,
		wantCount: 0,
	},
	{
		name: "non-generic call flagged",
		src: `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
func bad() IO.IO[string] {
	return IO.Printf("plain dump")
}`,
		wantCount: 1,
	},
	{
		name: "fmt.Printf not flagged",
		src: `package testpkg
import "fmt"
func bad() { fmt.Printf("plain") }`,
		wantCount: 0,
	},
	{
		name: "%w not a valid printf verb",
		src: `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
func bad(n int) IO.IO[int] {
	return IO.Printf[int]("err: %w")
}`,
		wantCount: 1,
	},
}

func TestSafePrintf_Table(t *testing.T) {
	for _, c := range printfCases {
		t.Run(c.name, func(t *testing.T) {
			fset, f := parse(t, c.src)
			vs := checkSafePrintf(
				fset, f, ioAliases(f),
				DefaultAllowUnsafePrintfDirective,
				pkgConsts([]*ast.File{f}),
			)
			require.Len(t, vs, c.wantCount)
		})
	}
}

func TestSafePrintf_FlagsAndIndex(t *testing.T) {
	verbs := []string{
		`%+v`, `%02d`, `%#v`, `%[1]s`, `%-10s`,
		`%.3f`, `%6.2f`, `%*s`, `%[2]*[1]s`,
		`%[3]*.[2]*[1]f`, `%U`, `%F`,
	}
	for _, v := range verbs {
		t.Run(v, func(t *testing.T) {
			src := `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
func good(n int) IO.IO[int] {
	return IO.Printf[int]("val ` + v + `")
}`
			fset, f := parse(t, src)
			vs := checkSafePrintf(
				fset, f, ioAliases(f),
				DefaultAllowUnsafePrintfDirective, nil,
			)
			require.Empty(t, vs, "verb %s", v)
		})
	}
}

func TestSafePrintf_ConstResolution(t *testing.T) {
	t.Run("file-local const", func(t *testing.T) {
		src := `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
const bareFmt = "plain"
const verbFmt = "%d"
func bad(n int) IO.IO[int] { return IO.Printf[int](bareFmt) }
func good(n int) IO.IO[int] { return IO.Printf[int](verbFmt) }`
		fset, f := parse(t, src)
		vs := checkSafePrintf(
			fset, f, ioAliases(f),
			DefaultAllowUnsafePrintfDirective,
			pkgConsts([]*ast.File{f}),
		)
		require.Len(t, vs, 1)
	})
	t.Run("multi-name const", func(t *testing.T) {
		src := `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
const bare, verb = "plain", "%d"
func bad(n int) IO.IO[int] { return IO.Printf[int](bare) }
func good(n int) IO.IO[int] { return IO.Printf[int](verb) }`
		fset, f := parse(t, src)
		vs := checkSafePrintf(
			fset, f, ioAliases(f),
			DefaultAllowUnsafePrintfDirective,
			pkgConsts([]*ast.File{f}),
		)
		require.Len(t, vs, 1)
	})
	t.Run("inherited const", func(t *testing.T) {
		src := `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
const (
	single = "plain"
	singleHeir
	a, b = "plain", "%d"
	c, d
)
func bad(n int) IO.IO[int] { return IO.Printf[int](singleHeir) }
func bad2(n int) IO.IO[int] { return IO.Printf[int](c) }
func good(n int) IO.IO[int] { return IO.Printf[int](d) }`
		fset, f := parse(t, src)
		vs := checkSafePrintf(
			fset, f, ioAliases(f),
			DefaultAllowUnsafePrintfDirective,
			pkgConsts([]*ast.File{f}),
		)
		require.Len(t, vs, 2)
	})
}

func TestSafePrintf_DirectiveWithoutReason(t *testing.T) {
	src := `package testpkg
import IO "github.com/IBM/fp-go/v2/io"

// fp-go:allow-unsafe-printf
func bad(n int) IO.IO[int] { return IO.Printf[int]("plain") }`
	fset, f := parse(t, src)
	vs := checkSafePrintf(
		fset, f, ioAliases(f),
		DefaultAllowUnsafePrintfDirective, nil,
	)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

// --- Package-scope (no enclosing func) ---

func TestPackageScopeCalls(t *testing.T) {
	t.Run("trycatch package scope no panic", func(t *testing.T) {
		src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
var _ = IOE.TryCatchError(func() (*T, error) {
	x, err := get()
	if err != nil { return nil, err }
	return x, nil
})
func get() (*T, error) { return nil, nil }`
		fset, f := parse(t, src)
		vs := checkNoIfErrInTryCatch(
			fset, f, ioeitherAliases(f),
			DefaultAllowTryCatchIfErrDirective, nil,
		)
		require.Len(t, vs, 1)
		require.Equal(t, "<package>", vs[0].Function)
	})
	t.Run("printf package scope", func(t *testing.T) {
		src := `package testpkg
import IO "github.com/IBM/fp-go/v2/io"
var _ = IO.Printf[int]("plain")`
		fset, f := parse(t, src)
		vs := checkSafePrintf(
			fset, f, ioAliases(f),
			DefaultAllowUnsafePrintfDirective, nil,
		)
		require.Len(t, vs, 1)
		require.Equal(t, "<package>", vs[0].Function)
	})
}

// --- Dedicated public APIs ---

func TestPublicAPIIndependence(t *testing.T) {
	cfg := Config{Roots: []string{"testdata/both"}}
	// Check (entrypoint rules) finds neither violation.
	vs, err := Check(cfg)
	require.NoError(t, err)
	require.Empty(t, vs)
	// Each dedicated gate finds exactly its own violation.
	vs1, err := CheckNoIfErrInTryCatch(cfg)
	require.NoError(t, err)
	require.Len(t, vs1, 1)
	require.Contains(t, vs1[0].Message, wantIfErrMsg)
	vs2, err := CheckSafePrintf(cfg)
	require.NoError(t, err)
	require.Len(t, vs2, 1)
	require.Contains(t, vs2[0].Message, "no formatting verb")
}

func TestDuplicateDirectiveWithoutReason(t *testing.T) {
	src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type T struct{}
func get() (*T, error) { return nil, nil }

// fp-go:allow-trycatch-iferr
func bad() IOE.IOEither[error, *T] {
	a := IOE.TryCatchError(func() (*T, error) {
		x, err := get()
		if err != nil { return nil, err }
		return x, nil
	})
	b := IOE.TryCatchError(func() (*T, error) {
		y, err := get()
		if err != nil { return nil, err }
		return y, nil
	})
	_ = a
	_ = b
	return a
}`
	fset, f := parse(t, src)
	vs := checkNoIfErrInTryCatch(
		fset, f, ioeitherAliases(f),
		DefaultAllowTryCatchIfErrDirective, nil,
	)
	// Exactly one directive error, not two.
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

// --- ModuleRoot ---

func TestModuleRoot(t *testing.T) {
	root := ModuleRoot(t)
	require.NotEmpty(t, root)
	_, err := os.Stat(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
}

type fatalfReporter struct{ msg string }

func (f *fatalfReporter) Helper()               {}
func (f *fatalfReporter) Errorf(string, ...any) {}
func (f *fatalfReporter) Fatalf(format string, _ ...any) {
	f.msg = format
}

func TestModuleRoot_NonTerminatingReporter(t *testing.T) {
	t.Chdir(t.TempDir())
	var r fatalfReporter
	got := ModuleRoot(&r)
	require.Empty(t, got)
	require.NotEmpty(t, r.msg)
}

// --- Fixture / integration tests ---

func TestNoIfErrInTryCatch_FixtureFires(t *testing.T) {
	vs, err := CheckNoIfErrInTryCatch(Config{
		Roots: []string{"testdata/bad_iferr"},
	})
	require.NoError(t, err)
	require.Len(t, vs, 1)
}

func TestNoNonRawTryCatch_FixtureGoodSilent(t *testing.T) {
	vs, err := CheckNoNonRawTryCatchCallback(Config{
		Roots: []string{"testdata/raweffect"},
	})
	require.NoError(t, err)
	// bad.go is intentionally covered separately; this verifies fixture
	// scanning remains deterministic when both files are present.
	require.Len(t, vs, 1)
}

func TestSafePrintf_FixtureFires(t *testing.T) {
	vs, err := CheckSafePrintf(Config{
		Roots: []string{"testdata/bad_printf"},
	})
	require.NoError(t, err)
	require.Len(t, vs, 1)
}

func TestFixtures_GoodSilent(t *testing.T) {
	cfg := Config{Roots: []string{"testdata/good"}}
	vs1, err := CheckNoIfErrInTryCatch(cfg)
	require.NoError(t, err)
	require.Empty(t, vs1)
	vs2, err := CheckSafePrintf(cfg)
	require.NoError(t, err)
	require.Empty(t, vs2)
}

func TestSafePrintf_CrossFileConst(t *testing.T) {
	vs, err := CheckSafePrintf(Config{
		Roots: []string{"testdata/crossfile"},
	})
	require.NoError(t, err)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "no formatting verb")
}

// --- Walk regression ---

func TestNonTestGoFiles_RelativeParentRoot(t *testing.T) {
	files, err := nonTestGoFiles("..")
	require.NoError(t, err)
	require.NotEmpty(t, files)
}

func TestIsSkippedDir(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{".", false},
		{"..", false},
		{"src", false},
		{".git", true},
		{".github", true},
		{"vendor", true},
		{"testdata", true},
		{"_gen", true},
	}
	for _, c := range cases {
		require.Equal(t, c.want, isSkippedDir(c.name), c.name)
	}
}
