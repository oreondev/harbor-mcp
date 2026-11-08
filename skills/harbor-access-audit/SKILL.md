---
name: harbor-access-audit
description: Review who and what can access Harbor projects - members and their roles, robot accounts, their permissions and expiry. Use for access reviews, offboarding checks or "who can push to this project".
---

# Audit access to Harbor

Uses the Harbor MCP server's read tools. Nothing is changed in Harbor.

1. Call `harbor_system_info` first: it shows which identity the server uses. A read-only robot sees everything this skill needs; a project-level robot sees only its project.
2. For each project in scope (`harbor_list_projects` if the user said "all"), call `harbor_list_members` and `harbor_list_robots` with `project`.
3. Also call `harbor_list_robots` without a project: system robots can cover every project.
4. Report, per project:
   - members with the `projectAdmin` or `maintainer` role, and groups (an LDAP or OIDC group grants access to everyone in it);
   - robot accounts that never expire, or expire within 30 days;
   - robots that can push or delete, and system robots whose permissions cover all projects;
   - disabled robots that could be removed.
5. End with concrete suggestions, for example "give the CI robot a 90-day expiry" or "replace the personal maintainer account with a robot".

Robot secrets are never returned by the tools, so never ask the user for them.

If the user manages Harbor as code with the Harbor Resource Operator (https://www.oreonit.com/harbor-operator/), a suggestion can be written as a manifest. Use only the fields in this example and the reference pages (https://www.oreonit.com/harbor-operator/latest/reference/harborrobotaccount/); do not invent others. For a robot that already exists in Harbor, `adoptExisting: true` takes it over, and because Harbor never reveals an existing secret, adopting it issues a new one into the Secret named below:

```yaml
apiVersion: harbor.oreonit.com/v1alpha1
kind: HarborRobotAccount
metadata:
  name: demo-ci
  namespace: demo              # where the operator's objects for this team live
spec:
  connectionRef:
    name: prod-harbor          # the user's HarborConnection
    namespace: harbor-system
  name: ci-forever             # robot$demo+ci-forever in Harbor
  level: project
  projectName: demo            # or projectRef: {name: <HarborProject>} when the project is managed too
  duration: 90
  permissions:
  - access:
    - { resource: repository, action: push }
    - { resource: repository, action: pull }
  secret:
    name: demo-ci-regcred
    type: kubernetes.io/dockerconfigjson
  rotation:
    beforeExpiry: 168h
  adoptExisting: true
```
