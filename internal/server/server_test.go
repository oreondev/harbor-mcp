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

package server_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor/fake"

	"github.com/oreondev/harbor-mcp/internal/server"
)

const (
	adminUser = "admin"
	adminPass = "s3cret-admin-pass"
	httpToken = "s3cret-http-token"
)

// connect starts the server against Harbor at url as user/pass and returns a
// client session over the SDK's in-memory transport.
func connect(t *testing.T, url, user, pass string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	hc, err := harbor.New(harbor.Options{URL: url, Username: user, Password: pass})
	if err != nil {
		t.Fatalf("harbor client: %v", err)
	}
	s := server.New(hc, server.Options{Version: "test", Secrets: []string{pass, httpToken}})
	st, ct := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		_ = ss.Wait()
	})
	return cs
}

func newHarbor(t *testing.T) *fake.Server {
	t.Helper()
	srv := fake.New(adminUser, adminPass)
	t.Cleanup(srv.Close)
	return srv
}

// call invokes a tool and decodes its structured result into out (when not nil).
func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error: %v", name, err)
	}
	if out != nil && !res.IsError {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s: decode %s: %v", name, b, err)
		}
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestReadToolsAreAnnotatedAndDescribed(t *testing.T) {
	srv := newHarbor(t)
	cs := connect(t, srv.URL(), adminUser, adminPass)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(res.Tools) == 0 {
		t.Fatal("no tools registered")
	}
	for _, tool := range res.Tools {
		if !strings.HasPrefix(tool.Name, "harbor_") || tool.Description == "" {
			t.Errorf("%s: want a harbor_ name and a description", tool.Name)
		}
		a := tool.Annotations
		if a == nil || !a.ReadOnlyHint || a.Title == "" {
			t.Errorf("%s: want read-only annotations with a title, got %+v", tool.Name, a)
		}
		if tool.OutputSchema == nil {
			t.Errorf("%s: want an output schema", tool.Name)
		}
	}
}

func TestSystemInfo(t *testing.T) {
	srv := newHarbor(t)
	cs := connect(t, srv.URL(), adminUser, adminPass)
	var got server.SystemInfo
	res := call(t, cs, "harbor_system_info", nil, &got)
	if res.IsError {
		t.Fatalf("tool error: %s", text(res))
	}
	if got.HarborVersion != srv.Version || got.User != adminUser || !got.Admin || got.AuthMode != "db_auth" {
		t.Fatalf("system info: %+v", got)
	}
}

func TestBadCredentialsAreAToolErrorWithoutTheSecret(t *testing.T) {
	srv := newHarbor(t)
	cs := connect(t, srv.URL(), adminUser, "wrong-"+adminPass)
	res := call(t, cs, "harbor_system_info", nil, nil)
	if !res.IsError {
		t.Fatal("want a tool error for rejected credentials")
	}
	msg := text(res)
	if !strings.Contains(msg, "401") || !strings.Contains(msg, "HARBOR_PASSWORD") {
		t.Fatalf("error should give the status and a hint: %q", msg)
	}
	if strings.Contains(msg, adminPass) {
		t.Fatalf("error leaks the password: %q", msg)
	}
}
