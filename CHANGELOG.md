# Changelog

All notable changes are listed here, newest first. Versions follow [semantic versioning](https://semver.org). Each release also has a [GitHub release](https://github.com/oreondev/harbor-mcp/releases) with the image digest, binaries and signed checksums.

## Unreleased

## 0.2.0 - 2026-11-08

- Agent skills stored in Harbor as OCI artifacts (the [Agent Skills OCI artifacts](https://github.com/ThomasVitale/agents-skills-oci-artifacts-spec) format): `harbor_list_skills` lists the skills in a project with version, tags and whether they are signed; `harbor_get_skill` returns a skill's `SKILL.md` or another of its files, read through the manifest so the project's cosign policy applies, with size, path and link checks on the content. Reading a skill needs `repository:pull` on its project ([#20](https://github.com/oreondev/harbor-mcp/pull/20)).
- `harbor_install_skill`, registered only when `HARBOR_MCP_SKILLS_DIR` is set, installs a skill into that directory with a `.harbor-skill.json` marker; it replaces only its own earlier install of the same repository and refuses unsigned skills unless `HARBOR_MCP_SKILLS_REQUIRE_SIGNED=false` ([#20](https://github.com/oreondev/harbor-mcp/pull/20)).
- An [Agent skills](https://www.oreonit.com/harbor-mcp/latest/skills/) docs page: pushing with ORAS, signing, the extra `repository:pull` permission for the skills project, and installing. The end-to-end suite pushes, signs, lists, reads and installs a skill as that robot, with the project's cosign policy off and on ([#20](https://github.com/oreondev/harbor-mcp/pull/20)).
- Harbor error messages no longer end in a double full stop when the server adds a hint ([#20](https://github.com/oreondev/harbor-mcp/pull/20)).

## 0.1.1 - 2026-11-05

- Documentation site at <https://www.oreonit.com/harbor-mcp/>, with a tool reference generated from the server, and these release notes ([#10](https://github.com/oreondev/harbor-mcp/pull/10)).
- `harbor-mcp -list-tools` prints every tool with its schemas and annotations as JSON ([#10](https://github.com/oreondev/harbor-mcp/pull/10)).
- Builds from `go install` report the module version instead of `dev` ([#10](https://github.com/oreondev/harbor-mcp/pull/10)).
- The MCP Registry entry links to the documentation site (`websiteUrl`).

## 0.1.0 - 2026-11-04

First release.

- Twelve read-only tools: `harbor_system_info`, `harbor_list_projects`, `harbor_get_project`, `harbor_list_repositories`, `harbor_list_artifacts`, `harbor_get_vulnerabilities`, `harbor_list_members`, `harbor_list_robots`, `harbor_list_registries`, `harbor_list_replications`, `harbor_list_webhooks` and `harbor_get_gc`. None of them changes anything in Harbor.
- stdio, and streamable HTTP with a bearer token.
- A documented read-only robot account (`docs/read-only-robot.json`) that every tool works with.
- Image `ghcr.io/oreondev/harbor-mcp` for amd64 and arm64, and binaries for Linux, macOS and Windows, all signed with cosign; listed in the MCP Registry as `io.github.oreondev/harbor-mcp`.
- Tested end to end on kind against Harbor 2.15; the nightly run covers 2.12 to 2.15.
