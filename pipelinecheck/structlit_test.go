package pipelinecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	wantStructLitMsg = "populated struct literal"

	structLitPreamble = `package testpkg
import (
	F "github.com/IBM/fp-go/v2/function"
	L "github.com/IBM/fp-go/v2/optics/lens"
)
type Person struct {
	Name string
	Age  int
}
type Box[T any] struct {
	Value T
}
var nameLens = L.MakeLens(
	func(p Person) string { return p.Name },
	func(p Person, v string) Person { p.Name = v; return p },
)
var ageLens = L.MakeLens(
	func(p Person) int { return p.Age },
	func(p Person, v int) Person { p.Age = v; return p },
)
`
)

var structLitCases = []struct {
	name      string
	body      string
	wantCount int
}{
	{
		name: "populated struct literal flagged",
		body: `func bad() Person {
	return Person{Name: "Alice", Age: 30}
}`,
		wantCount: 1,
	},
	{
		name: "pointer to populated struct literal flagged",
		body: `func bad() *Person {
	return &Person{Name: "Alice"}
}`,
		wantCount: 1,
	},
	{
		name: "generic populated struct literal flagged",
		body: `func bad() Box[string] {
	return Box[string]{Value: "hi"}
}`,
		wantCount: 1,
	},
	{
		name: "elided struct literal in slice flagged",
		body: `func bad() []Person {
	return []Person{{Name: "Alice"}, {Name: "Bob"}}
}`,
		wantCount: 2,
	},
	{
		name: "anonymous struct literal flagged",
		body: `func bad() any {
	return struct{ Name string }{"Alice"}
}`,
		wantCount: 1,
	},
	{
		name:      "package-scope populated struct literal flagged",
		body:      `var defaultPerson = Person{Name: "Alice"}`,
		wantCount: 1,
	},
	{
		name: "empty zero struct with lens.Set in Pipe clean",
		body: `func good() Person {
	return F.Pipe2(
		Person{},
		nameLens.Set("Alice"),
		ageLens.Set(30),
	)
}`,
		wantCount: 0,
	},
	{
		name: "slice and map literals clean",
		body: `func good() map[string][]string {
	return map[string][]string{
		"a": {"x", "y"},
	}
}`,
		wantCount: 0,
	},
}

func TestPopulatedStruct_Table(t *testing.T) {
	for _, tc := range structLitCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, structLitPreamble+tc.body)
			vs := checkNoPopulatedStruct(
				fset,
				f,
				DefaultAllowPopulatedStructDirective,
			)
			require.Len(t, vs, tc.wantCount)
			if tc.wantCount > 0 {
				require.Contains(
					t,
					vs[0].Message,
					wantStructLitMsg,
				)
			}
		})
	}
}

func TestPopulatedStruct_FileWithoutFpGoIgnored(t *testing.T) {
	fset, f := parse(t, `package testpkg
type Person struct { Name string }
func f() Person { return Person{Name: "Alice"} }`)
	require.Empty(t, checkNoPopulatedStruct(
		fset,
		f,
		DefaultAllowPopulatedStructDirective,
	))
}

func TestPopulatedStruct_AllowDirective(t *testing.T) {
	fset, f := parse(t, structLitPreamble+`
// fp-go:allow-struct-literal external SDK config struct
func ok() Person {
	return Person{Name: "Alice"}
}`)
	require.Empty(t, checkNoPopulatedStruct(
		fset,
		f,
		DefaultAllowPopulatedStructDirective,
	))
}

func TestPopulatedStruct_DirectiveWithoutReason(t *testing.T) {
	fset, f := parse(t, structLitPreamble+`
// fp-go:allow-struct-literal
func ok() Person {
	return Person{Name: "Alice"}
}`)
	vs := checkNoPopulatedStruct(
		fset,
		f,
		DefaultAllowPopulatedStructDirective,
	)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

func TestPopulatedStruct_CheckAndRequireOverRoots(t *testing.T) {
	dir := writeFixture(t, structLitPreamble+`
func bad() Person {
	return Person{Name: "Alice"}
}`)
	cfg := Config{Roots: []string{dir}}

	vs, err := CheckNoPopulatedStruct(cfg)
	require.NoError(t, err)
	require.Len(t, vs, 1)

	rec := &fakeReporter{}
	RequireNoPopulatedStruct(rec, cfg)
	require.Len(t, rec.errors, 1)
	require.Contains(t, rec.errors[0], wantStructLitMsg)
}

func TestPopulatedStruct_ScanErrorIsFatal(t *testing.T) {
	rec := &fakeReporter{}
	RequireNoPopulatedStruct(rec, Config{Roots: []string{
		t.TempDir() + "/missing",
	}})
	require.Contains(t, rec.fatal, "pipelinecheck:")
}
