---
title: Read-only identity
description: Create a Harbor system robot account that can only read, so the AI assistant gets exactly what the Harbor MCP server needs.
---

# Read-only identity

Run the server as a Harbor system robot account that can only read. [read-only-robot.json](read-only-robot.json) is the request body for Harbor's API, with exactly the permissions the tools need. The end-to-end tests create the same robot and run every tool as it.

```sh
curl -u admin -H 'Content-Type: application/json' \
  -d @read-only-robot.json https://harbor.example.com/api/v2.0/robots
```

Harbor shows the secret once, in the response; the user name is `robot$harbor-mcp`. Store the secret in your client's configuration or a secret manager, not in a shared file.

## What it can read

| Level | Resources |
|---|---|
| System | projects, quotas, robot accounts, registries, replication policies and executions, garbage collection |
| Every project | project and metadata, quota, repositories, artifacts, tags, labels, artifact additions, scan reports, members, robot accounts, immutable tag rules, tag retention, webhook policies |

Nothing in the list allows a change: no `create`, `update`, `delete`, `push` or `pull`.

Reading [agent skills](skills.md) needs `pull` on the skills project; that page shows how to add it for that project only.

!!! note
    The robot has no expiry (`duration: -1`). Set a duration in days if your policy requires rotation, and update the client's secret when you refresh it.

An administrator account also works, but gives the assistant far more than it needs.
