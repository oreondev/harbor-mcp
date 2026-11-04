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

// Package server builds the Harbor MCP server and its tools.
package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

// Options configure the server.
type Options struct {
	// Version is reported to clients.
	Version string
	// Username is the Harbor identity the server was configured with. It is
	// reported for robot accounts, which Harbor cannot describe.
	Username string
	// AllowWrite registers the write tools.
	AllowWrite bool
	// Secrets are removed from every error text before it reaches a client:
	// the Harbor password and the HTTP token.
	Secrets []string
}

const instructions = `Tools for one Harbor container registry, through its REST API v2.0.
Start with harbor_system_info to see the Harbor version and the identity the server runs as;
its permissions limit what the other tools can see. Lists are paged: pass page and pageSize
(at most 100) and read total in the result.`

// New returns an MCP server with the Harbor tools registered.
func New(hc *harbor.Client, o Options) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:       "harbor-mcp",
		Title:      "Harbor MCP server",
		Version:    o.Version,
		WebsiteURL: "https://github.com/oreondev/harbor-mcp",
	}, &mcp.ServerOptions{Instructions: instructions})
	t := &tools{hc: hc, username: o.Username, secrets: nonEmpty(o.Secrets)}
	t.register(s)
	return s
}

// tools holds what every tool handler needs.
type tools struct {
	hc       *harbor.Client
	username string
	secrets  []string
}

// readOnly annotates a tool that only reads from Harbor.
func readOnly(title string) *mcp.ToolAnnotations {
	open := true
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &open}
}

// fail turns a Harbor error into a tool error an assistant can explain, with
// any secret removed.
func (t *tools) fail(what string, err error) error {
	msg := fmt.Sprintf("%s: %v", what, err)
	if apiErr, ok := asAPIError(err); ok {
		msg = fmt.Sprintf("%s: Harbor answered %d", what, apiErr.StatusCode)
		if apiErr.Message != "" {
			msg += ": " + apiErr.Message
		}
		if hint := hints[apiErr.StatusCode]; hint != "" {
			msg += ". " + hint
		}
	}
	for _, s := range t.secrets {
		msg = strings.ReplaceAll(msg, s, "[redacted]")
	}
	return errors.New(msg)
}

var hints = map[int]string{
	http.StatusUnauthorized:        "The server's Harbor credentials were rejected; check HARBOR_USERNAME and HARBOR_PASSWORD.",
	http.StatusForbidden:           "The server's Harbor identity lacks the permission for this; harbor_system_info shows who it is.",
	http.StatusNotFound:            "Check the names; they are case-sensitive.",
	http.StatusTooManyRequests:     "Harbor is rate limiting; retry later.",
	http.StatusInternalServerError: "Harbor had an internal error; retry, and check Harbor's logs if it persists.",
	http.StatusBadGateway:          "Harbor is unreachable behind its proxy; retry later.",
	http.StatusServiceUnavailable:  "Harbor is unavailable; retry later.",
}

// asAPIError returns the Harbor API error inside err, if any.
func asAPIError(err error) (*harbor.APIError, bool) { return errors.AsType[*harbor.APIError](err) }

func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
