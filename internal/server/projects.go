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

package server

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

func (t *tools) registerProjects(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "harbor_list_projects",
		Description: "List Harbor projects the server's user can see, optionally only those whose name contains query.",
		Annotations: readOnly("List projects"),
	}, t.listProjects)
	mcp.AddTool(s, &mcp.Tool{
		Name: "harbor_get_project",
		Description: "One project in detail: visibility, storage quota and usage, vulnerability and signing policies, " +
			"CVE allowlist, and whether retention and immutability rules are set.",
		Annotations: readOnly("Get a project"),
	}, t.getProject)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "harbor_list_repositories",
		Description: "List the repositories of a project, optionally only those whose name contains query.",
		Annotations: readOnly("List repositories"),
	}, t.listRepositories)
	mcp.AddTool(s, &mcp.Tool{
		Name: "harbor_list_artifacts",
		Description: "List the artifacts (images, charts) of a repository, newest push first, with tags, size, " +
			"labels and the result of the last vulnerability scan.",
		Annotations: readOnly("List artifacts"),
	}, t.listArtifacts)
	mcp.AddTool(s, &mcp.Tool{
		Name: "harbor_get_vulnerabilities",
		Description: "The vulnerabilities found in one artifact (by tag or digest), most severe first, " +
			"at or above minSeverity (default High).",
		Annotations: readOnly("Get vulnerabilities"),
	}, t.getVulnerabilities)
}

// ---- projects ----

// ListProjectsArgs are the arguments of harbor_list_projects.
type ListProjectsArgs struct {
	Query string `json:"query,omitempty" jsonschema:"only projects whose name contains this"`
	PageArgs
}

// ProjectSummary is one project in a list.
type ProjectSummary struct {
	Name            string `json:"name"`
	ID              int64  `json:"id"`
	Public          bool   `json:"public"`
	RepositoryCount int64  `json:"repositoryCount"`
	AutoScan        bool   `json:"autoScan" jsonschema:"whether pushed images are scanned automatically"`
	ProxyCache      bool   `json:"proxyCache" jsonschema:"whether the project is a proxy cache of another registry"`
	Created         string `json:"created,omitempty"`
}

func projectSummary(p harbor.Project) ProjectSummary {
	s := ProjectSummary{Name: p.Name, ID: p.ProjectID, RepositoryCount: p.RepoCount, ProxyCache: p.RegistryID != 0, Created: ts(p.CreationTime)}
	if p.Metadata != nil {
		s.Public, s.AutoScan = on(p.Metadata.Public), on(p.Metadata.AutoScan)
	}
	return s
}

func (t *tools) listProjects(ctx context.Context, _ *mcp.CallToolRequest, in ListProjectsArgs) (*mcp.CallToolResult, Page[ProjectSummary], error) {
	o := in.options()
	ps, total, err := t.hc.ListProjects(ctx, in.Query, o)
	if err != nil {
		return nil, Page[ProjectSummary]{}, t.fail("list projects", err)
	}
	return nil, pageOf(mapAll(ps, projectSummary), total, o), nil
}

// ProjectArgs name a project.
type ProjectArgs struct {
	Project string `json:"project" jsonschema:"project name"`
}

// Quota is a project's storage quota in bytes; LimitBytes -1 means unlimited.
type Quota struct {
	UsedBytes  int64 `json:"usedBytes"`
	LimitBytes int64 `json:"limitBytes" jsonschema:"-1 means unlimited"`
}

// ProjectDetail is the result of harbor_get_project.
type ProjectDetail struct {
	ProjectSummary
	Owner   string `json:"owner,omitempty"`
	Updated string `json:"updated,omitempty"`
	Quota   *Quota `json:"quota,omitempty"`
	// PreventVulnerable blocks pulls of images with vulnerabilities at or above Severity.
	PreventVulnerable bool     `json:"preventVulnerable" jsonschema:"whether pulls of vulnerable images are blocked"`
	Severity          string   `json:"severity,omitempty" jsonschema:"the severity at which pulls are blocked"`
	ContentTrust      bool     `json:"contentTrust" jsonschema:"whether only signed images can be pulled"`
	CVEAllowlist      []string `json:"cveAllowlist" jsonschema:"CVE IDs ignored by the vulnerability policy"`
	RetentionPolicy   bool     `json:"retentionPolicy" jsonschema:"whether a tag retention policy is set"`
	ImmutableRules    int      `json:"immutableRules" jsonschema:"number of immutable tag rules"`
}

func (t *tools) getProject(ctx context.Context, _ *mcp.CallToolRequest, in ProjectArgs) (*mcp.CallToolResult, ProjectDetail, error) {
	p, err := t.hc.GetProjectByName(ctx, in.Project)
	if err != nil {
		return nil, ProjectDetail{}, t.fail("get project "+in.Project, err)
	}
	d := ProjectDetail{ProjectSummary: projectSummary(*p), Owner: p.OwnerName, Updated: ts(p.UpdateTime), CVEAllowlist: []string{}}
	if md := p.Metadata; md != nil {
		d.PreventVulnerable, d.Severity = on(md.PreventVul), md.Severity
		d.ContentTrust = on(md.EnableContentTrust) || on(md.EnableContentTrustCosign)
		d.RetentionPolicy = md.RetentionID != "" && md.RetentionID != "0"
	}
	if p.CVEAllowlist != nil {
		d.CVEAllowlist = mapAll(p.CVEAllowlist.Items, func(i harbor.CVEAllowlistItem) string { return i.CVEID })
	}
	q, err := t.hc.GetProjectQuota(ctx, p.ProjectID)
	if err != nil {
		return nil, ProjectDetail{}, t.fail("get the quota of project "+in.Project, err)
	}
	if q != nil {
		d.Quota = &Quota{UsedBytes: q.Used["storage"], LimitBytes: q.Hard["storage"]}
	}
	rules, err := t.hc.ListImmutableRules(ctx, p.ProjectID)
	if err != nil {
		return nil, ProjectDetail{}, t.fail("list the immutable rules of project "+in.Project, err)
	}
	d.ImmutableRules = len(rules)
	return nil, d, nil
}

// ---- repositories ----

// ListRepositoriesArgs are the arguments of harbor_list_repositories.
type ListRepositoriesArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	Query   string `json:"query,omitempty" jsonschema:"only repositories whose name contains this"`
	PageArgs
}

// RepositorySummary is one repository in a list.
type RepositorySummary struct {
	Name          string `json:"name" jsonschema:"full name including the project, for example library/nginx"`
	ArtifactCount int64  `json:"artifactCount"`
	PullCount     int64  `json:"pullCount"`
	Updated       string `json:"updated,omitempty"`
}

func (t *tools) listRepositories(ctx context.Context, _ *mcp.CallToolRequest, in ListRepositoriesArgs) (*mcp.CallToolResult, Page[RepositorySummary], error) {
	o := in.options()
	rs, total, err := t.hc.ListRepositories(ctx, in.Project, in.Query, o)
	if err != nil {
		return nil, Page[RepositorySummary]{}, t.fail("list the repositories of project "+in.Project, err)
	}
	return nil, pageOf(mapAll(rs, func(r harbor.Repository) RepositorySummary {
		return RepositorySummary{Name: r.Name, ArtifactCount: r.ArtifactCount, PullCount: r.PullCount, Updated: ts(r.UpdateTime)}
	}), total, o), nil
}

// ---- artifacts ----

// ListArtifactsArgs are the arguments of harbor_list_artifacts.
type ListArtifactsArgs struct {
	Project    string `json:"project" jsonschema:"project name"`
	Repository string `json:"repository" jsonschema:"repository name, with or without the project prefix"`
	PageArgs
}

// ScanResult is the outcome of the last vulnerability scan of an artifact.
type ScanResult struct {
	Status     string           `json:"status" jsonschema:"Success, Running, Pending, Error or Stopped"`
	Severity   string           `json:"severity,omitempty" jsonschema:"the highest severity found"`
	Total      int64            `json:"total"`
	Fixable    int64            `json:"fixable"`
	BySeverity map[string]int64 `json:"bySeverity,omitempty"`
	Scanner    string           `json:"scanner,omitempty"`
	Finished   string           `json:"finished,omitempty"`
}

// ArtifactSummary is one artifact in a list.
type ArtifactSummary struct {
	Digest    string      `json:"digest"`
	Tags      []string    `json:"tags"`
	Type      string      `json:"type" jsonschema:"IMAGE, CHART, CNAB and so on"`
	SizeBytes int64       `json:"sizeBytes"`
	Pushed    string      `json:"pushed,omitempty"`
	Pulled    string      `json:"pulled,omitempty" jsonschema:"empty when never pulled"`
	Labels    []string    `json:"labels"`
	Scan      *ScanResult `json:"scan,omitempty" jsonschema:"absent when never scanned"`
}

func artifactSummary(a harbor.Artifact) ArtifactSummary {
	s := ArtifactSummary{
		Digest: a.Digest, Type: a.Type, SizeBytes: a.Size, Pushed: ts(a.PushTime), Pulled: ts(a.PullTime),
		Tags:   mapAll(a.Tags, func(t harbor.Tag) string { return t.Name }),
		Labels: mapAll(a.Labels, func(l harbor.Label) string { return l.Name }),
	}
	if sc := a.Scan(); sc != nil {
		r := &ScanResult{Status: sc.ScanStatus, Severity: sc.Severity, Finished: ts(sc.EndTime)}
		if sc.Summary != nil {
			r.Total, r.Fixable, r.BySeverity = sc.Summary.Total, sc.Summary.Fixable, sc.Summary.Summary
		}
		if sc.Scanner != nil {
			r.Scanner = strings.TrimSpace(sc.Scanner.Name + " " + sc.Scanner.Version)
		}
		s.Scan = r
	}
	return s
}

func (t *tools) listArtifacts(ctx context.Context, _ *mcp.CallToolRequest, in ListArtifactsArgs) (*mcp.CallToolResult, Page[ArtifactSummary], error) {
	o := in.options()
	as, total, err := t.hc.ListArtifacts(ctx, in.Project, in.Repository, o)
	if err != nil {
		return nil, Page[ArtifactSummary]{}, t.fail("list the artifacts of "+in.Project+"/"+in.Repository, err)
	}
	return nil, pageOf(mapAll(as, artifactSummary), total, o), nil
}

// ---- vulnerabilities ----

// severityRank orders Harbor's severities; unknown values rank lowest.
var severityRank = map[string]int{"Critical": 5, "High": 4, "Medium": 3, "Low": 2, "Negligible": 1, "Unknown": 0}

// maxVulnerabilities caps the entries returned by harbor_get_vulnerabilities.
const maxVulnerabilities = 100

// GetVulnerabilitiesArgs are the arguments of harbor_get_vulnerabilities.
type GetVulnerabilitiesArgs struct {
	Project     string `json:"project" jsonschema:"project name"`
	Repository  string `json:"repository" jsonschema:"repository name, with or without the project prefix"`
	Reference   string `json:"reference" jsonschema:"tag or digest (sha256:...)"`
	MinSeverity string `json:"minSeverity,omitempty" jsonschema:"Critical, High (default), Medium, Low, Negligible or Unknown"`
	Limit       int    `json:"limit,omitempty" jsonschema:"most entries to return (default and maximum 100)"`
}

// Vulnerability is one finding.
type Vulnerability struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Package     string `json:"package"`
	Version     string `json:"version"`
	FixVersion  string `json:"fixVersion,omitempty" jsonschema:"empty when no fix is known"`
	Description string `json:"description,omitempty"`
	Link        string `json:"link,omitempty"`
}

// VulnerabilityReport is the result of harbor_get_vulnerabilities.
type VulnerabilityReport struct {
	Scanned         bool             `json:"scanned" jsonschema:"false when the artifact has no report"`
	Note            string           `json:"note,omitempty"`
	Scanner         string           `json:"scanner,omitempty"`
	Generated       string           `json:"generated,omitempty"`
	Severity        string           `json:"severity,omitempty" jsonschema:"the highest severity found"`
	BySeverity      map[string]int64 `json:"bySeverity,omitempty" jsonschema:"all findings by severity"`
	Matching        int              `json:"matching" jsonschema:"findings at or above minSeverity"`
	Vulnerabilities []Vulnerability  `json:"vulnerabilities" jsonschema:"the most severe matching findings, up to limit"`
}

func (t *tools) getVulnerabilities(ctx context.Context, _ *mcp.CallToolRequest, in GetVulnerabilitiesArgs) (*mcp.CallToolResult, VulnerabilityReport, error) {
	minSev := cmp.Or(in.MinSeverity, "High")
	minRank, ok := severityRank[minSev]
	if !ok {
		return nil, VulnerabilityReport{}, fmt.Errorf("minSeverity %q: use Critical, High, Medium, Low, Negligible or Unknown", in.MinSeverity)
	}
	limit := in.Limit
	if limit < 1 || limit > maxVulnerabilities {
		limit = maxVulnerabilities
	}
	where := in.Project + "/" + strings.TrimPrefix(in.Repository, in.Project+"/") + ":" + in.Reference
	rep, err := t.hc.GetVulnerabilityReport(ctx, in.Project, in.Repository, in.Reference)
	if apiErr, isAPI := asAPIError(err); isAPI && strings.Contains(apiErr.Message, "no scanner registration") {
		return nil, VulnerabilityReport{Vulnerabilities: []Vulnerability{},
			Note: "Harbor has no vulnerability scanner configured for this project, so " + where + " was never scanned."}, nil
	}
	if err != nil {
		return nil, VulnerabilityReport{}, t.fail("get the vulnerabilities of "+where, err)
	}
	if rep == nil {
		return nil, VulnerabilityReport{Vulnerabilities: []Vulnerability{}, Note: where + " has not been scanned."}, nil
	}
	out := VulnerabilityReport{Scanned: true, Generated: ts(rep.GeneratedAt), Severity: rep.Severity, BySeverity: map[string]int64{}}
	if rep.Scanner != nil {
		out.Scanner = strings.TrimSpace(rep.Scanner.Name + " " + rep.Scanner.Version)
	}
	var matching []harbor.Vulnerability
	for _, v := range rep.Vulnerabilities {
		out.BySeverity[v.Severity]++
		if severityRank[v.Severity] >= minRank {
			matching = append(matching, v)
		}
	}
	slices.SortStableFunc(matching, func(a, b harbor.Vulnerability) int {
		return cmp.Or(cmp.Compare(severityRank[b.Severity], severityRank[a.Severity]), cmp.Compare(a.ID, b.ID))
	})
	out.Matching = len(matching)
	out.Vulnerabilities = mapAll(matching[:min(limit, len(matching))], func(v harbor.Vulnerability) Vulnerability {
		r := Vulnerability{ID: v.ID, Severity: v.Severity, Package: v.Package, Version: v.Version, FixVersion: v.FixVersion,
			Description: clip(v.Description, 300)}
		if len(v.Links) > 0 {
			r.Link = v.Links[0]
		}
		return r
	})
	return nil, out, nil
}

// on reads Harbor's "true"/"false" metadata strings.
func on(s string) bool { return s == "true" }

// clip shortens s to at most n runes.
func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
