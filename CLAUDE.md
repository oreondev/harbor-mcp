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

`release.yml` runs the tests and lint, then builds, pushes and signs the image: `ghcr.io/oreondev/harbor-mcp:develop` on every push to `develop`, `X.Y.Z` on a tag. A tag also uploads the binaries from `make dist` with signed checksums and publishes `server.json` to the MCP Registry with `mcp-publisher` and GitHub OIDC. `server.json` must carry the tag's version in `version` and in the image reference, and the Dockerfile label `io.modelcontextprotocol.server.name` must match its `name`; `make validate-server-json` checks the file. The docs site (`mkdocs.yml`, `docs/`, `hack/docs/hooks.py`) is published by `docs.yml` to GitHub Pages and served at https://www.oreonit.com/harbor-mcp/ by a Cloudflare Worker; `docs/tools.md` is generated from `harbor-mcp -list-tools` at build time. `docs/read-only-robot.json` is the documented robot, and the e2e suite runs the server as it, so keep it in step with what the tools read.

## Branches and releases

Work branches (`feat/`, `fix/`, `chore/`, `docs/`) are cut from `develop` and squash-merged into it; `hack/release-bump.sh X.Y.Z` prepares the release PR into `develop` (server.json, README image references, CHANGELOG.md); merging `develop` into `main` with a new server.json version makes `.github/workflows/tag-release.yml` create the `vX.Y.Z` tag and start `release.yml` and `docs.yml` on it. Pushing a tag by hand still works. Dependabot opens a PR into `develop` when harbor-resource-operator releases a new version. User-visible changes get a line under `Unreleased` in `CHANGELOG.md`; the release bump renames it to `X.Y.Z - YYYY-MM-DD`, and `release.yml` fails without that section and uses it as the GitHub release notes. `docs/changelog.md` is copied from it at docs build time.
