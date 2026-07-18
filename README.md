# fp-go-loom

[![Go Reference](https://pkg.go.dev/badge/github.com/dictyBase/fp-go-loom.svg)](https://pkg.go.dev/github.com/dictyBase/fp-go-loom)
[![Go Report Card](https://goreportcard.com/badge/github.com/dictyBase/fp-go-loom)](https://goreportcard.com/report/github.com/dictyBase/fp-go-loom)
[![CI/CD](https://github.com/dictyBase/fp-go-loom/actions/workflows/ci.yml/badge.svg)](https://github.com/dictyBase/fp-go-loom/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-BSD%202--Clause-blue.svg)](LICENSE)

Reusable [fp-go v2](https://github.com/IBM/fp-go) combinators — predicates,
ord/eq instances, parse helpers, option pattern-matching, and IOEither/Either
bridges. Woven on top of fp-go v2; importable by any Go project.

## Contents

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
└── strutils/
    ├── strutils.go            # JoinStrings
    └── strutils_test.go
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
