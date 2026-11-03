# Security policy

The server holds credentials for Harbor and passes Harbor data to AI assistants, so security reports are taken seriously. Of particular interest: any way a tool result, log line or error leaks the Harbor credentials, the HTTP token, a robot secret or a webhook `auth_header`, and any way to reach a write tool when write tools are off.

## Reporting a vulnerability

Please do **not** open a public issue. Report privately by either:

- GitHub: the repository's **Security** tab → **Report a vulnerability**, or
- e-mail: fahri.cap@oreonit.com

Include the version, how to reproduce, and the impact you see. You will get an acknowledgement within 5 working days and an assessment within 14 days. Fixes are released as a patch version, and the advisory is published once a fix is available; reporters are credited unless they ask not to be.

## Supported versions

Only the latest minor release gets security fixes while the project is at `0.x`.

## Scope

In scope: the server code and the container image published from this repository. The Harbor client it uses lives in [harbor-resource-operator](https://github.com/oreondev/harbor-resource-operator); reports about the client can go to either repository. Vulnerabilities in Harbor itself go to the [Harbor project](https://github.com/goharbor/harbor/security/policy).
