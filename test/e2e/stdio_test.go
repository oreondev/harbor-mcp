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

// binary builds the server and returns its path; it skips the test when no
// Harbor is configured.
func binary(t *testing.T) string {
	t.Helper()
	if harborURL() == "" {
		t.Skip("HARBOR_URL not set")
	}
	bin := filepath.Join(t.TempDir(), "harbor-mcp")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=e2e", "-o", bin, "../../cmd/harbor-mcp")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// start builds the binary, runs it over stdio and returns a session and the
// server's stderr. env is added to the test's environment.
func start(t *testing.T, env ...string) (*mcp.ClientSession, *strings.Builder) {
	t.Helper()
	cmd := exec.Command(binary(t))
	cmd.Env = append(os.Environ(), env...)
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
	robot, err := hc.CreateRobot(ctx, harbor.RobotCreate{Name: "e2e-pull", Level: "project", Duration: 1,
		Permissions: []harbor.RobotPermission{{Kind: "project", Namespace: project,
			Access: []harbor.Access{{Resource: "repository", Action: "pull"}}}}})
	if err != nil {
		t.Fatalf("create robot: %v", err)
	}
	t.Cleanup(func() { _ = hc.DeleteRobot(ctx, robot.ID) })
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
	// Harbor lists project robots only per project; both forms must find it.
	// System robots covering every project may be listed too.
	for _, args := range []map[string]any{{"project": project}, {"query": "e2e-pull"}} {
		rb := call(t, cs, "harbor_list_robots", args)
		found := false
		for _, r := range items(rb) {
			found = found || r["name"] == robot.Name && r["expires"] != ""
		}
		if !found {
			t.Fatalf("robots %v: %v", args, rb)
		}
		if out, _ := json.Marshal(rb); strings.Contains(string(out), robot.Secret) {
			t.Fatal("the robot secret appears in harbor_list_robots")
		}
	}
	for _, name := range []string{"harbor_list_registries", "harbor_list_replications", "harbor_get_gc"} {
		call(t, cs, name, nil)
	}
	call(t, cs, "harbor_list_webhooks", map[string]any{"project": project})

	if strings.Contains(stderr.String(), pass) || strings.Contains(stderr.String(), robot.Secret) {
		t.Fatal("a secret appears in the server's log")
	}

	readOnlyRobot(t, hc)
}

// readOnlyRobot creates the system robot of docs/read-only-robot.json and
// calls every tool as it, so the documented permissions are tested on every
// Harbor version.
func readOnlyRobot(t *testing.T, hc *harbor.Client) {
	t.Helper()
	ctx := context.Background()
	data, err := os.ReadFile("../../docs/read-only-robot.json")
	if err != nil {
		t.Fatal(err)
	}
	var req harbor.RobotCreate
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("docs/read-only-robot.json: %v", err)
	}
	req.Name, req.Duration = "harbor-mcp-e2e-reader", 1
	robot, err := hc.CreateRobot(ctx, req)
	if err != nil {
		t.Fatalf("create the read-only robot: %v", err)
	}
	t.Cleanup(func() { _ = hc.DeleteRobot(ctx, robot.ID) })

	cs, stderr := start(t, "HARBOR_USERNAME="+robot.Name, "HARBOR_PASSWORD="+robot.Secret)
	if info := call(t, cs, "harbor_system_info", nil); info["user"] != robot.Name || info["robot"] != true {
		t.Fatalf("system info as the robot: %v", info)
	}
	for name, args := range map[string]map[string]any{
		"harbor_list_projects":       nil,
		"harbor_get_project":         {"project": project},
		"harbor_list_repositories":   {"project": project},
		"harbor_list_artifacts":      {"project": project, "repository": "team/app"},
		"harbor_get_vulnerabilities": {"project": project, "repository": "team/app", "reference": "1.1"},
		"harbor_list_members":        {"project": project},
		"harbor_list_robots":         {"project": project},
		"harbor_list_registries":     nil,
		"harbor_list_replications":   nil,
		"harbor_list_webhooks":       {"project": project},
		"harbor_get_gc":              nil,
	} {
		out, _ := json.Marshal(call(t, cs, name, args))
		if strings.Contains(string(out), robot.Secret) {
			t.Fatalf("%s: the robot secret appears in the result", name)
		}
	}
	if strings.Contains(stderr.String(), robot.Secret) {
		t.Fatal("the robot secret appears in the server's log")
	}
}
