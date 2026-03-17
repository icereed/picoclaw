# Project Guidelines

PicoClaw is an ultra-lightweight personal AI assistant in Go, targeting minimal hardware (<10 MB RAM). Simplicity and performance are non-negotiable.

## Code Style

### Go Conventions

- Imports in three groups separated by blank lines: stdlib, external, internal (`github.com/sipeed/picoclaw/...`).
- `context.Context` is always the first parameter when present.
- Wrap errors with `fmt.Errorf("doing X: %w", err)` — use a lowercase gerund phrase, no punctuation at the end.
- Define sentinel errors as package-level `var` using `errors.New`, not custom types, unless richer data is needed.
- Constructors follow `NewXxx(...)` pattern. Use functional options only when three or more optional parameters exist.
- Logging via `pkg/logger` (zerolog). Never use `log` or `fmt.Print*` for operational output.

### File Layout

Within a `.go` file, order sections: types/interfaces → constructors → exported methods → unexported methods → helpers.

## Architecture

```
cmd/           CLI entry points (picoclaw, picoclaw-launcher-tui)
pkg/           All library code, one domain per package
  agent/       Agent loop, context, memory, thinking
  channels/    Messaging platform adapters (Telegram, Discord, …)
  providers/   LLM provider abstraction (OpenAI, Anthropic, …)
  session/     Conversation session management
  mcp/         Model Context Protocol integration
  tools/       Built-in tool implementations
  config/      Configuration loading and validation
```

- Interfaces live in the package that *uses* them, not the one that implements them — except shared contracts (e.g. `channels.Channel`).
- Platform-specific code goes in a subpackage (e.g. `pkg/channels/telegram/`).
- `internal/` is used sparingly for implementation details that must not leak (e.g. `pkg/migrate/internal/`).

## Build and Test

```bash
make check    # Full pre-commit: deps + fmt + vet + test
make build    # Build binary (runs go generate first)
make test     # All tests
make fmt      # Format
make lint     # golangci-lint
```

Run `make check` before every commit.

### Testing Standards

- Use stdlib `testing` — not testify for assertions. `t.Errorf` / `t.Fatalf` only.
- Prefer table-driven tests with `t.Run(name, ...)` subtests.
- Name test functions `TestXxx` or `TestXxx_Scenario` — use underscores for scenario suffixes.
- Mark helpers with `t.Helper()`.
- Use `t.TempDir()` for file-system tests — never hard-code paths.
- Mocks are hand-written structs, not generated or framework-based.
- Test the public API of a package. Use `_test` package suffix for black-box tests when testing exported behaviour.

## Conventions

- Commit messages: imperative mood, reference issues (`Fix session leak (#123)`). See [Conventional Commits](https://www.conventionalcommits.org/).
- Branch names: `fix/telegram-timeout`, `feat/ollama-provider`, `docs/contributing-guide`.
- Never use `panic` outside of `init()` or truly unrecoverable programmer errors.
- Keep allocations minimal — this runs on $10 boards. Prefer reuse over convenience.
