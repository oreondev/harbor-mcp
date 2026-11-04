---
title: Connect a client
description: Configure Claude Code, Claude Desktop, Cursor, VS Code and other MCP clients to use the Harbor MCP server.
---

# Connect a client

Every client starts the server the same way: the container image (or the binary) with `HARBOR_URL`, `HARBOR_USERNAME` and `HARBOR_PASSWORD`. Use the [read-only robot](read-only-robot.md) as the user.

=== "Claude Code"

    ```sh
    claude mcp add harbor -e HARBOR_URL=https://harbor.example.com -e 'HARBOR_USERNAME=robot$harbor-mcp' -e HARBOR_PASSWORD=... \
      -- docker run -i --rm -e HARBOR_URL -e HARBOR_USERNAME -e HARBOR_PASSWORD ghcr.io/oreondev/harbor-mcp:{{ version }}
    ```

    Or, for a [shared HTTP server](http.md):

    ```sh
    claude mcp add --transport http harbor https://harbor-mcp.example.com/mcp \
      -H "Authorization: Bearer $HARBOR_MCP_HTTP_TOKEN"
    ```

=== "Claude Desktop / Cursor"

    In `claude_desktop_config.json`, or Cursor's `~/.cursor/mcp.json`:

    ```json
    {
      "mcpServers": {
        "harbor": {
          "command": "docker",
          "args": ["run", "-i", "--rm", "-e", "HARBOR_URL", "-e", "HARBOR_USERNAME", "-e", "HARBOR_PASSWORD",
                   "ghcr.io/oreondev/harbor-mcp:{{ version }}"],
          "env": {
            "HARBOR_URL": "https://harbor.example.com",
            "HARBOR_USERNAME": "robot$harbor-mcp",
            "HARBOR_PASSWORD": "..."
          }
        }
      }
    }
    ```

=== "VS Code"

    In `.vscode/mcp.json`; VS Code asks for the password once and stores it:

    ```json
    {
      "inputs": [
        { "type": "promptString", "id": "harbor-password", "description": "Harbor robot secret", "password": true }
      ],
      "servers": {
        "harbor": {
          "type": "stdio",
          "command": "docker",
          "args": ["run", "-i", "--rm", "-e", "HARBOR_URL", "-e", "HARBOR_USERNAME", "-e", "HARBOR_PASSWORD",
                   "ghcr.io/oreondev/harbor-mcp:{{ version }}"],
          "env": {
            "HARBOR_URL": "https://harbor.example.com",
            "HARBOR_USERNAME": "robot$harbor-mcp",
            "HARBOR_PASSWORD": "${input:harbor-password}"
          }
        }
      }
    }
    ```

=== "Binary"

    Without Docker, use the [binary](install.md) as the command and pass the same variables in `env`:

    ```json
    { "command": "harbor-mcp", "env": { "HARBOR_URL": "https://harbor.example.com", "HARBOR_USERNAME": "robot$harbor-mcp", "HARBOR_PASSWORD": "..." } }
    ```

Then ask, for example, *"What Harbor version is this, and who am I connected as?"*: the assistant calls `harbor_system_info` and should answer with the version and `robot$harbor-mcp`.

!!! tip "Harbor with a private CA"
    Mount the CA into the container and set `HARBOR_CA_FILE`, for example `-v /etc/ssl/harbor-ca.pem:/ca.pem:ro -e HARBOR_CA_FILE=/ca.pem`.
