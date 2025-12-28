# Repository Guidelines

## Project Structure & Module Organization
- Root: Go module `github.com/dadrian/relish` with `SPEC.md` and docs and importable library code (public API: Marshal/Unmarshal, Encoder/Decoder).
- Internals: single package `internal/` split by files (e.g., `lengths.go`, `binary.go`, `reflectx.go`, `buffers.go`). Keep shared helpers here, unexported.
- Tests: `*_test.go` next to code; golden vectors in `testdata/`.
- Tools: tools/ folder. Ignore this for now.

## Build, Test, and Development Commands
- Build: `go build ./...` — verify package compiles.
- Tests: `go test ./...` — run unit tests.
- Race: `go test -race ./...` — detect data races.
- Fuzz (where applicable): `go test -fuzz=Fuzz -fuzztime=30s ./...`.
- Vet: `go vet ./...` — static checks for common issues.
- Format: `gofmt -s -w .` (or `go fmt ./...`).

## Coding Style & Naming Conventions
- Formatting: gofmt/goimports; no manual bikeshedding. One file per type/area.
- Naming: exported `CamelCase`, unexported `lowerCamel`. Package names short, lower.
- Errors: return wrapped errors (`fmt.Errorf("context: %w", err)`); define sentinel vars when useful.
- Context: accept `context.Context` in public APIs only when it affects I/O.
- Docs: add Godoc comments to exported identifiers; reference `SPEC.md` sections when relevant.

## Testing Guidelines
- Tests live with code in `*_test.go`. Prefer table-driven tests.
- Add fuzzers for decoders (e.g., `FuzzDecode_*`) to guard against panics.
- Golden cases for TLV examples in `relish/testdata/`; round-trip and error-path tests.
- Avoid network and time-dependent behavior; keep tests deterministic and fast.

## Commit & Pull Request Guidelines
- Commits: concise, imperative subject (e.g., "parser: enforce field order").
- Scope small; separate refactors from behavior changes.
- PRs: include context, rationale, and links to `SPEC.md` sections affected.
- Require tests for new behavior and bug fixes; update docs/examples when APIs change.
- Screenshots/logs only when clarifying behavior; otherwise favor minimal diffs.

## Security & Robustness Notes
- Decoder must validate all lengths, IDs, and UTF-8; never panic on bad input.
- Enforce reasonable limits (length, nesting) and prefer streaming over large allocs.
- Treat unknown struct fields as ignorable per spec; do not reject them.
