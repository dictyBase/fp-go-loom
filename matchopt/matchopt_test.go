package matchopt_test

import (
	"strconv"
	"strings"
	"testing"

	O "github.com/IBM/fp-go/v2/option"
	"github.com/stretchr/testify/require"

	MO "github.com/dictyBase/fp-go-loom/matchopt"
)

// isEven is a reusable predicate atom for the pattern-arm tests.
func isEven(n int) bool { return n%2 == 0 }

// ============================================================================
// Case — conditional arm: Some(f(a)) when pred holds, None otherwise
// ============================================================================

func TestCase(t *testing.T) {
	double := func(n int) int { return n * 2 }

	t.Run(
		"predicate holds returns Some(f(a))",
		func(t *testing.T) {
			arm := MO.Case(isEven, double)
			require.Equal(t, O.Some(4), arm(2))
		},
	)

	t.Run("predicate fails returns None", func(t *testing.T) {
		arm := MO.Case(isEven, double)
		require.Equal(t, O.None[int](), arm(3))
	})

	t.Run("transforms the matched value", func(t *testing.T) {
		arm := MO.Case(
			func(s string) bool { return len(s) >= 3 },
			func(s string) string { return s[:3] },
		)
		require.Equal(t, O.Some("hel"), arm("hello"))
		require.Equal(t, O.None[string](), arm("hi"))
	})
}

// ============================================================================
// Const — conditional arm yielding a constant value
// ============================================================================

func TestConst(t *testing.T) {
	t.Run(
		"predicate holds returns Some(value)",
		func(t *testing.T) {
			arm := MO.Const(isEven, "even")
			require.Equal(t, O.Some("even"), arm(2))
		},
	)

	t.Run("predicate fails returns None", func(t *testing.T) {
		arm := MO.Const(isEven, "even")
		require.Equal(t, O.None[string](), arm(3))
	})

	t.Run(
		"constant never re-evaluates the input",
		func(t *testing.T) {
			arm := MO.Const(
				func(s string) bool { return s == "go" },
				"matches",
			)
			require.Equal(t, O.Some("matches"), arm("go"))
			require.Equal(t, O.None[string](), arm("rust"))
		},
	)
}

// ============================================================================
// Default — catch-all arm that always matches
// ============================================================================

func TestDefault(t *testing.T) {
	t.Run(
		"always returns Some(f(a))",
		func(t *testing.T) {
			arm := MO.Default(func(n int) string {
				return "value: " + strconv.Itoa(n)
			})
			require.Equal(t, O.Some("value: 1"), arm(1))
			require.Equal(t, O.Some("value: 2"), arm(2))
		},
	)

	t.Run(
		"ignores the input when f ignores it",
		func(t *testing.T) {
			arm := MO.Default(
				func(_ string) bool { return true },
			)
			require.Equal(t, O.Some(true), arm("anything"))
		},
	)
}

// ============================================================================
// Alt — first Some wins, None when all cases are None
// ============================================================================

func TestAlt(t *testing.T) {
	t.Run("returns first Some", func(t *testing.T) {
		result := MO.Alt([]O.Option[int]{
			O.None[int](), O.Some(1), O.Some(2),
		})
		require.Equal(t, O.Some(1), result)
	})

	t.Run("all None returns None", func(t *testing.T) {
		result := MO.Alt([]O.Option[string]{
			O.None[string](), O.None[string](),
		})
		require.Equal(t, O.None[string](), result)
	})

	t.Run("empty cases returns None", func(t *testing.T) {
		result := MO.Alt([]O.Option[int]{})
		require.Equal(t, O.None[int](), result)
	})

	t.Run("first case Some short-circuits", func(t *testing.T) {
		result := MO.Alt([]O.Option[string]{
			O.Some("first"), O.Some("second"),
		})
		require.Equal(t, O.Some("first"), result)
	})
}

// ============================================================================
// First — unwraps the first Some, else the fallback
// ============================================================================

func TestFirst(t *testing.T) {
	t.Run("returns value of first Some", func(t *testing.T) {
		result := MO.First("fallback", []O.Option[string]{
			O.None[string](), O.Some("hit"),
		})
		require.Equal(t, "hit", result)
	})

	t.Run("all None returns fallback", func(t *testing.T) {
		result := MO.First("fallback", []O.Option[string]{
			O.None[string](), O.None[string](),
		})
		require.Equal(t, "fallback", result)
	})

	t.Run("empty cases returns fallback", func(t *testing.T) {
		result := MO.First(42, []O.Option[int]{})
		require.Equal(t, 42, result)
	})

	t.Run("works with function values", func(t *testing.T) {
		noop := func(string) string { return "noop" }
		greet := func(s string) string { return "hi " + s }
		handler := MO.First(
			noop,
			[]O.Option[func(string) string]{
				O.None[func(string) string](),
				O.Some(greet),
			},
		)
		require.Equal(t, "hi bob", handler("bob"))
	})
}

// ============================================================================
// Integration — classifier built from matchopt arms, mirroring the
// fp-go-concepts pattern-matching example style
// ============================================================================

func TestClassifierPipeline(t *testing.T) {
	classify := func(n int) string {
		armNeg := MO.Const(
			func(n int) bool { return n < 0 },
			"neg",
		)
		armZero := MO.Const(
			func(n int) bool { return n == 0 },
			"zero",
		)
		armPos := MO.Default(func(int) string { return "pos" })
		return MO.First("unknown", []O.Option[string]{
			armNeg(n), armZero(n), armPos(n),
		})
	}

	t.Run("negative", func(t *testing.T) {
		require.Equal(t, "neg", classify(-5))
	})
	t.Run("zero", func(t *testing.T) {
		require.Equal(t, "zero", classify(0))
	})
	t.Run("positive", func(t *testing.T) {
		require.Equal(t, "pos", classify(7))
	})
}

func TestValidationChain(t *testing.T) {
	// Arms ordered most-specific first; Alt stops at the first Some.
	validate := func(s string) O.Option[string] {
		armEmpty := MO.Const(
			func(s string) bool { return s == "" },
			"empty",
		)
		armLong := MO.Case(
			func(s string) bool { return len(s) > 10 },
			func(s string) string { return "too long: " + s },
		)
		return MO.Alt([]O.Option[string]{
			armEmpty(s), armLong(s),
		})
	}

	t.Run("empty is caught by first arm", func(t *testing.T) {
		require.Equal(t, O.Some("empty"), validate(""))
	})
	t.Run("long is caught by second arm", func(t *testing.T) {
		require.Equal(
			t,
			O.Some("too long: abcdefghijk"),
			validate("abcdefghijk"),
		)
	})
	t.Run("valid input yields None", func(t *testing.T) {
		require.Equal(t, O.None[string](), validate("ok"))
	})
}

// ============================================================================
// Match — lazy first-match-wins dispatcher over unapplied cases
// ============================================================================

func TestMatch(t *testing.T) {
	half := MO.Match(
		"odd",
		[]O.Kleisli[int, string]{
			MO.Case(
				func(n int) bool { return n%2 == 0 },
				func(n int) string { return strconv.Itoa(n / 2) },
			),
			MO.Default(func(n int) string {
				return "odd: " + strconv.Itoa(n)
			}),
		},
	)

	t.Run("first matching case wins", func(t *testing.T) {
		require.Equal(t, "2", half(4))
	})
	t.Run("catch-all arm handles the rest", func(t *testing.T) {
		require.Equal(t, "odd: 3", half(3))
	})
}

func TestMatchSpecificBeforeGeneral(t *testing.T) {
	classify := MO.Match(
		"unknown",
		[]O.Kleisli[string, string]{
			MO.Const(
				func(s string) bool { return strings.HasPrefix(s, "admin/") },
				"admin",
			),
			MO.Const(
				func(s string) bool { return strings.HasPrefix(s, "admin") },
				"other-admin",
			),
		},
	)
	require.Equal(t, "admin", classify("admin/x"))
	require.Equal(t, "other-admin", classify("admin"))
}

func TestMatchFallback(t *testing.T) {
	m := MO.Match(
		"fallback",
		[]O.Kleisli[int, string]{
			MO.Const(
				func(n int) bool { return n < 0 },
				"negative",
			),
		},
	)
	require.Equal(t, "negative", m(-1))
	require.Equal(t, "fallback", m(1))
}

func TestMatchEmptyCases(t *testing.T) {
	m := MO.Match("fallback", []O.Kleisli[int, string]{})
	require.Equal(t, "fallback", m(1))
}

func TestMatchStopsAtFirstMatch(t *testing.T) {
	var calls int
	counting := func(_ int) O.Option[string] {
		calls++
		return O.Some("first")
	}
	never := func(_ int) O.Option[string] {
		t.Error("later case must not run after a match")
		return O.None[string]()
	}
	m := MO.Match("", []O.Kleisli[int, string]{counting, never})
	require.Equal(t, "first", m(1))
	require.Equal(t, 1, calls)
}
