---
title: Agent skills in Harbor
description: Store the agent skills your team writes in Harbor as signed OCI artifacts, and let the AI assistant list, read and install them through the Harbor MCP server.
---

# Agent skills in Harbor

An [agent skill](https://agentskills.io/specification) is a folder with a `SKILL.md` and, optionally, scripts and reference files. Harbor can store skills like images: one OCI artifact per version, in the [Agent Skills OCI artifacts](https://github.com/ThomasVitale/agents-skills-oci-artifacts-spec) format, with tags, immutability, retention, replication and cosign signatures. The server then lets the assistant:

- list the skills in a project (`harbor_list_skills`), with version, tags and whether each is signed;
- read one (`harbor_get_skill`): its `SKILL.md`, its file list, or another of its files;
- install one into your skills directory (`harbor_install_skill`), when you turn that on.

Use one project for skills (`skills` below) and one repository per skill.

## Push a skill

Pack the folder so the archive is rooted at the skill's name, write the config from the `SKILL.md` front matter, and push with [ORAS](https://oras.land):

```sh
tar czf review.tar.gz review/          # review/SKILL.md, review/scripts/...
echo '{"name":"review","description":"Reviews pull requests."}' > config.json
oras push harbor.example.com/skills/review:1.0.0 \
  --artifact-type application/vnd.agentskills.skill.v1 \
  --config config.json:application/vnd.agentskills.skill.config.v1+json \
  --annotation io.agentskills.skill.name=review \
  --annotation org.opencontainers.image.version=1.0.0 \
  review.tar.gz:application/vnd.agentskills.skill.content.v1.tar+gzip
```

Harbor shows the artifact with type `SKILL`.

## Sign it, and make Harbor enforce signatures

```sh
cosign sign --yes harbor.example.com/skills/review@sha256:...
```

Then turn on the project's cosign policy (in the Harbor UI: project, Configuration, "Cosign": prevent unsigned artifacts from being pulled; with the [Harbor Resource Operator](https://www.oreonit.com/harbor-operator/): `enableContentTrustCosign: true` on the `HarborProject`). Harbor then refuses to serve an unsigned skill, and the server says so. The server itself only checks that a signature is attached; Harbor's policy, or `cosign verify` in CI, checks who signed it.

Harbor's vulnerability scanners only handle images, so scan skills in CI before the push.

## Let the robot read skills

The [read-only robot](read-only-robot.md) can list skills but not read their content: that needs `repository:pull`. Grant it on the skills project only, so the robot still cannot pull images elsewhere:

```sh
jq '.permissions += [{"kind": "project", "namespace": "skills",
      "access": [{"resource": "repository", "action": "pull"}]}]' \
  read-only-robot.json > skills-robot.json
curl -u admin -H 'Content-Type: application/json' \
  -d @skills-robot.json https://harbor.example.com/api/v2.0/robots
```

The project must exist before you create the robot. The end-to-end tests create the same robot.

## Install skills

Set `HARBOR_MCP_SKILLS_DIR` to the absolute path of your client's skills directory, for example `/home/me/.claude/skills` for Claude Code. That registers `harbor_install_skill`, which:

- writes the skill to `<dir>/<name>/` with a `.harbor-skill.json` file naming the repository, digest and version;
- replaces only its own earlier install of the same repository, and refuses any other file or folder of that name;
- refuses skills without a cosign signature, unless `HARBOR_MCP_SKILLS_REQUIRE_SIGNED=false`.

A skill is instructions your assistant will follow, written by whoever pushed it. Read it with `harbor_get_skill` before installing it; the tool's description asks the assistant to show it to you and ask first.

Install is meant for stdio, where the server runs on your machine. In HTTP mode the skills would land on the server's machine, and the server logs a warning. With Docker, mount the directory; see [Install](install.md).
