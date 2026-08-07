package pipelinecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:funlen // table keeps file-join cases readable together.
func TestNoHandRolledFileJoinInArrayMap(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "filepath join in array map",
			src: `package testpkg
import (
	A "github.com/IBM/fp-go/v2/array"
	"path/filepath"
)
func bad(root string) []string {
	return A.Map(func(match string) string {
		return filepath.Join(root, filepath.FromSlash(match))
	})([]string{"x"})
}`,
			want: 1,
		},
		{
			name: "array alias resolves by import path",
			src: `package testpkg
import (
	arr "github.com/IBM/fp-go/v2/array"
	fp "path/filepath"
)
func bad(root string) []string {
	return arr.Map(func(match string) string {
		return fp.Join(root, match)
	})([]string{"x"})
}`,
			want: 1,
		},
		{
			name: "from slash without join is flagged",
			src: `package testpkg
import (
	A "github.com/IBM/fp-go/v2/array"
	fp "path/filepath"
)
func bad() []string {
	return A.Map(func(match string) string {
		return fp.FromSlash(match)
	})([]string{"x"})
}`,
			want: 1,
		},
		{
			name: "ordinary filepath join is clean",
			src: `package testpkg
import "path/filepath"
func good(root, name string) string { return filepath.Join(root, name) }`,
			want: 0,
		},
		{
			name: "similar map package is clean",
			src: `package testpkg
import (
	A "example.com/array"
	"path/filepath"
)
func good(root string) []string {
	return A.Map(func(match string) string {
		return filepath.Join(root, match)
	})([]string{"x"})
}`,
			want: 0,
		},
		{
			name: "canonical file join is clean",
			src: `package testpkg
import (
	A "github.com/IBM/fp-go/v2/array"
	F "github.com/IBM/fp-go/v2/function"
	FILE "github.com/IBM/fp-go/v2/file"
)
func good(root string) []string {
	mapper := F.Pipe1(root, F.Flip(FILE.Join))
	return A.Map(mapper)([]string{"x"})
}`,
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			got := checkNoHandRolledFileJoin(
				fset, f, DefaultAllowHandRolledFileJoinDirective,
			)
			require.Len(t, got, tc.want)
		})
	}
}

func TestNoHandRolledFileJoinDirective(t *testing.T) {
	src := `package testpkg
import (
	A "github.com/IBM/fp-go/v2/array"
	"path/filepath"
)
// fp-go:allow-hand-rolled-file-join compatibility adapter
func good(root string) []string {
	return A.Map(func(match string) string {
		return filepath.Join(root, match)
	})([]string{"x"})
}`
	fset, f := parse(t, src)
	require.Empty(t, checkNoHandRolledFileJoin(
		fset, f, DefaultAllowHandRolledFileJoinDirective,
	))
}

func TestNoHandRolledFileJoinCustomDirective(t *testing.T) {
	src := `package testpkg
import (
	A "github.com/IBM/fp-go/v2/array"
	"path/filepath"
)
// custom:allow-join compatibility adapter
func good(root string) []string {
	return A.Map(func(match string) string {
		return filepath.Join(root, match)
	})([]string{"x"})
}`
	fset, f := parse(t, src)
	require.Empty(t, checkNoHandRolledFileJoin(
		fset, f, "custom:allow-join",
	))
}

func TestNoHandRolledFileJoinPublicCustomDirective(
	t *testing.T,
) {
	dir := writeFixture(t, `package fixture
import (
	A "github.com/IBM/fp-go/v2/array"
	"path/filepath"
)
// custom:allow-join compatibility adapter
func good(root string) []string {
	return A.Map(func(match string) string {
		return filepath.Join(root, match)
	})([]string{"x"})
}`)
	got, err := CheckNoHandRolledFileJoinInArrayMap(Config{
		Roots:                            []string{dir},
		AllowHandRolledFileJoinDirective: "custom:allow-join",
	})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestNoHandRolledFileJoinFixtureFires(t *testing.T) {
	got, err := CheckNoHandRolledFileJoinInArrayMap(Config{
		Roots: []string{"testdata/filejoin"},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestNoHandRolledFileJoinEmptyDirectiveStillReports(
	t *testing.T,
) {
	src := `package testpkg
import (
	A "github.com/IBM/fp-go/v2/array"
	"path/filepath"
)
// fp-go:allow-hand-rolled-file-join
func bad(root string) []string {
	return A.Map(func(match string) string {
		return filepath.Join(root, match)
	})([]string{"x"})
}`
	fset, f := parse(t, src)
	got := checkNoHandRolledFileJoin(
		fset, f, DefaultAllowHandRolledFileJoinDirective,
	)
	require.Len(t, got, 1)
	require.Contains(t, got[0].Message, "non-empty reason")
}

func TestNoHandRolledFileJoinPublicAPI(t *testing.T) {
	dir := writeFixture(t, `package fixture
import (
	A "github.com/IBM/fp-go/v2/array"
	"path/filepath"
)
func bad(root string) []string {
	return A.Map(func(match string) string {
		return filepath.Join(root, match)
	})([]string{"x"})
}`)
	got, err := CheckNoHandRolledFileJoinInArrayMap(
		Config{
			Roots:                            []string{dir},
			AllowHandRolledFileJoinDirective: "custom:allow-join",
		},
	)
	require.NoError(t, err)
	require.Len(t, got, 1)

	r := &fakeReporter{}
	RequireNoHandRolledFileJoinInArrayMap(
		r, Config{Roots: []string{dir}},
	)
	require.Len(t, r.errors, 1)
}
