//go:build e2e

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

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

const skillsProject = "harbor-mcp-e2e-skills"

// callErr invokes a tool that must fail and returns its error text.
func callErr(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	raw, _ := json.Marshal(res.Content)
	if !res.IsError {
		t.Fatalf("%s %v: want a tool error, got %s", name, args, raw)
	}
	t.Logf("%s: error %s", name, clip(string(raw), 400))
	return string(raw)
}

// TestSkills pushes a signed and an unsigned version of a skill and lists,
// reads and installs them as the documented read-only robot with pull on the
// skills project, with the project's cosign policy off and on.
func TestSkills(t *testing.T) {
	bin := binary(t)
	user, pass := os.Getenv("HARBOR_USERNAME"), os.Getenv("HARBOR_PASSWORD")
	ctx := context.Background()
	hc, err := harbor.New(harbor.Options{URL: harborURL(), Username: user, Password: pass})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hc.GetProjectByName(ctx, skillsProject); harbor.IsNotFound(err) {
		if _, err := hc.CreateProject(ctx, harbor.ProjectReq{ProjectName: skillsProject}); err != nil {
			t.Fatalf("create project: %v", err)
		}
	}
	p, err := hc.GetProjectByName(ctx, skillsProject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, repo := range []string{"review", "app"} {
			if err := deleteRepository(user, pass, skillsProject, repo); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
		_ = hc.DeleteProject(ctx, p.ProjectID)
	})

	repo := skillsProject + "/review"
	skillMD := "---\nname: review\ndescription: Reviews pull requests.\n---\n\nRead the diff first.\n"
	if _, err := pushSkill(user, pass, repo, "0.9.0", "review", map[string]string{"SKILL.md": "old"}); err != nil {
		t.Fatalf("push 0.9.0: %v", err)
	}
	signed, err := pushSkill(user, pass, repo, "1.0.0", "review",
		map[string]string{"SKILL.md": skillMD, "scripts/diff.sh": "git diff\n"})
	if err != nil {
		t.Fatalf("push 1.0.0: %v", err)
	}
	if err := pushCosignSignature(user, pass, repo, signed); err != nil {
		t.Fatalf("sign 1.0.0: %v", err)
	}
	// An image next to the skill is left out of the list.
	if err := pushTestImage(user, pass, skillsProject+"/app", "1.0"); err != nil {
		t.Fatalf("push image: %v", err)
	}

	reader := skillsRobot(t, hc, "harbor-mcp-e2e-skills", true)
	dir := filepath.Join(t.TempDir(), "skills")
	cs, stderr := startBinary(t, bin, "HARBOR_USERNAME="+reader.Name, "HARBOR_PASSWORD="+reader.Secret,
		"HARBOR_MCP_SKILLS_DIR="+dir)

	page := call(t, cs, "harbor_list_skills", map[string]any{"project": skillsProject})
	if len(items(page)) != 1 {
		t.Fatalf("skills: %v", page)
	}
	s := items(page)[0]
	if s["name"] != "review" || s["version"] != "1.0.0" || s["digest"] != signed || s["signed"] != true ||
		s["versions"].(float64) != 2 {
		t.Fatalf("skill: %v", s)
	}
	got := call(t, cs, "harbor_get_skill", map[string]any{"project": skillsProject, "repository": "review"})
	if got["content"] != skillMD || got["signed"] != true || len(got["files"].([]any)) != 2 {
		t.Fatalf("newest skill: %v", got)
	}
	if old := call(t, cs, "harbor_get_skill", map[string]any{"project": skillsProject, "repository": "review",
		"reference": "0.9.0"}); old["content"] != "old" || old["signed"] != false {
		t.Fatalf("0.9.0: %v", old)
	}

	in := call(t, cs, "harbor_install_skill", map[string]any{"project": skillsProject, "repository": "review"})
	if in["path"] != filepath.Join(dir, "review") || in["digest"] != signed {
		t.Fatalf("install: %v", in)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "review", "scripts", "diff.sh")); string(b) != "git diff\n" {
		t.Fatalf("installed script: %q", b)
	}
	if msg := callErr(t, cs, "harbor_install_skill", map[string]any{"project": skillsProject, "repository": "review",
		"reference": "0.9.0"}); !strings.Contains(msg, "HARBOR_MCP_SKILLS_REQUIRE_SIGNED") {
		t.Fatalf("unsigned install: %s", msg)
	}

	// With the cosign policy on, Harbor itself refuses the unsigned version.
	cosign := func(on string) {
		t.Helper()
		if err := hc.UpdateProject(ctx, p.ProjectID, harbor.ProjectReq{
			Metadata: &harbor.ProjectMetadata{EnableContentTrustCosign: on}}); err != nil {
			t.Fatalf("cosign policy %s: %v", on, err)
		}
	}
	cosign("true")
	t.Cleanup(func() { cosign("false") })
	if got := call(t, cs, "harbor_get_skill", map[string]any{"project": skillsProject, "repository": "review"}); got["digest"] != signed {
		t.Fatalf("signed skill under the policy: %v", got)
	}
	if msg := callErr(t, cs, "harbor_get_skill", map[string]any{"project": skillsProject, "repository": "review",
		"reference": "0.9.0"}); !strings.Contains(msg, "only allows signed artifacts") {
		t.Fatalf("unsigned skill under the policy: %s", msg)
	}

	// The documented robot alone can list skills but not read them.
	lister := skillsRobot(t, hc, "harbor-mcp-e2e-lister", false)
	cs2, stderr2 := startBinary(t, bin, "HARBOR_USERNAME="+lister.Name, "HARBOR_PASSWORD="+lister.Secret)
	call(t, cs2, "harbor_list_skills", map[string]any{"project": skillsProject})
	if msg := callErr(t, cs2, "harbor_get_skill", map[string]any{"project": skillsProject, "repository": "review"}); !strings.Contains(msg, "repository:pull") {
		t.Fatalf("read without pull: %s", msg)
	}

	for _, log := range []string{stderr.String(), stderr2.String()} {
		if strings.Contains(log, pass) || strings.Contains(log, reader.Secret) || strings.Contains(log, lister.Secret) {
			t.Fatal("a secret appears in the server's log")
		}
	}
}

// skillsRobot creates the system robot of docs/read-only-robot.json, with
// the repository:pull on the skills project that reading skills needs when
// pull is set.
func skillsRobot(t *testing.T, hc *harbor.Client, name string, pull bool) *harbor.RobotCreated {
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
	req.Name, req.Duration = name, 1
	if pull {
		req.Permissions = append(req.Permissions, harbor.RobotPermission{Kind: "project", Namespace: skillsProject,
			Access: []harbor.Access{{Resource: "repository", Action: "pull"}}})
	}
	robot, err := hc.CreateRobot(ctx, req)
	if err != nil {
		t.Fatalf("create robot %s: %v", name, err)
	}
	t.Cleanup(func() { _ = hc.DeleteRobot(ctx, robot.ID) })
	return robot
}
