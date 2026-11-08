#!/usr/bin/env bash
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

# Bumps every versioned file to X.Y.Z for a release: server.json (version and
# image), the README image references, the Cursor plugin (.cursor-plugin/plugin.json
# and mcp.json) and CHANGELOG.md (Unreleased becomes
# "X.Y.Z - today"). Run it on a chore/release-X.Y.Z branch from develop; it
# does not commit.
set -euo pipefail

new="${1:?usage: release-bump.sh X.Y.Z}"
cd "$(dirname "$0")/.."

if ! echo "$new" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "version $new is not MAJOR.MINOR.PATCH" >&2; exit 1
fi
old=$(jq -r .version server.json)
if [ "$old" = "$new" ]; then
  echo "server.json is already $new" >&2; exit 1
fi
if git rev-parse -q --verify "refs/tags/v$new" > /dev/null; then
  echo "tag v$new already exists" >&2; exit 1
fi
if ! ./hack/release-notes.sh Unreleased > /dev/null 2>&1; then
  echo "CHANGELOG.md has nothing under Unreleased; add the release notes first" >&2; exit 1
fi

old_re=${old//./\\.}
# Only the version field and image references, so jq's formatting is not needed.
sed -i -E -e "s/(\"version\": \")$old_re\"/\1$new\"/" -e "s/(harbor-mcp:)$old_re\"/\1$new\"/" server.json
sed -i -E -e "s/(harbor-mcp:)$old_re([^0-9]|$)/\1$new\2/g" -e "s/replace $old_re with/replace $new with/" README.md
# The Cursor plugin: its version and the image it starts.
sed -i -E "s/(\"version\": \")$old_re\"/\1$new\"/" .cursor-plugin/plugin.json
sed -i -E "s/(harbor-mcp:)$old_re\"/\1$new\"/" mcp.json
sed -i "s/^## Unreleased$/## Unreleased\n\n## $new - $(date -u +%F)/" CHANGELOG.md
make validate-server-json

echo "Bumped $old to $new. Check git diff, then open the release PR."
