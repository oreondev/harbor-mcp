# Copyright 2026 Oreon IT.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""MkDocs hooks: generate the tool reference and release notes, and fill in the release version.

docs/tools.md is written from `harbor-mcp -list-tools`, so it always matches
the code; it is generated at build time and not committed.
"""

import json
import os
import shutil
import subprocess

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


def release_version():
    """The version in server.json, which every release bumps."""
    with open(os.path.join(ROOT, "server.json")) as f:
        return json.load(f)["version"]


def cell(text):
    return " ".join(str(text).split()).replace("|", "\\|")


def type_of(schema):
    t = schema.get("type", "object")
    if isinstance(t, list):
        t = " or ".join(x for x in t if x != "null")
    if t == "array":
        return "[]" + type_of(schema.get("items", {}))
    return t


def tools_page(tools):
    lines = [
        "---",
        "title: Tools",
        "description: Every tool of the Harbor MCP server with its arguments, generated from the server itself.",
        "---",
        "",
        "# Tools",
        "",
        f"The server has {len(tools)} tools, all read-only (`readOnlyHint`); none of them changes or deletes anything in Harbor. "
        "Lists are paged: `page` starts at 1, `pageSize` defaults to 25 and is at most 100. "
        "This page is generated from `harbor-mcp -list-tools`.",
        "",
        "| Tool | What it answers |",
        "|---|---|",
    ]
    lines += [f"| [`{t['name']}`](#{t['name']}) | {cell(t['annotations'].get('title', ''))} |" for t in tools]
    for t in tools:
        lines += ["", f"## `{t['name']}`", "", t.get("description", ""), ""]
        props = t.get("inputSchema", {}).get("properties", {})
        required = set(t.get("inputSchema", {}).get("required", []))
        if props:
            lines += ["| Argument | Type | Required | Description |", "|---|---|---|---|"]
            for name in props:
                p = props[name]
                lines.append(f"| `{name}` | {cell(type_of(p))} | {'yes' if name in required else ''} | {cell(p.get('description', ''))} |")
        else:
            lines.append("No arguments.")
        out = t.get("outputSchema", {}).get("properties", {})
        if out:
            lines += ["", "Returns: " + ", ".join(f"`{k}`" for k in out) + "."]
    return "\n".join(lines) + "\n"


def on_pre_build(config, **kwargs):
    raw = subprocess.run(["go", "run", "./cmd/harbor-mcp", "-list-tools"], cwd=ROOT, check=True,
                         capture_output=True, text=True).stdout
    with open(os.path.join(ROOT, "docs", "tools.md"), "w") as f:
        f.write(tools_page(json.loads(raw)))
    shutil.copyfile(os.path.join(ROOT, "CHANGELOG.md"), os.path.join(ROOT, "docs", "changelog.md"))


def on_page_markdown(markdown, page, config, files, **kwargs):
    return markdown.replace("{{ version }}", release_version())
