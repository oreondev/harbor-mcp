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

package server

import (
	"context"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

func (t *tools) registerAccess(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "harbor_list_robots",
		Description: "List robot accounts with their permissions and expiry, optionally only those with access to project " +
			"or whose name contains query. Harbor lists project robots per project, so without project this makes one " +
			"Harbor request per project. Secrets are never shown.",
		Annotations: readOnly("List robot accounts"),
	}, t.listRobots)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "harbor_list_members",
		Description: "List the users and groups that are members of a project, with their roles.",
		Annotations: readOnly("List project members"),
	}, t.listMembers)
}

// ListRobotsArgs are the arguments of harbor_list_robots.
type ListRobotsArgs struct {
	Project string `json:"project,omitempty" jsonschema:"only robots with permissions in this project"`
	Query   string `json:"query,omitempty" jsonschema:"only robots whose name contains this"`
	PageArgs
}

// RobotPermission is what a robot may do in one namespace.
type RobotPermission struct {
	Kind      string   `json:"kind" jsonschema:"project or system"`
	Namespace string   `json:"namespace" jsonschema:"project name, or / for system-wide"`
	Access    []string `json:"access" jsonschema:"resource:action pairs, for example repository:pull"`
}

// RobotSummary is one robot account in a list.
type RobotSummary struct {
	Name        string            `json:"name" jsonschema:"full robot name, for example robot$ci"`
	Level       string            `json:"level" jsonschema:"system or project"`
	Description string            `json:"description,omitempty"`
	Disabled    bool              `json:"disabled"`
	Expires     string            `json:"expires,omitempty" jsonschema:"empty when it never expires"`
	Expired     bool              `json:"expired"`
	Permissions []RobotPermission `json:"permissions"`
}

func robotSummary(r harbor.Robot, now time.Time) RobotSummary {
	s := RobotSummary{Name: r.Name, Level: r.Level, Description: r.Description, Disabled: r.Disable,
		Permissions: mapAll(r.Permissions, func(p harbor.RobotPermission) RobotPermission {
			return RobotPermission{Kind: p.Kind, Namespace: p.Namespace,
				Access: mapAll(p.Access, func(a harbor.Access) string { return a.Resource + ":" + a.Action })}
		})}
	if r.ExpiresAt > 0 {
		exp := time.Unix(r.ExpiresAt, 0)
		s.Expires, s.Expired = ts(exp), exp.Before(now)
	}
	return s
}

func (t *tools) listRobots(ctx context.Context, _ *mcp.CallToolRequest, in ListRobotsArgs) (*mcp.CallToolResult, Page[RobotSummary], error) {
	all, err := t.robots(ctx, in.Project)
	if err != nil {
		return nil, Page[RobotSummary]{}, t.fail("list robot accounts", err)
	}
	now := time.Now()
	out := make([]RobotSummary, 0, len(all))
	for _, r := range all {
		if in.Query != "" && !strings.Contains(r.Name, in.Query) {
			continue
		}
		if in.Project != "" && !robotIn(r, in.Project) {
			continue
		}
		out = append(out, robotSummary(r, now))
	}
	return nil, localPage(out, in.PageArgs), nil
}

// robots returns the system robots and the project robots of project, or of
// every project when project is empty: Harbor's /robots leaves project robots
// out unless it is asked for one project.
func (t *tools) robots(ctx context.Context, project string) ([]harbor.Robot, error) {
	all, err := t.hc.ListRobots(ctx, "")
	if err != nil {
		return nil, err
	}
	var projects []harbor.Project
	if project != "" {
		p, err := t.hc.GetProjectByName(ctx, project)
		if err != nil {
			return nil, err
		}
		projects = []harbor.Project{*p}
	} else if projects, err = t.allProjects(ctx); err != nil {
		return nil, err
	}
	for _, p := range projects {
		rs, err := t.hc.ListProjectRobots(ctx, p.ProjectID, "")
		if err != nil {
			return nil, err
		}
		all = append(all, rs...)
	}
	return all, nil
}

// allProjects returns every project the server's identity can see.
func (t *tools) allProjects(ctx context.Context) ([]harbor.Project, error) {
	var all []harbor.Project
	for page := 1; ; page++ {
		ps, total, err := t.hc.ListProjects(ctx, "", harbor.PageOptions{Page: page, PageSize: 100})
		if err != nil {
			return nil, err
		}
		all = append(all, ps...)
		if len(ps) == 0 || int64(len(all)) >= total {
			return all, nil
		}
	}
}

// robotIn reports whether r has permissions in project (system robots with
// a "*" namespace cover every project).
func robotIn(r harbor.Robot, project string) bool {
	for _, p := range r.Permissions {
		if p.Kind == "project" && (p.Namespace == project || p.Namespace == "*") {
			return true
		}
	}
	return false
}

// MembersArgs are the arguments of harbor_list_members.
type MembersArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	PageArgs
}

// Member is one project member.
type Member struct {
	Name string `json:"name"`
	Type string `json:"type" jsonschema:"user or group"`
	Role string `json:"role" jsonschema:"projectAdmin, maintainer, developer, guest or limitedGuest"`
}

func (t *tools) listMembers(ctx context.Context, _ *mcp.CallToolRequest, in MembersArgs) (*mcp.CallToolResult, Page[Member], error) {
	p, err := t.hc.GetProjectByName(ctx, in.Project)
	if err != nil {
		return nil, Page[Member]{}, t.fail("get project "+in.Project, err)
	}
	ms, err := t.hc.ListProjectMembers(ctx, p.ProjectID, "")
	if err != nil {
		return nil, Page[Member]{}, t.fail("list the members of project "+in.Project, err)
	}
	return nil, localPage(mapAll(ms, func(m harbor.ProjectMemberEntity) Member {
		typ := "user"
		if m.EntityType == "g" {
			typ = "group"
		}
		return Member{Name: m.EntityName, Type: typ, Role: m.RoleName}
	}), in.PageArgs), nil
}
