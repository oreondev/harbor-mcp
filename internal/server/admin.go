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
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

func (t *tools) registerAdmin(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "harbor_list_webhooks",
		Description: "List the webhook policies of a project: events and target addresses. " +
			"Authorization headers are never shown, only whether one is set.",
		Annotations: readOnly("List webhooks"),
	}, t.listWebhooks)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "harbor_get_gc",
		Description: "The garbage collection schedule and the outcome of the last garbage collection run.",
		Annotations: readOnly("Get garbage collection"),
	}, t.getGC)
}

// WebhookTarget is where a webhook sends events.
type WebhookTarget struct {
	Type           string `json:"type" jsonschema:"http or slack"`
	Address        string `json:"address"`
	PayloadFormat  string `json:"payloadFormat,omitempty" jsonschema:"Default or CloudEvents"`
	SkipCertVerify bool   `json:"skipCertVerify"`
	AuthHeaderSet  bool   `json:"authHeaderSet" jsonschema:"whether an Authorization header is configured (never shown)"`
}

// Webhook is one webhook policy.
type Webhook struct {
	Name        string          `json:"name"`
	Enabled     bool            `json:"enabled"`
	Description string          `json:"description,omitempty"`
	Events      []string        `json:"events"`
	Targets     []WebhookTarget `json:"targets"`
}

func (t *tools) listWebhooks(ctx context.Context, _ *mcp.CallToolRequest, in MembersArgs) (*mcp.CallToolResult, Page[Webhook], error) {
	p, err := t.hc.GetProjectByName(ctx, in.Project)
	if err != nil {
		return nil, Page[Webhook]{}, t.fail("get project "+in.Project, err)
	}
	ws, err := t.hc.ListWebhookPolicies(ctx, p.ProjectID)
	if err != nil {
		return nil, Page[Webhook]{}, t.fail("list the webhooks of project "+in.Project, err)
	}
	return nil, localPage(mapAll(ws, func(w harbor.WebhookPolicy) Webhook {
		return Webhook{Name: w.Name, Enabled: w.Enabled, Description: w.Description, Events: nonNil(w.EventTypes),
			Targets: mapAll(w.Targets, func(tg harbor.WebhookTarget) WebhookTarget {
				return WebhookTarget{Type: tg.Type, Address: tg.Address, PayloadFormat: tg.PayloadFormat,
					SkipCertVerify: tg.SkipCertVerify, AuthHeaderSet: tg.AuthHeader != ""}
			})}
	}), in.PageArgs), nil
}

// GCRun is one garbage collection run.
type GCRun struct {
	Status  string `json:"status" jsonschema:"Pending, Running, Success, Error or Stopped"`
	Started string `json:"started,omitempty"`
	Updated string `json:"updated,omitempty"`
}

// GCStatus is the result of harbor_get_gc.
type GCStatus struct {
	Scheduled bool   `json:"scheduled"`
	Schedule  string `json:"schedule,omitempty" jsonschema:"Hourly, Daily, Weekly, Custom or None"`
	Cron      string `json:"cron,omitempty"`
	NextRun   string `json:"nextRun,omitempty"`
	LastRun   *GCRun `json:"lastRun,omitempty" jsonschema:"absent when garbage collection never ran"`
}

func (t *tools) getGC(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, GCStatus, error) {
	sched, err := t.hc.GetGCSchedule(ctx)
	if err != nil {
		return nil, GCStatus{}, t.fail("get the garbage collection schedule", err)
	}
	var out GCStatus
	if sched != nil && sched.Schedule != nil && sched.Schedule.Type != "" && sched.Schedule.Type != "None" {
		out = GCStatus{Scheduled: true, Schedule: sched.Schedule.Type, Cron: sched.Schedule.Cron, NextRun: sched.Schedule.NextScheduledTime}
	}
	last, err := t.hc.LatestGC(ctx)
	if err != nil {
		return nil, GCStatus{}, t.fail("get the last garbage collection run", err)
	}
	if last != nil {
		out.LastRun = &GCRun{Status: last.JobStatus, Started: ts(last.CreationTime), Updated: ts(last.UpdateTime)}
	}
	return nil, out, nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
