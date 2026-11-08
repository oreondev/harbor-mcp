# Harbor MCP server: design

Status: version 0.1.0 is released with the read tools (section 4), both transports and the release workflow. Version 0.5, 2026-11-04.

## 1. Purpose

A Model Context Protocol (MCP) server that lets AI assistants (Claude, Cursor, VS Code, Docker's MCP Toolkit, ToolHive on Kubernetes) answer questions about a running Harbor: which projects exist and how full their quotas are, what was pushed recently, which artifacts have critical vulnerabilities, whether replication is healthy, which robot accounts expire soon. It reuses the Harbor client and the fake Harbor from [harbor-resource-operator](https://github.com/oreondev/harbor-resource-operator) (`pkg/harbor`, `pkg/harbor/fake`), which are tested against Harbor 2.12 to 2.15, pinned to an operator release.

Scope matches the operator's: objects inside a running Harbor through API v2.0. It does not install Harbor, does not move image contents (the one exception is reading agent skills, section 10, which are small text artifacts), and never deletes anything.

On 2026-11-03 no maintained Harbor MCP server existed: `bupd/harbor-mcp-server` (TypeScript, 12 tools, last push 2026-02-20, not official), `nomagicln/mcp-harbor` (archived), and no Harbor entry in the official MCP Registry.

## 2. Shape

| Item | Choice |
|---|---|
| Binary | `cmd/harbor-mcp` in this repository, module `github.com/oreondev/harbor-mcp`, released from its own `vX.Y.Z` tags |
| Image | `ghcr.io/oreondev/harbor-mcp:X.Y.Z`, multi-arch (amd64, arm64), distroless, non-root, signed with cosign, SBOM and provenance |
| SDK | `github.com/modelcontextprotocol/go-sdk` (official Go SDK, v1.8.0 on 2026-11-03) |
| Transports | stdio (default; for desktop clients and Docker's toolkit) and streamable HTTP with `-http <addr>` (for ToolHive, kmcp or a Deployment): MCP at `/mcp`, an unauthenticated `GET /healthz` for probes, stateless sessions so replicas need no affinity, cross-origin browser requests refused. TLS is left to the ingress or mesh in front |
| Registry name | `io.github.oreondev/harbor-mcp` (namespace proved by GitHub Actions OIDC in the release workflow) |
| Licence | Apache-2.0, same as the repository |

## 3. Configuration

Environment variables, each with a `_FILE` variant for mounted Secrets:

| Variable | Meaning |
|---|---|
| `HARBOR_URL` | Harbor base URL, `https://harbor.example.com` |
| `HARBOR_USERNAME`, `HARBOR_PASSWORD` | Credentials. The documented setup is a system robot account with read permissions only; admin works but is not recommended |
| `HARBOR_CA_FILE` | Extra CA bundle (PEM) |
| `HARBOR_INSECURE_SKIP_VERIFY` | `true` to skip TLS verification; logged as a warning at start |
| `HARBOR_MCP_RATE` | Requests per second to Harbor, default 10, shared by all tools (the operator's limiter) |
| `HARBOR_MCP_ALLOW_WRITE` | `true` registers the write tools (section 5); off by default |
| `HARBOR_MCP_HTTP_TOKEN` | Bearer token required on every HTTP request in `-http` mode; the server refuses to start in HTTP mode without one unless `-http` binds to a loopback address |
| `HARBOR_MCP_SKILLS_DIR` | Absolute path; registers `harbor_install_skill` (section 10.3) |
| `HARBOR_MCP_SKILLS_REQUIRE_SIGNED` | `false` lets `harbor_install_skill` install unsigned skills; default `true` |

Credentials and the HTTP token are never logged, never returned in tool output and never included in error messages. Robot secrets and webhook `auth_header` values are removed from every result.

## 4. Read tools (first release)

Every read tool carries the MCP annotations `readOnlyHint: true` and `openWorldHint: true`. Lists take `query` (name filter), `page` and `pageSize` (default 25, maximum 100) and report the total, so an assistant can page instead of receiving thousands of rows. Results are JSON (`structuredContent` with an output schema, also sent as text). Times are RFC 3339 strings in UTC, empty when Harbor has none (an artifact never pulled).

| Tool | Harbor endpoints | Returns |
|---|---|---|
| `harbor_system_info` | `GET /systeminfo`, `GET /users/current` | version, auth mode, the identity the server runs as |
| `harbor_list_projects` | `GET /projects` | name, ID, public, repository count, auto-scan, proxy cache, created (quota is in `harbor_get_project`, to keep a page at one request) |
| `harbor_get_project` | `GET /projects/{name}`, `GET /quotas?reference=project&reference_id=`, `GET /projects/{id}/immutabletagrules` | metadata, quota, CVE allowlist, whether a retention policy is set, number of immutable rules |
| `harbor_list_repositories` | `GET /projects/{name}/repositories` | name, artifact count, pull count, last update |
| `harbor_list_artifacts` | `GET /projects/{name}/repositories/{repo}/artifacts?with_tag=true&with_scan_overview=true` | digest, tags, size, push and pull time, scan status and severity counts |
| `harbor_get_vulnerabilities` | `GET .../artifacts/{reference}/additions/vulnerabilities` | the scan report for one artifact, most severe first, filtered by `minSeverity` (default High) and capped by `limit` (at most 100), with counts by severity; a project without a scanner (Harbor's 404 "no scanner registration") is a note, not an error |
| `harbor_list_robots` | `GET /robots` | name, level, permissions, disabled, expiry and whether expired (secrets never); optional filters by project and name |
| `harbor_list_members` | `GET /projects/{name}/members` | entity, type, role |
| `harbor_list_registries` | `GET /registries` | name, type, URL, health status (credentials never) |
| `harbor_list_replications` | `GET /replication/policies`, `GET /replication/executions?policy_id=` | policy, direction, trigger, enabled, last execution status and counts |
| `harbor_list_webhooks` | `GET /projects/{name}/webhook/policies` | name, events, targets (address and type; `auth_header` removed) |
| `harbor_get_gc` | `GET /system/gc/schedule`, `GET /system/gc` | schedule and the last run |

The client gains list methods for projects, repositories, artifacts and vulnerability reports (added in the operator repository, then pinned here); the rest exist already. The fake server (`pkg/harbor/fake`) gets the same endpoints, with the documented quirks kept (`job_parameters` as a JSON string, empty 200 body for no GC schedule).

## 5. Write tools (opt-in)

Registered only with `HARBOR_MCP_ALLOW_WRITE=true`, annotated `readOnlyHint: false`, `destructiveHint: false`, `idempotentHint` where true. None of them deletes or overwrites data.

| Tool | Harbor endpoint |
|---|---|
| `harbor_scan_artifact` | `POST .../artifacts/{reference}/scan` |
| `harbor_start_replication` | `POST /replication/executions` |
| `harbor_add_artifact_label` | `POST .../artifacts/{reference}/labels` |
| `harbor_create_project` | `POST /projects` (name, public, quota; nothing else) |

Not offered in any mode: deleting projects, repositories, artifacts, tags or robots; creating robot accounts (the secret would land in a chat transcript; use the operator's `HarborRobotAccount`, which writes it to a Secret); user, LDAP, OIDC and system configuration changes.

## 6. Errors

Harbor errors map to MCP tool errors (`isError: true`) with Harbor's status code and message, so the assistant can explain them: 401 and 403 name the missing permission, 404 says what was not found, 429 and 5xx say to retry. A 500 on a deleted retention policy is reported as "not found", as in the operator.

## 7. Testing

- Unit tests for every tool against the fake Harbor, through the SDK's in-memory transport, so the JSON schemas and annotations are tested as clients see them.
- A test that no tool result contains the configured password, the HTTP token, a robot secret or a webhook `auth_header`.
- An e2e test that starts the binary against a real Harbor from the official chart on kind and calls each read tool, on Harbor 2.12 to 2.15 like the operator.
- `make lint` at 0 issues.

## 8. Packaging and listings

- Release workflow (a `vX.Y.Z` tag on `main`): build, push and sign `ghcr.io/oreondev/harbor-mcp`, with the label `io.modelcontextprotocol.server.name=io.github.oreondev/harbor-mcp` that the registry checks; then publish `server.json` to the official MCP Registry with `mcp-publisher` and GitHub OIDC. The registry feeds GitHub's MCP registry, VS Code and other clients.
- `server.json` lives at the repository root and is versioned with the release.
- After the first release, one submission each: Docker MCP Catalog (`docker/mcp-registry`), ToolHive catalog (`stacklok/toolhive-catalog`), LobeHub (`lhm` CLI), a Cursor marketplace plugin that wraps the server, Smithery and mcp.so. Each listing sets the homepage to the documentation page and states that the server is a community project, not affiliated with the Harbor project or the CNCF.
- Helm: a small hand-written chart `charts/harbor-mcp` for HTTP mode (Deployment, Service, Secret reference, NetworkPolicy), from the second release on.

## 9. Order of work

1. Client list methods and fake endpoints, with tests, in the operator repository; pin that commit here.
2. Server skeleton: configuration, stdio transport, `harbor_system_info`, the in-memory test harness.
3. The other read tools.
4. HTTP transport with the bearer token.
5. Image, signing, `server.json`, registry publishing in the release workflow; README section.
6. e2e spec.
7. Listings (section 8).
8. Write tools, then the Helm chart.

## 10. Agent skills

Harbor as the registry for the agent skills a team writes: a skill (a folder with `SKILL.md`, as in the [Agent Skills](https://agentskills.io/specification) format) is pushed to a Harbor project as an OCI artifact, signed, and listed, read and installed through this server. Design version 2026-11-06.

### 10.1 Format

The [Agent Skills OCI artifacts spec](https://github.com/ThomasVitale/agents-skills-oci-artifacts-spec) (v0.1.0 draft, April 2026). On 2026-11-06 it had the most implementations (arconia-cli, salaboy/skills-oci), NVIDIA's agent-package-spec builds on it, and it can be pushed with plain ORAS. One manifest per skill version:

| Field | Value |
|---|---|
| `artifactType` | `application/vnd.agentskills.skill.v1` |
| config | `application/vnd.agentskills.skill.config.v1+json` (name, description and the other `SKILL.md` frontmatter) |
| one layer | `application/vnd.agentskills.skill.content.v1.tar+gzip`, rooted at `<skill-name>/` |
| annotations | `io.agentskills.skill.name` (required), `org.opencontainers.image.{title,description,version,source,revision,licenses}` |
| tags | the version (`1.2.0`, immutable by a tag rule), plus `latest` |

Convention: one repository per skill, `<project>/<skill-name>`. Other formats (NVIDIA, ToolHive, Alibaba ACR) are listed with their type but not read until one of them is adopted more widely.

### 10.2 What Harbor does with it (checked on Harbor 2.15, 2026-11-06)

- The artifact gets `type: SKILL` (Harbor's default processor derives it from the config media type), `artifact_type`, the manifest annotations, and the config JSON as `extra_attrs`. Listing skills needs only the REST API; `q=type=SKILL` filters a repository's artifacts.
- `GET /v2/<project>/<repo>/manifests/<ref>` and `/blobs/<digest>` accept basic auth, so reading a skill needs no token service.
- With the project's `enable_content_trust_cosign` on, a manifest pull of an unsigned artifact returns 412 `PROJECTPOLICYVIOLATION` "The image is not signed by cosign."; a blob pull by digest is not checked. This server therefore always resolves the manifest first, so the project policy decides whether a skill can be read.
- Harbor's scanners only take `IMAGE` artifacts (`controller/scan/util.go`, read in the source), so Harbor does not scan skills. Scanning happens in CI before the push (NVIDIA SkillSpector or Cisco skill-scanner, both Apache-2.0); the report can be attached as a referrer (an attestation) next to the signature.

### 10.3 Tools

Read tools, registered by default (they read only what the robot can pull):

| Tool | Harbor calls | Returns |
|---|---|---|
| `harbor_list_skills` | `GET /projects/{p}/repositories`, then `GET .../artifacts?q=type=SKILL&with_tag=true&with_accessory=true` per repository | per skill: name, description, version, tags, digest, push time, whether a cosign signature is attached; paged over repositories |
| `harbor_get_skill` | the artifact, then `GET /v2/.../manifests/{ref}` and `/blobs/{layer}` | the `SKILL.md` text, the file list with sizes, digest, version, signed; optional `path` returns one other text file of the skill |

Install, registered only when `HARBOR_MCP_SKILLS_DIR` is set (stdio use; in HTTP mode the directory would be on the server, which is rarely what the user wants, so the server warns):

| Tool | Does |
|---|---|
| `harbor_install_skill` | reads the skill as `harbor_get_skill` does, writes it to a temporary directory inside `<dir>` through `os.Root`, with `.harbor-skill.json` (repository, digest, version, signed), and renames it to `<dir>/<skill-name>/`. Refuses when that path exists and is not a directory with a marker naming the same repository (a symlink, a file, a directory it did not install); a failed install leaves the old one in place. A skill that carries its own marker file is refused. With `HARBOR_MCP_SKILLS_REQUIRE_SIGNED=true` (default) it refuses a skill without a cosign signature accessory. Annotated destructive and idempotent; one install at a time |

Permissions: listing needs only what the documented robot has. Reading and installing need `repository:pull`, which that robot does not have; the docs add a second project-level entry granting `repository:pull` on the skills projects only (`namespace: "skills"`, not `*`), so the robot still cannot read image contents elsewhere. Without it the tools report Harbor's 401 or 403 with that hint.

Limits on every read: compressed layer at most 1 MiB, unpacked at most 4 MiB and 500 entries; only regular files and directories; no absolute paths, `..`, symlinks or hard links; mode bits limited to 0644 and 0755. Text returned to the model is capped at 64 KiB per file. A skill is instructions for the model, so tool results say which project and digest it came from and whether it is signed.

Signing: the server checks that a signature exists; it does not verify it cryptographically. Verification is Harbor's job (`enableContentTrustCosign` on the project, which Harbor enforces at pull time; keyless identities are checked by `cosign verify` in CI). The docs recommend the policy for every skills project.

### 10.4 Client and fake (operator repository first)

- `Artifact` gains `ArtifactType`, `Annotations`, `ExtraAttrs` and `Accessories` (type, digest); `ListArtifactsOfType(ctx, project, repo, type, po)` adds `q=type=<T>` and `with_accessory=true`.
- `GetManifest(ctx, project, repo, ref)` and `GetBlob(ctx, project, repo, digest, limit)` on the registry API (`/v2/`, basic auth, digest checked, `limit` enforced while reading). A 412 policy violation becomes an `APIError` with Harbor's message.
- The fake stores manifests, blobs, artifact types and accessories, filters by `q=type=`, and can enforce the cosign policy per project, with the 412 body above.

### 10.5 Around the server

- Push: documented with ORAS (`oras push --artifact-type application/vnd.agentskills.skill.v1 --config ...`) and a GitHub Actions workflow: pack, scan with SkillSpector, push to Harbor, `cosign sign` keyless, attach the scan report. No CLI of our own; arconia-cli and skr exist.
- Operator: a sample for a skills project: `HarborProject` with `enableContentTrustCosign`, an immutable tag rule for version tags, a retention policy keeping the last ten versions, a CI robot that can push and an agent robot that can only pull.
- Docs: a "Skills in Harbor" page on both sites, and a blog post.

### 10.6 Order of work

1. Client and fake in the operator repository (10.4), with tests and a Harbor e2e check; release the operator patch and bump the pin here.
2. `harbor_list_skills` and `harbor_get_skill`, with tests against the fake, including the unpack limits and a hostile tarball.
3. `harbor_install_skill`.
4. e2e on kind: push a skill with oras-go, sign it with cosign, list, read and install it as the read-only robot, with the project policy on and off.
5. Operator sample, push workflow, docs page, blog post.
