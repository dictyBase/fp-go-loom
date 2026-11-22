package endoutils_test

import (
	"testing"

	En "github.com/IBM/fp-go/v2/endomorphism"
	F "github.com/IBM/fp-go/v2/function"
	L "github.com/IBM/fp-go/v2/optics/lens"
	"github.com/stretchr/testify/require"

	endoutils "github.com/dictyBase/fp-go-loom/endomorphism"
)

type source struct {
	name string
	port int
}

type config struct {
	Name string
	Port int
	Host string
}

var (
	nameL = L.MakeLens(
		func(c config) string { return c.Name },
		func(c config, v string) config { c.Name = v; return c },
	)
	portL = L.MakeLens(
		func(c config) int { return c.Port },
		func(c config, v int) config { c.Port = v; return c },
	)
	hostL = L.MakeLens(
		func(c config) string { return c.Host },
		func(c config, v string) config { c.Host = v; return c },
	)
	srcNameL = L.MakeLens(
		func(s source) string { return s.name },
		func(s source, v string) source { s.name = v; return s },
	)
	srcPortL = L.MakeLens(
		func(s source) int { return s.port },
		func(s source, v int) source { s.port = v; return s },
	)
	edits = []func(source) En.Endomorphism[config]{
		F.Flow2(srcNameL.Get, nameL.Set),
		F.Flow2(srcPortL.Get, portL.Set),
		F.Flow2(F.Constant1[source]("localhost"), hostL.Set),
	}
)

func TestBuildFromSetsEveryField(t *testing.T) {
	build := endoutils.BuildFrom(edits)
	got := build(
		F.Pipe2(
			source{},
			srcNameL.Set("api"),
			srcPortL.Set(8080),
		),
	)
	require.Equal(t, "api", nameL.Get(got))
	require.Equal(t, 8080, portL.Get(got))
	require.Equal(t, "localhost", hostL.Get(got))
}

func TestBuildFromIsReusable(t *testing.T) {
	build := endoutils.BuildFrom(edits)
	first := build(
		F.Pipe2(source{}, srcNameL.Set("a"), srcPortL.Set(1)),
	)
	second := build(
		F.Pipe2(source{}, srcNameL.Set("b"), srcPortL.Set(2)),
	)
	require.Equal(t, "a", nameL.Get(first))
	require.Equal(t, 1, portL.Get(first))
	require.Equal(t, "b", nameL.Get(second))
	require.Equal(t, 2, portL.Get(second))
}

func TestBuildFromEmptyReturnsZero(t *testing.T) {
	require.Zero(
		t,
		endoutils.BuildFrom[source, config](
			nil,
		)(
			F.Pipe1(source{}, srcNameL.Set("x")),
		),
	)
	require.Zero(
		t,
		endoutils.BuildFrom(
			[]func(source) En.Endomorphism[config]{},
		)(F.Pipe1(source{}, srcNameL.Set("x"))),
	)
}

func TestBuildFromLastWriteWins(t *testing.T) {
	build := endoutils.BuildFrom(
		[]func(source) En.Endomorphism[config]{
			F.Flow2(F.Constant1[source]("first"), nameL.Set),
			F.Flow2(srcNameL.Get, nameL.Set),
		},
	)
	require.Equal(
		t,
		"last",
		nameL.Get(
			build(F.Pipe1(source{}, srcNameL.Set("last"))),
		),
	)
}

func TestBuildFromIsLazy(t *testing.T) {
	calls := 0
	count := func(s source) string {
		calls++
		return srcNameL.Get(s)
	}
	build := endoutils.BuildFrom(
		[]func(source) En.Endomorphism[config]{
			F.Flow2(count, nameL.Set),
		},
	)
	require.Zero(t, calls)
	build(F.Pipe1(source{}, srcNameL.Set("x")))
	require.Equal(t, 1, calls)
}
