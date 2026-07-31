# fp-go-loom

[![Go Reference](https://pkg.go.dev/badge/github.com/dictyBase/fp-go-loom.svg)](https://pkg.go.dev/github.com/dictyBase/fp-go-loom)
[![CI/CD](https://github.com/dictyBase/fp-go-loom/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/dictyBase/fp-go-loom/actions/workflows/ci.yml?branch=develop)
[![License](https://img.shields.io/badge/License-BSD%202--Clause-blue.svg)](LICENSE)

Reusable [fp-go v2](https://github.com/IBM/fp-go) combinators — predicates,
ord/eq instances, parse helpers, option pattern-matching, and IOEither/Either
bridges. Woven on top of fp-go v2; importable by any Go project.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Install](#install)
- [Packages](#packages)
- [Usage](#usage)
  - [Arrays](#arrays)
  - [Strings](#strings)
  - [Either & error handling](#either--error-handling)
  - [IOEither bridge](#ioeither-bridge)
  - [Option pattern matching](#option-pattern-matching)
  - [Predicates](#predicates)
  - [Pipeline and style checks](#pipeline-and-style-checks)
- [Pipelines](#pipelines)
  - [Either validation chain](#either-validation-chain)
  - [Parse and range-validate](#parse-and-range-validate)
  - [CSV parse and filter](#csv-parse-and-filter)
  - [Password strength classification](#password-strength-classification)
- [Project Structure](#project-structure)
- [Development](#development)
- [License](#license)

## Prerequisites

- [Go](https://go.dev/) 1.24+
- [gotestsum](https://github.com/gotestyourself/gotestsum) (for test commands)
- [golangci-lint](https://golangci-lint.run/) (for lint/format)

## Install

```bash
go get github.com/dictyBase/fp-go-loom
```

## Packages

| Package | Exports |
|---|---|
| `array` | `Compact`, `ParseWith` |
| `either/parse` | `ParseInt` |
| `eithererr` | `ConstErr` |
| `ioeitherutils` | `ToEither` |
| `matchopt` | `Case`, `Const`, `Default`, `Alt`, `First` |
| `predicate/ord` | `IntOrd`, `Float64Ord`, `IntEq`, `Float64Eq`, `StringEq`, `IntBetween`, `IntBetweenInclusive`, `MinStrLen`, `MaxStrLen`, `StrLenEq`, `NotEqualF64`, `NotEqualInt`, `NotEqualStr`, `StrEq` |
| `predicate/array` | `IsNonEmpty`, `MinLen`, `MaxLen`, `LenEq` |
| `predicate/bytes` | `HasPositiveLen`, `IsNonEmpty` |
| `predicate/strings` | `LastIndexOf`, `HasSuffix`, `ContainsRuneClass`, `HasAtSign`, `StrLenBetween` |
| `strutils` | `JoinStrings` |
| `pipelinecheck` | `Check`, `Require`, `Config`, `Violation`, `Reporter`, `ModuleRoot`, `CheckNoIfErrInTryCatch`, `RequireNoIfErrInTryCatch`, `CheckSafePrintf`, `RequireSafePrintf`, `CheckRedundantFold`, `RequireNoRedundantFold`, `FunctionPkgPath`, `IOEitherPkgPath`, `IOPkgPath`, `EitherPkgPath`, `DefaultAllowDirective`, `DefaultAllowAppliedSeedDirective`, `DefaultAllowTryCatchIfErrDirective`, `DefaultAllowUnsafePrintfDirective`, `DefaultAllowRedundantFoldDirective` |

> **Note:** package names may differ from the import path's last segment
> (e.g. `array` → `arrutils`, `predicate/ord` → `predord`). Alias explicitly
> as shown in the examples below.

## Usage

Each snippet shows the package import and a representative call; outputs
are shown in comments. Full signatures live on
[pkg.go.dev](https://pkg.go.dev/github.com/dictyBase/fp-go-loom).

### Arrays

```go
import (
    "strconv"

    O "github.com/IBM/fp-go/v2/option"
    arrutils "github.com/dictyBase/fp-go-loom/array"
)

// Compact: drop None, keep Some values (catMaybes)
opts := []O.Option[string]{
    O.Some("a"), O.None[string](), O.Some("b"),
}
arrutils.Compact[string](opts) // []string{"a", "b"}

// ParseWith: map []string to []A, silently discarding parse errors
parseInts := arrutils.ParseWith(strconv.Atoi)
parseInts([]string{"1", "bad", "42", "x", "7"}) // []int{1, 42, 7}
```

### Strings

```go
import "github.com/dictyBase/fp-go-loom/strutils"

strutils.JoinStrings("a", "b", "c") // "abc"
strutils.JoinStrings()             // ""
```

### Either & error handling

```go
import (
    E "github.com/IBM/fp-go/v2/either"

    eitherparse "github.com/dictyBase/fp-go-loom/either/parse"
    eithererr "github.com/dictyBase/fp-go-loom/eithererr"
)

// ParseInt: string -> Either[error, int]
eitherparse.ParseInt("42")  // Right[error, int](42)
eitherparse.ParseInt("abc") // Left[error, int]("failed to parse int: ...")
E.IsRight(eitherparse.ParseInt("42")) // true

// ConstErr: stable sentinel error factory. The same errors.New instance is
// returned on every call, so errors.Is stays stable across invocations.
fn := eithererr.ConstErr[string]("invalid")
fn("anything") // error("invalid")
fn("a") == fn("b") // true — identical instance
```

### IOEither bridge

```go
import (
    IOE "github.com/IBM/fp-go/v2/ioeither"

    ioeutils "github.com/dictyBase/fp-go-loom/ioeitherutils"
)

// ToEither: force a lazy IOEither into an eager Either
ioe := IOE.Right[error, int](42)
ioeutils.ToEither[error, int](ioe) // Right[error, int](42)
```

### Option pattern matching

```go
import (
    O "github.com/IBM/fp-go/v2/option"

    MO "github.com/dictyBase/fp-go-loom/matchopt"
)

// Build pattern-match arms over Option
armNeg := MO.Const(func(n int) bool { return n < 0 }, "neg")
armZero := MO.Const(func(n int) bool { return n == 0 }, "zero")
armPos := MO.Default(func(n int) string { return "pos" })

classify := func(n int) string {
    arms := []O.Option[string]{armNeg(n), armZero(n), armPos(n)}
    return MO.First("unknown", arms)
}
classify(-5) // "neg"
classify(0)  // "zero"
classify(7)  // "pos"

// Alt: first Some wins, or None
MO.Alt([]O.Option[int]{
    O.None[int](), O.Some(1), O.Some(2),
}) // Some(1)
```

### Predicates

Ord/eq instances and derived predicates (`predicate/ord`):

```go
import predord "github.com/dictyBase/fp-go-loom/predicate/ord"

predord.MinStrLen(3)("hello")            // true — len(s) >= 3
predord.MaxStrLen(5)("hello")            // true — len(s) <= 5
predord.StrLenEq(3)("hey")               // true — len(s) == 3
predord.IntBetween(1, 10)(5)             // true — 1 <= x < 10 (exclusive)
predord.IntBetween(1, 10)(10)            // false
predord.IntBetweenInclusive(1, 10)(10)  // true — inclusive upper
predord.NotEqualInt(5)(6)                // true — x != v
predord.StrEq("go")("go")                // true — x == s

// Base instances to pass into fp-go combinators:
//   predord.IntOrd, predord.Float64Ord,
//   predord.IntEq, predord.Float64Eq, predord.StringEq
```

Slice-length predicates (`predicate/array`):

```go
import predarrays "github.com/dictyBase/fp-go-loom/predicate/array"

predarrays.IsNonEmpty[int]()([]int{1})                  // true — len > 0
predarrays.IsNonEmpty[int]()([]int{})                    // false
predarrays.MinLen[string](3)([]string{"a", "b", "c"})    // true — len >= 3
predarrays.MaxLen[string](3)([]string{"a", "b"})         // true — len <= 3
predarrays.LenEq[int](2)([]int{1, 2})                    // true — len == 2
```

Byte predicates (`predicate/bytes`):

```go
import predbytes "github.com/dictyBase/fp-go-loom/predicate/bytes"

predbytes.HasPositiveLen([]byte("data")) // true — len > 0
predbytes.HasPositiveLen([]byte{})       // false
predbytes.HasPositiveLen(nil)            // false
predbytes.IsNonEmpty([]byte("x"))        // true — alias for HasPositiveLen
```

String predicates from curried stdlib (`predicate/strings`):

```go
import (
    "unicode"

    predstrings "github.com/dictyBase/fp-go-loom/predicate/strings"
)

predstrings.LastIndexOf("@")("foo@bar")                 // 3
predstrings.HasSuffix(".go")("hello.go")               // true
predstrings.ContainsRuneClass(unicode.IsUpper)("Hello") // true
predstrings.HasAtSign("user@example.com")               // true
predstrings.StrLenBetween(3, 5)("hello")                // true — inclusive
```

### Pipeline and style checks

`pipelinecheck` flags fp-go CLI entrypoints that delegate the whole
continuation to a single-use wrapper via `F.Pipe1(seed, namedFn)` instead
of inlining the steps into one `F.PipeN`. Drop a small `_test.go` into
any package whose `run*`/Action handlers should stay inline:

```go
package cmd

import (
	"strings"
	"testing"

	pipelinecheck "github.com/dictyBase/fp-go-loom/pipelinecheck"
)

func TestPipelineContinuity(t *testing.T) {
	pipelinecheck.Require(t, pipelinecheck.Config{
		Roots: []string{"."},
		IsEntrypoint: func(name string) bool {
			return strings.HasPrefix(name, "run")
		},
	})
}
```

Opt a genuinely-reused wrapper out with a doc comment carrying a
non-empty reason:

```go
// fp-go:allow-pipe1-handoff reused by runA and runB
func runThing(ctx context.Context, c *cli3.Command) error {
	return F.Pipe1(seed(ctx, c), sharedHandler)
}
```

The function-package alias is resolved from each file's imports, so
`F`, `fn`, or the default `function` all work. Analysis is syntax-only
(see package doc for limitations).

#### TryCatch raw-effect rule

`RequireNoIfErrInTryCatch` flags `if err != nil` / `if nil != err` inside
an `IOE.TryCatchError` callback literal. `TryCatchError` should hold the
raw SDK effect; projection and wrapping belong in `IOE.MapLeft` /
`IOE.Map`. The rule applies to **all** functions, not just entrypoints.
Resolve the `IOEither` import alias automatically (`IOE`, `ioeither`,
or a custom alias).

```go
func TestNoIfErrInTryCatch(t *testing.T) {
	pipelinecheck.RequireNoIfErrInTryCatch(t, pipelinecheck.Config{
		Roots: []string{"."},
	})
}
```

Opt out with `// fp-go:allow-trycatch-iferr <reason>`.

#### Safe-Printf rule

`RequireSafePrintf` flags `IO.Printf` calls whose literal or package-local
constant format string has no real formatting verb. A verb-less format
dumps the value via `%!(EXTRA type=value)` and can leak secrets such as
API keys. The rule applies to **all** functions. The printf directive
parser handles flags, `[index]`, width/precision (digits, `*`, indexed
stars), and the documented fmt verbs (`%U`, `%F` included; `%w` excluded
— it is only meaningful to `fmt.Errorf`). Package-local string constants
are resolved across files in the same package, including grouped,
inherited, and concatenated ones.

```go
func TestSafePrintf(t *testing.T) {
	pipelinecheck.RequireSafePrintf(t, pipelinecheck.Config{
		Roots: []string{"."},
	})
}
```

Opt out with `// fp-go:allow-unsafe-printf <reason>`.

#### Redundant-Fold rule

`RequireNoRedundantFold` flags `E.Fold` / `E.Match` calls whose Right
(success) arm discards its argument by returning nil. That shape is
"extract the Left error, return nil on Right" — the long form of
`E.ToError[A]`. Two discard shapes are detected: an anonymous func
whose body is a single `return nil`, and a function-package
`Constant*` combinator wrapping nil or a `T(nil)` conversion. Reserve
`Fold` for the case where both arms produce a real value from their
input (durable rule 10).

```go
func TestNoRedundantFold(t *testing.T) {
	pipelinecheck.RequireNoRedundantFold(
		t, pipelinecheck.Config{Roots: []string{"."}},
	)
}
```

Opt out with `// fp-go:allow-redundant-fold <reason>`.

## Pipelines

The fp-go-concepts examples chain multiple combinators with `F.Pipe` to
build real-world flows. These four pipelines show loom functions composed
across packages. Every snippet was verified to compile and its asserted
output to hold before transcription.

### Either validation chain

Monadic `E.Chain` threads a value through stages; each `E.FromPredicate`
fails fast with a stable sentinel from `eithererr.ConstErr`, so `errors.Is`
keeps identity across calls.

```go
import (
    E "github.com/IBM/fp-go/v2/either"
    F "github.com/IBM/fp-go/v2/function"
    Str "github.com/IBM/fp-go/v2/string"

    eithererr "github.com/dictyBase/fp-go-loom/eithererr"
    predord "github.com/dictyBase/fp-go-loom/predicate/ord"
)

func ValidateUsername(username string) E.Either[error, string] {
    return F.Pipe3(
        username,
        E.FromPredicate(
            Str.IsNonEmpty,
            eithererr.ConstErr[string]("username is required"),
        ),
        E.Chain(E.FromPredicate(
            predord.MinStrLen(3),
            eithererr.ConstErr[string](
                "username must be at least 3 characters",
            ),
        )),
        E.Chain(E.FromPredicate(
            predord.MaxStrLen(20),
            eithererr.ConstErr[string](
                "username must be at most 20 characters",
            ),
        )),
    )
}

ValidateUsername("alice") // Right[error, string]("alice")
ValidateUsername("")      // Left — "username is required"
ValidateUsername("ab")    // Left — "username must be at least 3 characters"
```

### Parse and range-validate

`eitherparse.ParseInt` returns `Either`; `E.Chain` composes it with a
range check built from `predord.IntBetween`. Parse failure and range
failure surface as distinct lefts.

```go
import (
    E "github.com/IBM/fp-go/v2/either"
    F "github.com/IBM/fp-go/v2/function"

    eitherparse "github.com/dictyBase/fp-go-loom/either/parse"
    eithererr "github.com/dictyBase/fp-go-loom/eithererr"
    predord "github.com/dictyBase/fp-go-loom/predicate/ord"
)

func ParsePort(portStr string) E.Either[error, int] {
    return F.Pipe1(
        eitherparse.ParseInt(portStr),
        E.Chain(E.FromPredicate(
            predord.IntBetween(1, 65536),
            eithererr.ConstErr[int](
                "port must be between 1 and 65535",
            ),
        )),
    )
}

ParsePort("8080") // Right[error, int](8080)
ParsePort("abc")  // Left — "failed to parse int: ..."
ParsePort("70000") // Left — "port must be between 1 and 65535"
```

### CSV parse and filter

`A.FilterMap` keeps rows whose split passes `predarrays.LenEq`; each kept
row is mapped into a struct via `O.Map`.

```go
import (
    "strings"

    A "github.com/IBM/fp-go/v2/array"
    F "github.com/IBM/fp-go/v2/function"
    O "github.com/IBM/fp-go/v2/option"

    predarrays "github.com/dictyBase/fp-go-loom/predicate/array"
)

type Customer struct{ ID, Name string }

func ParseCustomers(lines []string) []Customer {
    return F.Pipe1(
        lines,
        A.FilterMap(func(line string) O.Option[Customer] {
            parts := strings.Split(line, ",")
            return F.Pipe2(
                parts,
                O.FromPredicate(predarrays.LenEq[string](2)),
                O.Map(func(p []string) Customer {
                    return Customer{ID: p[0], Name: p[1]}
                }),
            )
        }),
    )
}

ParseCustomers([]string{"1,Alice", "bad", "2,Bob", "3,Carol,Extra"})
// []Customer{{ID:"1", Name:"Alice"}, {ID:"2", Name:"Bob"}}
```

### Password strength classification

`predstrings.ContainsRuneClass` and `predord.MinStrLen`/`MaxStrLen` compose
into strength predicates via `Pred.And`; `matchopt.Const` + `First`
classify. Strong implies medium, so arms run most-specific first.

```go
import (
    "unicode"

    F "github.com/IBM/fp-go/v2/function"
    O "github.com/IBM/fp-go/v2/option"
    Pred "github.com/IBM/fp-go/v2/predicate"

    MO "github.com/dictyBase/fp-go-loom/matchopt"
    predord "github.com/dictyBase/fp-go-loom/predicate/ord"
    predstrings "github.com/dictyBase/fp-go-loom/predicate/strings"
)

func ClassifyPassword(pw string) string {
    hasUpper := predstrings.ContainsRuneClass(unicode.IsUpper)
    hasLower := predstrings.ContainsRuneClass(unicode.IsLower)
    hasDigit := predstrings.ContainsRuneClass(unicode.IsDigit)
    isLongEnough := predord.MinStrLen(8)
    isShort := predord.MaxStrLen(7)

    medium := F.Pipe2(
        isLongEnough, Pred.And(hasUpper), Pred.And(hasLower),
    )
    strong := F.Pipe1(medium, Pred.And(hasDigit))

    armStrong := MO.Const(strong, "strong")
    armMedium := MO.Const(medium, "medium")
    armWeak := MO.Const(isShort, "weak")

    return MO.First("unknown", []O.Option[string]{
        armStrong(pw), armMedium(pw), armWeak(pw),
    })
}

ClassifyPassword("123")         // "weak"
ClassifyPassword("Password")    // "medium"
ClassifyPassword("Password123") // "strong"
```

## Project Structure

```
.
├── array/
│   ├── array.go               # Compact, ParseWith
│   └── array_test.go
├── either/
│   └── parse/
│       ├── parse.go           # ParseInt
│       └── parse_test.go
├── eithererr/
│   ├── errors.go              # ConstErr
│   └── errors_test.go
├── ioeitherutils/
│   └── ioeitherutils.go       # ToEither
├── matchopt/
│   └── matchopt.go            # Case, Const, Default, Alt, First
├── predicate/
│   ├── ord/
│   │   ├── ord.go             # IntOrd, IntBetween, MinStrLen, ...
│   │   └── ord_test.go
│   ├── array/
│   │   ├── array.go           # IsNonEmpty, MinLen, MaxLen, LenEq
│   │   └── array_test.go
│   ├── bytes/
│   │   ├── bytes.go           # HasPositiveLen, IsNonEmpty
│   │   └── bytes_test.go
│   └── strings/
│       ├── strings.go         # LastIndexOf, HasSuffix, HasAtSign, ...
│       └── strings_test.go
├── strutils/
│   ├── strutils.go            # JoinStrings
│   └── strutils_test.go
└── pipelinecheck/
    ├── pipelinecheck.go       # Check, Require, Config, Violation
    ├── trycatch.go            # NoIfErrInTryCatch rule
    ├── printf.go              # SafePrintf rule + const resolution
    ├── pipelinecheck_test.go  # handoff + applied-seed tests
    └── stylecheck_test.go     # trycatch + printf tests
```

## Development

```bash
# Run tests
gotestsum --format pkgname-and-test-fails --format-hide-empty-pkg -- ./...

# Run tests with verbose output
gotestsum --format testdox -- ./...

# Lint
golangci-lint run ./...

# Format (gofumpt + goimports + golines)
golangci-lint fmt
```

## License

BSD-2-Clause. See [LICENSE](LICENSE).
