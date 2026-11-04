# Override BASE_IMAGE to build from another registry, e.g. docker.io/library/golang:1.26
ARG BASE_IMAGE=golang:1.26
# The builder runs on the build platform and cross-compiles for TARGETOS/TARGETARCH,
# so multi-arch builds need no emulation.
FROM --platform=${BUILDPLATFORM} ${BASE_IMAGE} AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o harbor-mcp ./cmd/harbor-mcp

FROM gcr.io/distroless/static:nonroot
# The MCP Registry checks that this matches the name in server.json.
LABEL io.modelcontextprotocol.server.name="io.github.oreondev/harbor-mcp"
COPY --from=builder /workspace/harbor-mcp /harbor-mcp
USER 65532:65532
# stdio by default (docker run -i); add -http :8080 for streamable HTTP.
ENTRYPOINT ["/harbor-mcp"]
