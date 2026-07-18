# AGENTS.md

## Commands

```bash
# Test all
gotestsum --format pkgname-and-test-fails --format-hide-empty-pkg -- ./...

# Test single package (run from that dir)
gotestsum --format testdox -- .

# Watch mode
gotestsum --watch --format pkgname-and-test-fails --format-hide-empty-pkg -- ./...

# Lint
golangci-lint run ./...

# Format (gofumpt + goimports + golines)
golangci-lint fmt
```

## Repo structure

- Single-module library: root `go.mod` only.
- Sub-packages under the module root: `array`, `either/parse`, `eithererr`,
  `ioeitherutils`, `matchopt`, `predicate/{ord,array,bytes,strings}`, `strutils`.
- All packages are importable libraries (no `main.go`).

## Coding conventions

- **Line length**: 65 chars max (`golines` with `chain-split-dots: true`).
- **Imports**: three groups (stdlib / external / internal) via `goimports`,
  gci prefix `github.com/dictyBase/fp-go-loom`.
- **Formatters**: `gofumpt` (not `gofmt`).
- **fp-go v2 style**: namespace imports
  (`O "github.com/IBM/fp-go/v2/option"`), `F.Pipe`/`F.Pipe1`/`F.Pipe2`
  composition, generic type aliases.
- Prefer `Option` for nullable fields and `Either` for error paths.
- Test files next to source (e.g. `array.go` + `array_test.go`).
