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

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor/fake"

	"github.com/oreondev/harbor-mcp/internal/server"
)

const (
	robotSecret    = "s3cret-robot-value"
	registrySecret = "s3cret-registry-value"
	webhookSecret  = "Bearer s3cret-webhook-value"
)

// seeded is a fake Harbor with one of everything the tools read.
func seeded(t *testing.T) *fake.Server {
	t.Helper()
	ctx := context.Background()
	srv := newHarbor(t)
	hc, err := harbor.New(harbor.Options{URL: srv.URL(), Username: adminUser, Password: adminPass})
	if err != nil {
		t.Fatal(err)
	}
	pid := srv.AddProject("library", true)
	srv.AddProject("team-a", false)
	srv.Locked(func() {
		p := srv.Projects[pid]
		p.Metadata.AutoScan, p.Metadata.PreventVul, p.Metadata.Severity, p.Metadata.RetentionID = "true", "true", "high", "7"
		p.CVEAllowlist = &harbor.CVEAllowlist{Items: []harbor.CVEAllowlistItem{{CVEID: "CVE-2026-0002"}}}
	})
	q, err := hc.GetProjectQuota(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	must(t, hc.UpdateQuota(ctx, q.ID, map[string]int64{"storage": 1 << 30}))
	if _, err := hc.CreateImmutableRule(ctx, pid, harbor.TagRule{Action: "immutable", Template: "immutable_template",
		TagSelectors:   []harbor.TagSelector{{Kind: "doublestar", Decoration: "matches", Pattern: "v*"}},
		ScopeSelectors: map[string][]harbor.TagSelector{"repository": {{Kind: "doublestar", Decoration: "repoMatches", Pattern: "**"}}}}); err != nil {
		t.Fatal(err)
	}

	srv.AddRepository("library", "nginx")
	srv.AddRepository("library", "team/app")
	scan := map[string]harbor.ScanSummary{"application/vnd.security.vulnerability.report; version=1.1": {
		ScanStatus: "Success", Severity: "Critical", Scanner: &harbor.Scanner{Name: "Trivy", Version: "v0.60.0"},
		Summary: &harbor.VulnerabilitySummary{Total: 4, Fixable: 3, Summary: map[string]int64{"Critical": 1, "High": 2, "Low": 1}},
	}}
	report := &harbor.VulnerabilityReport{Severity: "Critical", Scanner: &harbor.Scanner{Name: "Trivy", Version: "v0.60.0"},
		Vulnerabilities: []harbor.Vulnerability{
			{ID: "CVE-2026-0003", Package: "zlib", Version: "1.3", Severity: "High"},
			{ID: "CVE-2026-0004", Package: "curl", Version: "8.0", Severity: "Low"},
			{ID: "CVE-2026-0001", Package: "openssl", Version: "3.0.0", FixVersion: "3.0.1", Severity: "Critical",
				Description: strings.Repeat("x", 500), Links: []string{"https://avd.aquasec.com/nvd/cve-2026-0001"}},
			{ID: "CVE-2026-0002", Package: "glibc", Version: "2.36", Severity: "High"},
		}}
	pushed := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	srv.AddArtifact("library", "team/app", harbor.Artifact{Digest: "sha256:aaa", Type: "IMAGE", Size: 1234, PushTime: pushed,
		Tags: []harbor.Tag{{Name: "1.0"}}}, nil)
	srv.AddArtifact("library", "team/app", harbor.Artifact{Digest: "sha256:bbb", Type: "IMAGE", Size: 5678, PushTime: pushed.Add(time.Hour),
		Tags: []harbor.Tag{{Name: "1.1"}, {Name: "latest"}}, Labels: []harbor.Label{{Name: "release"}}, ScanOverview: scan}, report)

	srv.AddRobot(harbor.Robot{Name: "robot$library+ci", Level: "project", ExpiresAt: time.Now().Add(-time.Hour).Unix(),
		Permissions: []harbor.RobotPermission{{Kind: "project", Namespace: "library",
			Access: []harbor.Access{{Resource: "repository", Action: "pull"}, {Resource: "repository", Action: "push"}}}}}, robotSecret)
	srv.AddRobot(harbor.Robot{Name: "robot$team-a+reader", Level: "project", ExpiresAt: -1,
		Permissions: []harbor.RobotPermission{{Kind: "project", Namespace: "team-a",
			Access: []harbor.Access{{Resource: "repository", Action: "pull"}}}}}, robotSecret)

	regID := srv.AddRegistry(harbor.Registry{Name: "edge", Type: "harbor", URL: "https://edge.example.com",
		Credential: &harbor.RegistryCredential{Type: "basic", AccessKey: "edge-user", AccessSecret: registrySecret}})
	polID, err := hc.CreateReplicationPolicy(ctx, harbor.ReplicationPolicy{Name: "to-edge", Enabled: true,
		DestRegistry: &harbor.Registry{ID: regID}, Trigger: &harbor.ReplicationTrigger{Type: "manual"}})
	if err != nil {
		t.Fatal(err)
	}
	must(t, hc.StartReplication(ctx, polID))

	if _, err := hc.CreateWebhookPolicy(ctx, pid, harbor.WebhookPolicy{Name: "siem", Enabled: true, EventTypes: []string{"PUSH_ARTIFACT"},
		Targets: []harbor.WebhookTarget{{Type: "http", Address: "https://siem.example.com/hook", AuthHeader: webhookSecret}}}); err != nil {
		t.Fatal(err)
	}
	srv.AddRepository("team-a", "review")
	pushSkill(t, srv, "team-a", "review", "review", "1.0.0", skillLayer(t, "review", map[string]string{"SKILL.md": reviewMD}), "1.0.0")
	srv.AddGCRun("Success")
	must(t, hc.CreateGCSchedule(ctx, harbor.GCSchedule{Schedule: &harbor.ScheduleObj{Type: "Daily", Cron: "0 0 0 * * *"}}))
	return srv
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// get calls a tool, fails the test on a tool error, and decodes the result.
func get[T any](t *testing.T, srv *fake.Server, name string, args map[string]any) T {
	t.Helper()
	cs := connect(t, srv.URL(), adminUser, adminPass)
	var out T
	if res := call(t, cs, name, args, &out); res.IsError {
		t.Fatalf("%s: tool error: %s", name, text(res))
	}
	return out
}

func TestProjectTools(t *testing.T) {
	srv := seeded(t)
	pg := get[server.Page[server.ProjectSummary]](t, srv, "harbor_list_projects", map[string]any{"pageSize": 1})
	if pg.Total != 2 || len(pg.Items) != 1 || pg.Items[0].Name != "library" || !pg.Items[0].Public || !pg.Items[0].AutoScan {
		t.Fatalf("list projects: %+v", pg)
	}
	pg = get[server.Page[server.ProjectSummary]](t, srv, "harbor_list_projects", map[string]any{"query": "team"})
	if pg.Total != 1 || pg.Items[0].Name != "team-a" {
		t.Fatalf("list projects by query: %+v", pg)
	}

	d := get[server.ProjectDetail](t, srv, "harbor_get_project", map[string]any{"project": "library"})
	if d.Quota == nil || d.Quota.LimitBytes != 1<<30 || !d.PreventVulnerable || d.Severity != "high" || !d.RetentionPolicy ||
		d.ImmutableRules != 1 || len(d.CVEAllowlist) != 1 || d.RepositoryCount != 2 {
		t.Fatalf("get project: %+v", d)
	}
	cs := connect(t, srv.URL(), adminUser, adminPass)
	if res := call(t, cs, "harbor_get_project", map[string]any{"project": "nope"}, nil); !res.IsError ||
		!strings.Contains(text(res), "404") {
		t.Fatalf("missing project: want a 404 tool error, got %s", text(res))
	}
	if res := call(t, cs, "harbor_get_project", map[string]any{}, nil); !res.IsError {
		t.Fatalf("missing required argument should be rejected: %s", text(res))
	}

	rs := get[server.Page[server.RepositorySummary]](t, srv, "harbor_list_repositories", map[string]any{"project": "library"})
	if rs.Total != 2 || rs.Items[1].Name != "library/team/app" || rs.Items[1].ArtifactCount != 2 {
		t.Fatalf("list repositories: %+v", rs)
	}

	as := get[server.Page[server.ArtifactSummary]](t, srv, "harbor_list_artifacts",
		map[string]any{"project": "library", "repository": "library/team/app"})
	if as.Total != 2 {
		t.Fatalf("list artifacts: %+v", as)
	}
	top := as.Items[0]
	if top.Digest != "sha256:bbb" || strings.Join(top.Tags, ",") != "1.1,latest" || top.Labels[0] != "release" ||
		top.Scan == nil || top.Scan.Total != 4 || top.Scan.BySeverity["High"] != 2 || top.Scan.Scanner != "Trivy v0.60.0" ||
		top.Pushed != "2026-11-01T13:00:00Z" || top.Pulled != "" {
		t.Fatalf("newest artifact: %+v", top)
	}
	if as.Items[1].Scan != nil || len(as.Items[1].Labels) != 0 {
		t.Fatalf("unscanned artifact: %+v", as.Items[1])
	}
}

func TestVulnerabilities(t *testing.T) {
	srv := seeded(t)
	args := func(extra map[string]any) map[string]any {
		m := map[string]any{"project": "library", "repository": "team/app", "reference": "1.1"}
		maps.Copy(m, extra)
		return m
	}
	r := get[server.VulnerabilityReport](t, srv, "harbor_get_vulnerabilities", args(nil))
	ids := make([]string, 0, len(r.Vulnerabilities))
	for _, v := range r.Vulnerabilities {
		ids = append(ids, v.ID)
	}
	if !r.Scanned || r.Matching != 3 || strings.Join(ids, ",") != "CVE-2026-0001,CVE-2026-0002,CVE-2026-0003" ||
		r.BySeverity["Low"] != 1 || r.Scanner != "Trivy v0.60.0" {
		t.Fatalf("default (High and above, most severe first): %+v", r)
	}
	first := r.Vulnerabilities[0]
	if first.FixVersion != "3.0.1" || first.Link == "" || len([]rune(first.Description)) != 300 {
		t.Fatalf("first finding: fix %q, link %q, description %d runes", first.FixVersion, first.Link, len([]rune(first.Description)))
	}
	r = get[server.VulnerabilityReport](t, srv, "harbor_get_vulnerabilities", args(map[string]any{"minSeverity": "Low", "limit": 1}))
	if r.Matching != 4 || len(r.Vulnerabilities) != 1 {
		t.Fatalf("Low with limit 1: %+v", r)
	}
	r = get[server.VulnerabilityReport](t, srv, "harbor_get_vulnerabilities", map[string]any{"project": "library", "repository": "team/app", "reference": "1.0"})
	if r.Scanned || !strings.Contains(r.Note, "not been scanned") {
		t.Fatalf("unscanned: %+v", r)
	}
	srv.Locked(func() { srv.NoScanner = true })
	r = get[server.VulnerabilityReport](t, srv, "harbor_get_vulnerabilities", args(nil))
	if r.Scanned || !strings.Contains(r.Note, "no vulnerability scanner") {
		t.Fatalf("no scanner: %+v", r)
	}
	cs := connect(t, srv.URL(), adminUser, adminPass)
	if res := call(t, cs, "harbor_get_vulnerabilities", args(map[string]any{"minSeverity": "severe"}), nil); !res.IsError ||
		!strings.Contains(text(res), "Critical, High") {
		t.Fatalf("bad minSeverity: %s", text(res))
	}
}

func TestAccessAndReplicationTools(t *testing.T) {
	srv := seeded(t)
	rb := get[server.Page[server.RobotSummary]](t, srv, "harbor_list_robots", map[string]any{"project": "library"})
	if rb.Total != 1 || rb.Items[0].Name != "robot$library+ci" || !rb.Items[0].Expired ||
		strings.Join(rb.Items[0].Permissions[0].Access, ",") != "repository:pull,repository:push" {
		t.Fatalf("robots of library: %+v", rb)
	}
	rb = get[server.Page[server.RobotSummary]](t, srv, "harbor_list_robots", map[string]any{"query": "reader"})
	if rb.Total != 1 || rb.Items[0].Expires != "" || rb.Items[0].Expired {
		t.Fatalf("robot that never expires: %+v", rb)
	}

	ms := get[server.Page[server.Member]](t, srv, "harbor_list_members", map[string]any{"project": "library"})
	if ms.Total != 1 || ms.Items[0] != (server.Member{Name: adminUser, Type: "user", Role: "projectAdmin"}) {
		t.Fatalf("members: %+v", ms)
	}

	regs := get[server.Page[server.RegistrySummary]](t, srv, "harbor_list_registries", nil)
	if regs.Total != 1 || regs.Items[0].Name != "edge" || regs.Items[0].Status != "healthy" {
		t.Fatalf("registries: %+v", regs)
	}
	reps := get[server.Page[server.ReplicationSummary]](t, srv, "harbor_list_replications", nil)
	if reps.Total != 1 || reps.Items[0].Direction != "push" || reps.Items[0].Remote != "edge" || reps.Items[0].LastRun == nil ||
		reps.Items[0].Trigger != "manual" {
		t.Fatalf("replications: %+v", reps)
	}

	wh := get[server.Page[server.Webhook]](t, srv, "harbor_list_webhooks", map[string]any{"project": "library"})
	if wh.Total != 1 || !wh.Items[0].Targets[0].AuthHeaderSet || wh.Items[0].Events[0] != "PUSH_ARTIFACT" {
		t.Fatalf("webhooks: %+v", wh)
	}
	gc := get[server.GCStatus](t, srv, "harbor_get_gc", nil)
	if !gc.Scheduled || gc.Schedule != "Daily" || gc.LastRun == nil || gc.LastRun.Status != "Success" {
		t.Fatalf("gc: %+v", gc)
	}
}

func TestPaging(t *testing.T) {
	srv := newHarbor(t)
	for i := range 7 {
		srv.AddRobot(harbor.Robot{Name: fmt.Sprintf("robot$r%d", i), Level: "system"}, robotSecret)
	}
	p := get[server.Page[server.RobotSummary]](t, srv, "harbor_list_robots", map[string]any{"page": 2, "pageSize": 5})
	if p.Total != 7 || len(p.Items) != 2 || p.Page != 2 || p.PageSize != 5 || p.Items[0].Name != "robot$r5" {
		t.Fatalf("page 2 of 7 by 5: %+v", p)
	}
	p = get[server.Page[server.RobotSummary]](t, srv, "harbor_list_robots", map[string]any{"page": 9, "pageSize": 500})
	if p.Total != 7 || len(p.Items) != 0 || p.PageSize != 100 {
		t.Fatalf("past the end, capped page size: %+v", p)
	}
}

// TestNoSecretsInAnyResult calls every tool and checks that no secret
// appears anywhere in the result, structured or text.
func TestNoSecretsInAnyResult(t *testing.T) {
	srv := seeded(t)
	cs := connect(t, srv.URL(), adminUser, adminPass)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"project": "library", "repository": "team/app", "reference": "1.1", "minSeverity": "Unknown"}
	perTool := map[string]map[string]any{
		"harbor_list_skills": {"project": "team-a"},
		"harbor_get_skill":   {"project": "team-a", "repository": "review", "reference": "1.0.0"},
	}
	for _, tool := range tools.Tools {
		props := map[string]any{}
		args := maps.Clone(args)
		maps.Copy(args, perTool[tool.Name])
		if s, isMap := tool.InputSchema.(map[string]any); isMap {
			if p, hasProps := s["properties"].(map[string]any); hasProps {
				for k := range p {
					if v, known := args[k]; known {
						props[k] = v
					}
				}
			}
		}
		res := call(t, cs, tool.Name, props, nil)
		raw, _ := json.Marshal(res)
		for _, secret := range []string{adminPass, robotSecret, registrySecret, "s3cret-webhook-value", httpToken} {
			if strings.Contains(string(raw), secret) {
				t.Errorf("%s leaks a secret: %s", tool.Name, raw)
			}
		}
		if res.IsError {
			t.Errorf("%s: unexpected tool error: %s", tool.Name, text(res))
		}
	}
}
