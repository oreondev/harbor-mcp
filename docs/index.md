---
title: Harbor MCP server
description: A read-only Model Context Protocol server that lets Claude, Cursor, VS Code and other AI assistants answer questions about a Harbor container registry.
---

# Harbor MCP server

A [Model Context Protocol](https://modelcontextprotocol.io) server for [Harbor](https://goharbor.io), the open-source container registry. It lets AI assistants such as Claude, Cursor and VS Code answer questions about a running Harbor: projects and quotas, repositories and artifacts, vulnerability scan results, robot accounts, replication health and garbage collection.

Ask things like:

- *Which artifacts in `payments` have critical vulnerabilities, and when were they last pulled?*
- *Which robot accounts expire in the next 30 days, and what can they push?*
- *Is replication to the edge registry healthy? When did it last succeed?*
- *Which projects are close to their storage quota?*

<div class="grid cards" markdown>

- **[Install](install.md)**: container image, binaries or `go install`.
- **[Connect a client](clients.md)**: Claude Code, Claude Desktop, Cursor, VS Code.
- **[Read-only identity](read-only-robot.md)**: a Harbor robot account that can only read.
- **[Tools](tools.md)**: the twelve tools and their arguments.

</div>

## Design in short

- **Read-only.** Twelve tools cover projects, repositories, artifacts, vulnerabilities, robots, members, registries, replication, webhooks and garbage collection. No tool changes or deletes anything.
- **Secrets stay out of the conversation.** Harbor credentials, robot secrets and webhook authorization headers never appear in tool results, logs or errors; a test enforces it.
- **Two transports.** stdio for desktop clients, and streamable HTTP with a bearer token for running in a cluster ([HTTP mode](http.md)).
- **Signed and listed.** A signed multi-arch image, `ghcr.io/oreondev/harbor-mcp`, listed in the official MCP Registry as `io.github.oreondev/harbor-mcp`.
- **Tested against real Harbor.** Harbor 2.15 on every pull request and 2.12 to 2.15 nightly, as an administrator and as the read-only robot.

It uses the Harbor client of the [Harbor Resource Operator](https://www.oreonit.com/harbor-operator/), the Kubernetes operator that manages Harbor projects, robot accounts and replication as code.

!!! note
    This is an independent community project, maintained by [Oreon IT](https://www.oreonit.com). It is not affiliated with or endorsed by the Harbor project or the CNCF; Harbor is a trademark of The Linux Foundation.
