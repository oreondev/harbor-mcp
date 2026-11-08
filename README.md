# Harbor MCP server

[![oreondev/harbor-mcp MCP server](https://glama.ai/mcp/servers/oreondev/harbor-mcp/badges/score.svg)](https://glama.ai/mcp/servers/oreondev/harbor-mcp)
[![Listed on mcpservers.org](https://mcpservers.org/badge.svg)](https://mcpservers.org/servers/oreondev/harbor-mcp)

A [Model Context Protocol](https://modelcontextprotocol.io) server for [Harbor](https://goharbor.io), the open-source container registry. It lets AI assistants such as Claude, Cursor and VS Code answer questions about a running Harbor: projects and quotas, repositories and artifacts, vulnerability scan results, robot accounts, replication health and garbage collection.

Documentation: **https://www.oreonit.com/harbor-mcp/**

This is an independent community project. It is not affiliated with or endorsed by the Harbor project or the CNCF; Harbor is a trademark of The Linux Foundation.

## Status

Changes per release are in the [release notes](CHANGELOG.md). The read tools work over stdio and streamable HTTP, and are tested against a fake Harbor, Harbor 2.15 on every pull request and Harbor 2.12 to 2.15 nightly, as an administrator and as the read-only robot below. The design is in [docs/DESIGN.md](docs/DESIGN.md).

- Read-only towards Harbor: fourteen tools covering projects, repositories, artifacts, vulnerabilities, robots, members, registries, replication, webhooks, garbage collection and agent skills stored in Harbor. Write tools (scan, start replication, add a label, create a project) will come later, only with an explicit opt-in. No tool deletes anything in Harbor.
- Opt-in skill install: with `HARBOR_MCP_SKILLS_DIR` set, `harbor_install_skill` writes a skill from Harbor into that directory on your machine, signed skills only by default.
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
| `harbor_list_skills`, `harbor_get_skill` | agent skills stored in a project as OCI artifacts; one skill's `SKILL.md` and files, digest and signature status |
| `harbor_install_skill` | installs a skill into `HARBOR_MCP_SKILLS_DIR` (registered only when that is set) |

Pushing, signing and installing skills, and the extra pull permission reading them needs: [docs/skills.md](docs/skills.md).

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
| `HARBOR_MCP_SKILLS_DIR` | absolute path of a skills directory, for example `/home/me/.claude/skills`; registers `harbor_install_skill`, which installs skills there |
| `HARBOR_MCP_SKILLS_REQUIRE_SIGNED` | `false` to also install skills without a cosign signature; default `true` |

One-click install (the client asks for the URL and credentials, or you fill them in afterwards):

[![Install in VS Code](https://img.shields.io/badge/VS_Code-Install_harbor--mcp-0098FF?logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=harbor&inputs=%5B%7B%22type%22%3A%22promptString%22%2C%22id%22%3A%22harbor-url%22%2C%22description%22%3A%22Harbor%20URL%2C%20for%20example%20https%3A%2F%2Fharbor.example.com%22%7D%2C%7B%22type%22%3A%22promptString%22%2C%22id%22%3A%22harbor-username%22%2C%22description%22%3A%22Harbor%20user%20or%20robot%20name%22%7D%2C%7B%22type%22%3A%22promptString%22%2C%22id%22%3A%22harbor-password%22%2C%22description%22%3A%22Password%20or%20robot%20secret%22%2C%22password%22%3Atrue%7D%5D&config=%7B%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-e%22%2C%22HARBOR_URL%22%2C%22-e%22%2C%22HARBOR_USERNAME%22%2C%22-e%22%2C%22HARBOR_PASSWORD%22%2C%22ghcr.io%2Foreondev%2Fharbor-mcp%3Alatest%22%5D%2C%22env%22%3A%7B%22HARBOR_URL%22%3A%22%24%7Binput%3Aharbor-url%7D%22%2C%22HARBOR_USERNAME%22%3A%22%24%7Binput%3Aharbor-username%7D%22%2C%22HARBOR_PASSWORD%22%3A%22%24%7Binput%3Aharbor-password%7D%22%7D%7D)
[![Add to Cursor](https://img.shields.io/badge/Cursor-Add_harbor--mcp-000000?logo=cursor&logoColor=white)](https://cursor.com/en/install-mcp?name=harbor&config=eyJjb21tYW5kIjoiZG9ja2VyIiwiYXJncyI6WyJydW4iLCItaSIsIi0tcm0iLCItZSIsIkhBUkJPUl9VUkwiLCItZSIsIkhBUkJPUl9VU0VSTkFNRSIsIi1lIiwiSEFSQk9SX1BBU1NXT1JEIiwiZ2hjci5pby9vcmVvbmRldi9oYXJib3ItbWNwOmxhdGVzdCJdLCJlbnYiOnsiSEFSQk9SX1VSTCI6Imh0dHBzOi8vaGFyYm9yLmV4YW1wbGUuY29tIiwiSEFSQk9SX1VTRVJOQU1FIjoicm9ib3QkaGFyYm9yLW1jcCIsIkhBUkJPUl9QQVNTV09SRCI6Ii4uLiJ9fQ%3D%3D)

Claude Code, with the image (replace 0.1.1 with the latest release):

```sh
claude mcp add harbor -e HARBOR_URL=https://harbor.example.com -e 'HARBOR_USERNAME=robot$harbor-mcp' -e HARBOR_PASSWORD=... \
  -- docker run -i --rm -e HARBOR_URL -e HARBOR_USERNAME -e HARBOR_PASSWORD ghcr.io/oreondev/harbor-mcp:0.1.1
```

Claude Desktop, Cursor and other clients that read an `mcpServers` file:

```json
{
  "mcpServers": {
    "harbor": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "-e", "HARBOR_URL", "-e", "HARBOR_USERNAME", "-e", "HARBOR_PASSWORD",
               "ghcr.io/oreondev/harbor-mcp:0.1.1"],
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
  ghcr.io/oreondev/harbor-mcp:0.1.1 -http :8080
claude mcp add --transport http harbor https://harbor-mcp.example.com/mcp -H "Authorization: Bearer $HARBOR_MCP_HTTP_TOKEN"
```

MCP is served at `/mcp` and every request needs the token; `GET /healthz` is open for probes. Without a token the server only listens on a loopback address. Put TLS in front of it (an ingress or a mesh).

### Verify the image

```sh
cosign verify ghcr.io/oreondev/harbor-mcp:0.1.1 \
  --certificate-identity-regexp '^https://github.com/oreondev/harbor-mcp/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Maintainers

Maintained by [Oreon IT](https://www.oreonit.com). Issues and pull requests are welcome.

## Security

Report vulnerabilities privately; see [SECURITY.md](SECURITY.md).

## Licence

[Apache-2.0](LICENSE).
