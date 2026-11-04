---
title: Install
description: Run the Harbor MCP server from its signed container image, a release binary or go install, and configure it with environment variables.
---

# Install

=== "Container image"

    ```sh
    docker pull ghcr.io/oreondev/harbor-mcp:{{ version }}
    ```

    Multi-arch (amd64, arm64), distroless, non-root, about 12 MB. Verify the signature:

    ```sh
    cosign verify ghcr.io/oreondev/harbor-mcp:{{ version }} \
      --certificate-identity-regexp '^https://github.com/oreondev/harbor-mcp/\.github/workflows/release\.yml@' \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com
    ```

=== "Binary"

    Download the archive for your platform (Linux, macOS, Windows; amd64 and arm64) from the [releases](https://github.com/oreondev/harbor-mcp/releases/tag/v{{ version }}). `checksums.txt` is signed; verify it with:

    ```sh
    cosign verify-blob --bundle checksums.txt.sigstore.json \
      --certificate-identity-regexp '^https://github.com/oreondev/harbor-mcp/\.github/workflows/release\.yml@' \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
    sha256sum --ignore-missing -c checksums.txt
    ```

=== "go install"

    ```sh
    go install github.com/oreondev/harbor-mcp/cmd/harbor-mcp@v{{ version }}
    ```

## Configuration

Configuration comes from environment variables. Each one except `HARBOR_CA_FILE` can instead be read from a file named by `<NAME>_FILE`, for mounted secrets.

| Variable | Meaning |
|---|---|
| `HARBOR_URL` | Harbor base URL, for example `https://harbor.example.com` (required) |
| `HARBOR_USERNAME`, `HARBOR_PASSWORD` | The account the server acts as; use the [read-only robot](read-only-robot.md) (required) |
| `HARBOR_CA_FILE` | Extra CA bundle (PEM) for Harbor's certificate |
| `HARBOR_INSECURE_SKIP_VERIFY` | `true` to skip TLS verification of Harbor |
| `HARBOR_MCP_RATE` | Requests per second to Harbor, default 10 |
| `HARBOR_MCP_HTTP_TOKEN` | Bearer token for [HTTP mode](http.md) |

## Flags

| Flag | Meaning |
|---|---|
| `-http <addr>` | Serve streamable HTTP on this address instead of stdio |
| `-list-tools` | Print the tool definitions as JSON and exit; needs no Harbor |
| `-version` | Print the version and exit |

## Next

[Connect your AI client](clients.md).
