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
- [Quick Start](#quick-start)
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

| Package | Purpose |
|---|---|
| `array` | `Compact` (catMaybes), `ParseWith` (filterMap parse) |
| `either/parse` | `ParseInt` → `Either[error, int]` |
| `eithererr` | `ConstErr` (stable sentinel error factory) |
| `ioeitherutils` | `ToEither` (lazy IOEither → eager Either) |
| `matchopt` | Option-based pattern-match arms |
| `predicate/ord` | Reusable ord/eq instances + derived predicates |
| `predicate/array` | Slice-length predicates |
| `predicate/bytes` | `[]byte` length predicates |
| `predicate/strings` | String predicates from curried stdlib |
| `strutils` | `JoinStrings` via string monoid fold |

## Quick Start

```go
package main

import (
    "fmt"

    "github.com/dictyBase/fp-go-loom/array"
    predord "github.com/dictyBase/fp-go-loom/predicate/ord"
    O "github.com/IBM/fp-go/v2/option"
)

func main() {
    // Predicate: is the string at least 3 chars?
    fmt.Println(predord.MinStrLen(3)("hello")) // true

    // Compact: drop None, keep Some values
    opts := []O.Option[string]{
        O.Some("a"),
        O.None[string](),
        O.Some("b"),
    }
    fmt.Println(array.Compact[string](opts)) // [a b]
}
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
