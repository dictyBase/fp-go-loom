package pipelinecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:funlen // table keeps raw-effect cases readable together.
func TestNoNonRawTryCatchCallback(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "direct raw call is clean",
			src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func fetch() (string, error) { return "", nil }
func good() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) { return fetch() })
}`,
			want: 0,
		},
		{
			name: "fmt error is non raw",
			src: `package testpkg
import (
	"fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
func fetch() (string, error) { return "", nil }
func bad() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		value, err := fetch()
		if err != nil { return "", fmt.Errorf("fetch: %w", err) }
		return value, nil
	})
}`,
			want: 1,
		},
		{
			name: "lens setter is non raw",
			src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Value string }
type Lens struct{}
func (Lens) Set(string) func(State) State { return nil }
func fetch() (string, error) { return "", nil }
func bad(lens Lens, state State) IOE.IOEither[error, State] {
	return IOE.TryCatchError(func() (State, error) {
		_ = lens.Set("value")(state)
		return state, nil
	})
}`,
			want: 1,
		},
		{
			name: "success projection is non raw",
			src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type Response struct{ Name string }
func fetch() (Response, error) { return Response{}, nil }
func bad() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		response, err := fetch()
		return response.Name, err
	})
}`,
			want: 1,
		},
		{
			name: "call projection is non raw",
			src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type Response struct{ Name string }
func fetch() (Response, error) { return Response{}, nil }
func name(response Response) string { return response.Name }
func bad() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		response, err := fetch()
		return name(response), err
	})
}`,
			want: 1,
		},
		{
			name: "raw call with helper argument is clean",
			src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func fetch(value string) (string, error) { return value, nil }
func helper() string { return "value" }
func good() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		return fetch(helper())
	})
}`,
			want: 0,
		},
		{
			name: "custom fmt alias is non raw",
			src: `package testpkg
import (
	f "fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
func fetch() (string, error) { return "", nil }
func bad() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		return "", f.Errorf("fetch: %w", nil)
	})
}`,
			want: 1,
		},
		{
			name: "setter outside callback is clean",
			src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Value string }
type Lens struct{}
func (Lens) Set(string) func(State) State { return nil }
func fetch() (State, error) { return State{}, nil }
func good(lens Lens) IOE.IOEither[error, State] {
	return IOE.Map[error](lens.Set("value"))(IOE.TryCatchError(fetch))
}`,
			want: 0,
		},
		{
			name: "outside callback is clean",
			src: `package testpkg
import (
	"fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
func fetch() (string, error) { return "", nil }
func good() IOE.IOEither[error, string] {
	return IOE.Map[error](func(value string) string {
		return fmt.Sprintf("%s", value)
	})(IOE.TryCatchError(fetch))
}`,
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			got := checkNoNonRawTryCatchCallback(
				fset, f, ioeitherAliases(f),
				DefaultAllowNonRawTryCatchDirective,
			)
			require.Len(t, got, tc.want)
		})
	}
}

func TestNoNonRawTryCatchHeaderSetFalsePositive(t *testing.T) {
	src := `package testpkg
import (
	"net/http"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
func good() IOE.IOEither[error, *http.Response] {
	return IOE.TryCatchError(func() (*http.Response, error) {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		req.Header.Set("Foo", "Bar")
		return http.DefaultClient.Do(req)
	})
}`
	fset, f := parse(t, src)
	got := checkNoNonRawTryCatchCallback(
		fset, f, ioeitherAliases(f),
		DefaultAllowNonRawTryCatchDirective,
	)
	require.Empty(t, got)
}

func TestNoNonRawTryCatchGeneratedLensReceiver(t *testing.T) {
	src := `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
type State struct{ Value string }
type Lens struct{}
type Lenses struct{ Value Lens }
func (Lens) Set(string) func(State) State { return nil }
func bad(lenses Lenses, state State) IOE.IOEither[error, State] {
	return IOE.TryCatchError(func() (State, error) {
		_ = lenses.Value.Set("value")(state)
		return state, nil
	})
}`
	fset, f := parse(t, src)
	got := checkNoNonRawTryCatchCallback(
		fset, f, ioeitherAliases(f),
		DefaultAllowNonRawTryCatchDirective,
	)
	require.Len(t, got, 1)
}

func TestNoNonRawTryCatchDirective(t *testing.T) {
	src := `package testpkg
import (
	"fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
func fetch() (string, error) { return "", nil }
// fp-go:allow-non-raw-trycatch legacy SDK adapter
func good() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		value, err := fetch()
		if err != nil { return "", fmt.Errorf("fetch: %w", err) }
		return value, nil
	})
}`
	fset, f := parse(t, src)
	require.Empty(t, checkNoNonRawTryCatchCallback(
		fset,
		f,
		ioeitherAliases(f),
		DefaultAllowNonRawTryCatchDirective,
	))
}

func TestNoNonRawTryCatchCustomDirective(t *testing.T) {
	src := `package testpkg
import (
	"fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
func fetch() (string, error) { return "", nil }
// custom:allow-raw legacy adapter
func good() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		return "", fmt.Errorf("fetch: %w", nil)
	})
}`
	fset, f := parse(t, src)
	require.Empty(t, checkNoNonRawTryCatchCallback(
		fset, f, ioeitherAliases(f), "custom:allow-raw",
	))
}

func TestNoNonRawTryCatchPublicCustomDirective(t *testing.T) {
	dir := writeFixture(t, `package fixture
import (
	"fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
// custom:allow-raw compatibility adapter
func good() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		return "", fmt.Errorf("fetch: %w", nil)
	})
}`)
	got, err := CheckNoNonRawTryCatchCallback(Config{
		Roots:                        []string{dir},
		AllowNonRawTryCatchDirective: "custom:allow-raw",
	})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestNoNonRawTryCatchFixtureFires(t *testing.T) {
	got, err := CheckNoNonRawTryCatchCallback(Config{
		Roots: []string{"testdata/raweffect"},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestNoNonRawTryCatchOverlapIsDistinct(t *testing.T) {
	src := `package testpkg
import (
	"fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
func fetch() (string, error) { return "", nil }
func bad() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		value, err := fetch()
		if err != nil { return "", fmt.Errorf("fetch: %w", err) }
		return value, nil
	})
}`
	fset, f := parse(t, src)
	ifErr := checkNoIfErrInTryCatch(
		fset, f, ioeitherAliases(f),
		DefaultAllowTryCatchIfErrDirective, nil,
	)
	nonRaw := checkNoNonRawTryCatchCallback(
		fset, f, ioeitherAliases(f),
		DefaultAllowNonRawTryCatchDirective,
	)
	require.Len(t, ifErr, 1)
	require.Contains(t, ifErr[0].Message, "if err != nil")
	require.Len(t, nonRaw, 1)
	require.Contains(t, nonRaw[0].Message, "non-raw work")
}

func TestNoNonRawTryCatchEmptyDirectiveStillReports(
	t *testing.T,
) {
	src := `package testpkg
import (
	"fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
func fetch() (string, error) { return "", nil }
// fp-go:allow-non-raw-trycatch
func bad() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		value, err := fetch()
		if err != nil { return "", fmt.Errorf("fetch: %w", err) }
		return value, nil
	})
}`
	fset, f := parse(t, src)
	got := checkNoNonRawTryCatchCallback(
		fset,
		f,
		ioeitherAliases(f),
		DefaultAllowNonRawTryCatchDirective,
	)
	require.Len(t, got, 1)
	require.Contains(t, got[0].Message, "non-empty reason")
}

func TestNoNonRawTryCatchPublicAPI(t *testing.T) {
	dir := writeFixture(t, `package fixture
import (
	"fmt"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)
func fetch() (string, error) { return "", nil }
func bad() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		value, err := fetch()
		if err != nil { return "", fmt.Errorf("fetch: %w", err) }
		return value, nil
	})
}`)
	got, err := CheckNoNonRawTryCatchCallback(
		Config{Roots: []string{dir}},
	)
	require.NoError(t, err)
	require.Len(t, got, 1)

	r := &fakeReporter{}
	RequireNoNonRawTryCatchCallback(
		r, Config{Roots: []string{dir}},
	)
	require.Len(t, r.errors, 1)
}
