# Harbor MCP server

A [Model Context Protocol](https://modelcontextprotocol.io) server for [Harbor](https://goharbor.io), the open-source container registry. It lets AI assistants such as Claude, Cursor and VS Code answer questions about a running Harbor: projects and quotas, repositories and artifacts, vulnerability scan results, robot accounts, replication health and garbage collection.

This is an independent community project. It is not affiliated with or endorsed by the Harbor project or the CNCF; Harbor is a trademark of The Linux Foundation.

## Status

In development; nothing is released yet. The twelve read tools work and are tested against a fake Harbor and Harbor 2.15; HTTP mode, the image and the registry listing come next. The design is in [docs/DESIGN.md](docs/DESIGN.md).

Planned for the first release:

- Read-only by default: twelve tools covering projects, repositories, artifacts, vulnerabilities, robots, members, registries, replication, webhooks and garbage collection. Write tools (scan, start replication, add a label, create a project) only with an explicit opt-in. No tool deletes anything.
- Secrets stay out of the conversation: Harbor credentials, robot secrets and webhook authorization headers never appear in tool results, logs or errors.
- stdio for desktop clients, and streamable HTTP with a bearer token for running in a cluster.
- A signed multi-arch image, `ghcr.io/oreondev/harbor-mcp`, listed in the official MCP Registry.
- Tested against Harbor 2.12 to 2.15, using the Harbor client of [harbor-resource-operator](https://github.com/oreondev/harbor-resource-operator), the Kubernetes operator that manages Harbor projects, robot accounts and replication as code.

## Security

Report vulnerabilities privately; see [SECURITY.md](SECURITY.md).

## Licence

[Apache-2.0](LICENSE).
