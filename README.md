# fp-go-loom

Reusable [fp-go v2](https://github.com/IBM/fp-go) combinators — predicates,
ord/eq instances, parse helpers, option pattern-matching, and IOEither/Either
bridges. Woven on top of fp-go v2; importable by any Go project.

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

## Example

```go
package main

import (
    "fmt"

    predord "github.com/dictyBase/fp-go-loom/predicate/ord"
)

func main() {
    valid := predord.MinStrLen(3)("hello")
    fmt.Println(valid) // true
}
```

## License

BSD-2-Clause. See [LICENSE](LICENSE).
