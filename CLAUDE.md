# Harbor MCP server: notes for Claude Code and other coding agents

Read `docs/DESIGN.md` first.

## What this project is

An MCP server for Harbor's REST API v2.0. It reads Harbor objects by default; write tools are opt-in, and no tool deletes anything. It never installs Harbor or moves image contents.

## Conventions

- The Harbor client and the fake Harbor come from `github.com/oreondev/harbor-resource-operator/pkg/harbor` and `pkg/harbor/fake`, pinned to an operator release. New Harbor endpoints are added there first (client, fake and tests), then the pin is bumped here. Do not copy the client.
- Credentials, the HTTP token, robot secrets and webhook `auth_header` values are never logged and never returned in tool results or errors; a test enforces it.
- Every tool: JSON input and output schemas, MCP annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`), paging for lists (default 25, maximum 100).
- Tests: every tool through the SDK's in-memory transport against the fake Harbor; HTTP mode (token, health, cross-origin) through `httptest`.
- Go 1.26; `golangci-lint` at 0 issues; the `sort` package is forbidden, use `slices`.

## CI

`.github/workflows`: unit tests with a tidy check and govulncheck (`test.yml`), golangci-lint (`lint.yml`), and e2e on kind with the official Harbor chart (`test-e2e.yml`: Harbor 2.15 on pushes and pull requests, 2.12 to 2.15 nightly on develop). Actions are pinned by SHA, with the same versions as harbor-resource-operator. `go.mod` pins the toolchain (`toolchain go1.26.10`) so govulncheck sees a patched standard library.

## Branches and releases

Work branches (`feat/`, `fix/`, `chore/`, `docs/`) are cut from `develop` and squash-merged into it; `develop` merges into `main` for a release, and a `vX.Y.Z` tag on `main` publishes it.
