//go:build e2e

/*
Copyright 2026 Fahri Cap.

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

// Package e2e runs the built binary over stdio against a real Harbor named by
// HARBOR_URL, HARBOR_USERNAME and HARBOR_PASSWORD (an administrator). Run with
// go test -tags=e2e ./test/e2e/; it skips when HARBOR_URL is unset.
package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

const project = "harbor-mcp-e2e"

// start builds the binary, runs it over stdio and returns a session and the
// server's stderr.
func start(t *testing.T) (*mcp.ClientSession, *strings.Builder) {
	t.Helper()
	if harborURL() == "" {
		t.Skip("HARBOR_URL not set")
	}
	bin := filepath.Join(t.TempDir(), "harbor-mcp")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=e2e", "-o", bin, "../../cmd/harbor-mcp")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin)
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "e2e"}, nil).
		Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v\nstderr: %s", err, stderr.String())
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs, stderr
}

// call invokes a tool and returns its structured result as a generic map.
func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	raw, _ := json.Marshal(res)
	if res.IsError {
		t.Fatalf("%s: tool error: %s", name, raw)
	}
	if strings.Contains(string(raw), os.Getenv("HARBOR_PASSWORD")) {
		t.Fatalf("%s: result contains the password", name)
	}
	b, _ := json.Marshal(res.StructuredContent)
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	t.Logf("%s: %s", name, clip(string(b), 400))
	return out
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func items(m map[string]any) []map[string]any {
	list := m["items"].([]any)
	out := make([]map[string]any, 0, len(list))
	for _, i := range list {
		out = append(out, i.(map[string]any))
	}
	return out
}

func TestEveryReadToolAgainstRealHarbor(t *testing.T) {
	cs, stderr := start(t)
	user, pass := os.Getenv("HARBOR_USERNAME"), os.Getenv("HARBOR_PASSWORD")
	ctx := context.Background()
	hc, err := harbor.New(harbor.Options{URL: harborURL(), Username: user, Password: pass})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hc.GetProjectByName(ctx, project); harbor.IsNotFound(err) {
		if _, err := hc.CreateProject(ctx, harbor.ProjectReq{ProjectName: project}); err != nil {
			t.Fatalf("create project: %v", err)
		}
	}
	t.Cleanup(func() {
		if err := deleteRepository(user, pass, project, "team/app"); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if p, err := hc.GetProjectByName(ctx, project); err == nil {
			_ = hc.DeleteProject(ctx, p.ProjectID)
		}
	})
	for _, tag := range []string{"1.0", "1.1"} {
		if err := pushTestImage(user, pass, project+"/team/app", tag); err != nil {
			t.Fatalf("push %s: %v", tag, err)
		}
	}

	info := call(t, cs, "harbor_system_info", nil)
	if !strings.HasPrefix(info["harborVersion"].(string), "v2.") || info["user"] != user {
		t.Fatalf("system info: %v", info)
	}
	ps := call(t, cs, "harbor_list_projects", map[string]any{"query": project})
	if ps["total"].(float64) < 1 || items(ps)[0]["name"] != project {
		t.Fatalf("projects: %v", ps)
	}
	d := call(t, cs, "harbor_get_project", map[string]any{"project": project})
	if d["repositoryCount"].(float64) != 1 || d["quota"] == nil {
		t.Fatalf("project: %v", d)
	}
	rs := call(t, cs, "harbor_list_repositories", map[string]any{"project": project})
	if items(rs)[0]["name"] != project+"/team/app" || items(rs)[0]["artifactCount"].(float64) != 2 {
		t.Fatalf("repositories: %v", rs)
	}
	as := call(t, cs, "harbor_list_artifacts", map[string]any{"project": project, "repository": project + "/team/app"})
	if as["total"].(float64) != 2 || items(as)[0]["tags"].([]any)[0] != "1.1" || items(as)[0]["pulled"] != nil {
		t.Fatalf("artifacts: %v", as)
	}
	v := call(t, cs, "harbor_get_vulnerabilities", map[string]any{"project": project, "repository": "team/app", "reference": "1.1"})
	if v["scanned"] == true {
		t.Logf("this Harbor has a scanner; report: %v", v)
	} else if v["note"] == nil {
		t.Fatalf("unscanned artifact without a note: %v", v)
	}
	ms := call(t, cs, "harbor_list_members", map[string]any{"project": project})
	if ms["total"].(float64) < 1 {
		t.Fatalf("members: %v", ms)
	}
	for _, name := range []string{"harbor_list_robots", "harbor_list_registries", "harbor_list_replications", "harbor_get_gc"} {
		call(t, cs, name, nil)
	}
	call(t, cs, "harbor_list_webhooks", map[string]any{"project": project})

	if strings.Contains(stderr.String(), pass) {
		t.Fatal("the password appears in the server's log")
	}
}
