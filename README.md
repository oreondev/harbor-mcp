# Harbor MCP server

A [Model Context Protocol](https://modelcontextprotocol.io) server for [Harbor](https://goharbor.io), the open-source container registry. It lets AI assistants such as Claude, Cursor and VS Code answer questions about a running Harbor: projects and quotas, repositories and artifacts, vulnerability scan results, robot accounts, replication health and garbage collection.

This is an independent community project. It is not affiliated with or endorsed by the Harbor project or the CNCF; Harbor is a trademark of The Linux Foundation.

## Status

Version 0.1.0 is the first release. The twelve read tools work over stdio and streamable HTTP, and are tested against a fake Harbor, Harbor 2.15 on every pull request and Harbor 2.12 to 2.15 nightly, as an administrator and as the read-only robot below. The design is in [docs/DESIGN.md](docs/DESIGN.md).

- Read-only: twelve tools covering projects, repositories, artifacts, vulnerabilities, robots, members, registries, replication, webhooks and garbage collection. Write tools (scan, start replication, add a label, create a project) will come later, only with an explicit opt-in. No tool deletes anything.
- Secrets stay out of the conversation: Harbor credentials, robot secrets and webhook authorization headers never appear in tool results, logs or errors.
- stdio for desktop clients, and streamable HTTP with a bearer token for running in a cluster.
- A signed multi-arch image, `ghcr.io/oreondev/harbor-mcp`, listed in the official MCP Registry as `io.github.oreondev/harbor-mcp`.
- Uses the Harbor client of [harbor-resource-operator](https://github.com/oreondev/harbor-resource-operator), the Kubernetes operator that manages Harbor projects, robot accounts and replication as code.

## Tools

| Tool | Answers |
|---|---|
| `harbor_system_info` | Harbor version, authentication mode, and the user or robot the server acts as |
| `harbor_list_projects`, `harbor_get_project` | projects; quota, CVE allowlist, retention and immutability of one |
| `harbor_list_repositories`, `harbor_list_artifacts` | repositories; artifacts with tags, size, push and pull times and scan summary |
| `harbor_get_vulnerabilities` | the vulnerabilities of one artifact, worst first |
| `harbor_list_members`, `harbor_list_robots` | who has access: members with roles, robot accounts with permissions and expiry |
| `harbor_list_registries`, `harbor_list_replications` | replication endpoints and policies, with the last run |
| `harbor_list_webhooks`, `harbor_get_gc` | webhook policies; the garbage collection schedule |

## Set up a read-only Harbor identity

Run the server as a system robot account that can only read. [docs/read-only-robot.json](docs/read-only-robot.json) is the request body for Harbor's API, with the permissions the tools need (the end-to-end tests use the same file):

```sh
curl -u admin -H 'Content-Type: application/json' -d @docs/read-only-robot.json \
  https://harbor.example.com/api/v2.0/robots
```

Harbor shows the secret once, in the response; the user name is `robot$harbor-mcp`. An administrator account also works, but gives the assistant more than it needs.

## Use it

Configuration comes from environment variables; each except `HARBOR_CA_FILE` can instead be read from a file named by `<NAME>_FILE`, for mounted secrets:

| Variable | Meaning |
|---|---|
| `HARBOR_URL` | Harbor base URL, for example `https://harbor.example.com` (required) |
| `HARBOR_USERNAME`, `HARBOR_PASSWORD` | the robot name and secret (required) |
| `HARBOR_CA_FILE` | extra CA bundle (PEM) for Harbor's certificate |
| `HARBOR_INSECURE_SKIP_VERIFY` | `true` to skip TLS verification of Harbor |
| `HARBOR_MCP_RATE` | requests per second to Harbor, default 10 |
| `HARBOR_MCP_HTTP_TOKEN` | bearer token for HTTP mode |

Claude Code, with the image (replace 0.1.0 with the latest release):

```sh
claude mcp add harbor -e HARBOR_URL=https://harbor.example.com -e 'HARBOR_USERNAME=robot$harbor-mcp' -e HARBOR_PASSWORD=... \
  -- docker run -i --rm -e HARBOR_URL -e HARBOR_USERNAME -e HARBOR_PASSWORD ghcr.io/oreondev/harbor-mcp:0.1.0
```

Claude Desktop, Cursor and other clients that read an `mcpServers` file:

```json
{
  "mcpServers": {
    "harbor": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "-e", "HARBOR_URL", "-e", "HARBOR_USERNAME", "-e", "HARBOR_PASSWORD",
               "ghcr.io/oreondev/harbor-mcp:0.1.0"],
      "env": {
        "HARBOR_URL": "https://harbor.example.com",
        "HARBOR_USERNAME": "robot$harbor-mcp",
        "HARBOR_PASSWORD": "..."
      }
    }
  }
}
```

Without Docker, download a binary from the [releases](https://github.com/oreondev/harbor-mcp/releases) (or `go install github.com/oreondev/harbor-mcp/cmd/harbor-mcp@latest`) and use `harbor-mcp` as the command.

### HTTP mode

For a shared server, for example a Deployment next to Harbor, start it with `-http` and a token:

```sh
docker run -d -p 8080:8080 -e HARBOR_URL -e HARBOR_USERNAME -e HARBOR_PASSWORD -e HARBOR_MCP_HTTP_TOKEN \
  ghcr.io/oreondev/harbor-mcp:0.1.0 -http :8080
claude mcp add --transport http harbor https://harbor-mcp.example.com/mcp -H "Authorization: Bearer $HARBOR_MCP_HTTP_TOKEN"
```

MCP is served at `/mcp` and every request needs the token; `GET /healthz` is open for probes. Without a token the server only listens on a loopback address. Put TLS in front of it (an ingress or a mesh).

### Verify the image

```sh
cosign verify ghcr.io/oreondev/harbor-mcp:0.1.0 \
  --certificate-identity-regexp '^https://github.com/oreondev/harbor-mcp/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Maintainers

Maintained by [Oreon IT](https://www.oreonit.com). Issues and pull requests are welcome.

## Security

Report vulnerabilities privately; see [SECURITY.md](SECURITY.md).

## Licence

[Apache-2.0](LICENSE).
