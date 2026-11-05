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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

func (t *tools) registerReplication(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "harbor_list_registries",
		Description: "List the remote registries (endpoints) Harbor replicates to or from, with their health. " +
			"Credentials are never shown.",
		Annotations: readOnly("List registries"),
	}, t.listRegistries)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "harbor_list_replications",
		Description: "List replication policies with source, destination, trigger and the outcome of the last run.",
		Annotations: readOnly("List replication policies"),
	}, t.listReplications)
}

// QueryArgs filter a list by name.
type QueryArgs struct {
	Query string `json:"query,omitempty" jsonschema:"only items whose name contains this"`
	PageArgs
}

// RegistrySummary is one registry endpoint.
type RegistrySummary struct {
	Name        string `json:"name"`
	Type        string `json:"type" jsonschema:"harbor, docker-hub, aws-ecr, google-gcr and so on"`
	URL         string `json:"url"`
	Insecure    bool   `json:"insecure" jsonschema:"whether TLS verification is off"`
	Status      string `json:"status" jsonschema:"healthy or unhealthy"`
	Description string `json:"description,omitempty"`
}

func (t *tools) listRegistries(ctx context.Context, _ *mcp.CallToolRequest, in QueryArgs) (*mcp.CallToolResult, Page[RegistrySummary], error) {
	regs, err := t.hc.ListRegistries(ctx, in.Query)
	if err != nil {
		return nil, Page[RegistrySummary]{}, t.fail("list registries", err)
	}
	return nil, localPage(mapAll(regs, func(r harbor.Registry) RegistrySummary {
		return RegistrySummary{Name: r.Name, Type: r.Type, URL: r.URL, Insecure: r.Insecure, Status: r.Status, Description: r.Description}
	}), in.PageArgs), nil
}

// Execution is one run of a replication policy.
type Execution struct {
	Status    string `json:"status" jsonschema:"InProgress, Succeed, Failed or Stopped"`
	Trigger   string `json:"trigger" jsonschema:"manual, scheduled or event_based"`
	Started   string `json:"started,omitempty"`
	Ended     string `json:"ended,omitempty"`
	Total     int64  `json:"total"`
	Succeeded int64  `json:"succeeded"`
	Failed    int64  `json:"failed"`
	Message   string `json:"message,omitempty"`
}

// ReplicationSummary is one replication policy.
type ReplicationSummary struct {
	Name                 string     `json:"name"`
	Enabled              bool       `json:"enabled"`
	Direction            string     `json:"direction" jsonschema:"push (from this Harbor) or pull (into this Harbor)"`
	Remote               string     `json:"remote" jsonschema:"name of the remote registry"`
	DestinationNamespace string     `json:"destinationNamespace,omitempty"`
	Trigger              string     `json:"trigger" jsonschema:"manual, scheduled or event_based"`
	Cron                 string     `json:"cron,omitempty"`
	ReplicateDeletion    bool       `json:"replicateDeletion"`
	LastRun              *Execution `json:"lastRun,omitempty" jsonschema:"absent when it never ran"`
}

func (t *tools) listReplications(ctx context.Context, _ *mcp.CallToolRequest, in QueryArgs) (*mcp.CallToolResult, Page[ReplicationSummary], error) {
	pols, err := t.hc.ListReplicationPolicies(ctx, in.Query)
	if err != nil {
		return nil, Page[ReplicationSummary]{}, t.fail("list replication policies", err)
	}
	pg := localPage(pols, in.PageArgs)
	out := make([]ReplicationSummary, 0, len(pg.Items))
	// One request per policy for its last run, so only for this page.
	for _, p := range pg.Items {
		s := ReplicationSummary{Name: p.Name, Enabled: p.Enabled, DestinationNamespace: p.DestNamespace,
			ReplicateDeletion: p.ReplicateDeletion, Direction: "push"}
		remote := p.DestRegistry
		if p.SrcRegistry != nil && p.SrcRegistry.ID != 0 {
			s.Direction, remote = "pull", p.SrcRegistry
		}
		if remote != nil {
			s.Remote = remote.Name
		}
		if p.Trigger != nil {
			s.Trigger = p.Trigger.Type
			if p.Trigger.TriggerSettings != nil {
				s.Cron = p.Trigger.TriggerSettings.Cron
			}
		}
		ex, err := t.hc.LatestReplicationExecution(ctx, p.ID)
		if err != nil {
			return nil, Page[ReplicationSummary]{}, t.fail("get the last run of replication policy "+p.Name, err)
		}
		if ex != nil {
			s.LastRun = &Execution{Status: ex.Status, Trigger: ex.Trigger, Started: ts(ex.StartTime), Ended: ts(ex.EndTime),
				Total: ex.Total, Succeeded: ex.Succeed, Failed: ex.Failed, Message: clip(ex.StatusText, 300)}
		}
		out = append(out, s)
	}
	return nil, Page[ReplicationSummary]{Items: out, Total: pg.Total, Page: pg.Page, PageSize: pg.PageSize}, nil
}
