/*
Copyright 2026 Oreon IT.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The Cursor plugin at the repository root (.cursor-plugin/plugin.json,
// mcp.json and the skills in skills/) follows Cursor's submission checklist
// and starts the image of the current release.
func TestCursorPlugin(t *testing.T) {
	root := filepath.Join("..", "..")
	var manifest struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		Homepage    string `json:"homepage"`
		License     string `json:"license"`
		Logo        string `json:"logo"`
		Skills      string `json:"skills"`
		MCPServers  string `json:"mcpServers"`
		Variables   struct {
			Properties map[string]struct {
				Type  string `json:"type"`
				Title string `json:"title"`
			} `json:"properties"`
			Required []string `json:"required"`
		} `json:"variables"`
	}
	readJSON(t, filepath.Join(root, ".cursor-plugin", "plugin.json"), &manifest)
	var server struct{ Version string }
	readJSON(t, filepath.Join(root, "server.json"), &server)

	if !ValidName(manifest.Name) || manifest.Description == "" || manifest.Homepage == "" || manifest.License == "" {
		t.Fatalf("manifest: %+v", manifest)
	}
	if manifest.Version != server.Version {
		t.Fatalf("plugin version %s, server.json %s", manifest.Version, server.Version)
	}
	for _, p := range []string{manifest.Logo, manifest.Skills, manifest.MCPServers} {
		if p == "" || filepath.IsAbs(p) || strings.Contains(p, "..") {
			t.Fatalf("manifest path %q: want a relative path inside the repository", p)
		}
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Fatal(err)
		}
	}

	mcp, err := os.ReadFile(filepath.Join(root, manifest.MCPServers))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(mcp) || !strings.Contains(string(mcp), "ghcr.io/oreondev/harbor-mcp:"+server.Version+`"`) {
		t.Fatalf("%s: want valid JSON starting the %s image", manifest.MCPServers, server.Version)
	}
	for _, m := range regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`).FindAllStringSubmatch(string(mcp), -1) {
		if _, ok := manifest.Variables.Properties[m[1]]; !ok {
			t.Errorf("%s uses ${%s}, which the manifest does not declare", manifest.MCPServers, m[1])
		}
	}
	for _, v := range manifest.Variables.Required {
		if _, ok := manifest.Variables.Properties[v]; !ok {
			t.Errorf("required variable %s is not declared", v)
		}
	}

	dirs, err := os.ReadDir(filepath.Join(root, manifest.Skills))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("skills: %v", err)
	}
	front := regexp.MustCompile(`(?s)\A---\nname: (\S+)\ndescription: (.+?)\n---\n`)
	for _, d := range dirs {
		b, err := os.ReadFile(filepath.Join(root, manifest.Skills, d.Name(), "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		m := front.FindSubmatch(b)
		if m == nil || string(m[1]) != d.Name() || !ValidName(d.Name()) || len(m[2]) > 1024 {
			t.Errorf("%s/SKILL.md: want front matter with name %q and a description", d.Name(), d.Name())
		}
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
