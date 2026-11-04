---
title: HTTP mode
description: Run the Harbor MCP server as a shared streamable HTTP service with a bearer token, for example as a Kubernetes Deployment next to Harbor.
---

# HTTP mode

For a shared server, for example a Deployment next to Harbor, start it with `-http` and a token:

```sh
docker run -d -p 8080:8080 \
  -e HARBOR_URL -e HARBOR_USERNAME -e HARBOR_PASSWORD -e HARBOR_MCP_HTTP_TOKEN \
  ghcr.io/oreondev/harbor-mcp:{{ version }} -http :8080
```

- MCP is served at `/mcp` (streamable HTTP, stateless: any replica can serve any request).
- Every request needs `Authorization: Bearer <HARBOR_MCP_HTTP_TOKEN>`; others get 401.
- `GET /healthz` is open, for liveness and readiness probes.
- Browser cross-origin requests are refused.
- Without a token the server refuses to listen beyond a loopback address.
- TLS is left to what is in front of it: an ingress, a gateway or a service mesh.

Clients connect with the URL and the header:

```sh
claude mcp add --transport http harbor https://harbor-mcp.example.com/mcp \
  -H "Authorization: Bearer $HARBOR_MCP_HTTP_TOKEN"
```

## On Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: harbor-mcp
spec:
  replicas: 2
  selector: { matchLabels: { app: harbor-mcp } }
  template:
    metadata: { labels: { app: harbor-mcp } }
    spec:
      containers:
      - name: harbor-mcp
        image: ghcr.io/oreondev/harbor-mcp:{{ version }}
        args: ["-http", ":8080"]
        env:
        - { name: HARBOR_URL, value: https://harbor.example.com }
        - { name: HARBOR_USERNAME_FILE, value: /secrets/username }
        - { name: HARBOR_PASSWORD_FILE, value: /secrets/password }
        - { name: HARBOR_MCP_HTTP_TOKEN_FILE, value: /secrets/token }
        ports: [{ containerPort: 8080 }]
        readinessProbe: { httpGet: { path: /healthz, port: 8080 } }
        livenessProbe: { httpGet: { path: /healthz, port: 8080 } }
        volumeMounts: [{ name: secrets, mountPath: /secrets, readOnly: true }]
      volumes:
      - name: secrets
        secret: { secretName: harbor-mcp }   # keys: username, password, token
```

Put a Service and your usual ingress with TLS in front of it.
