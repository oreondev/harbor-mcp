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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor/fake"

	"github.com/oreondev/harbor-mcp/internal/server"
)

// bearer adds an Authorization header to every request.
type bearer string

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}

func httpServer(t *testing.T, token string) string {
	t.Helper()
	srv := fake.New(adminUser, adminPass)
	t.Cleanup(srv.Close)
	hc, err := harbor.New(harbor.Options{URL: srv.URL(), Username: adminUser, Password: adminPass})
	if err != nil {
		t.Fatal(err)
	}
	s := server.New(hc, server.Options{Version: "test", Secrets: []string{adminPass, token}})
	hs := httptest.NewServer(server.HTTPHandler(s, token, nil))
	t.Cleanup(hs.Close)
	return hs.URL
}

func httpSession(t *testing.T, url, token string) (*mcp.ClientSession, error) {
	t.Helper()
	hc := &http.Client{}
	if token != "" {
		hc.Transport = bearer(token)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: url + "/mcp", HTTPClient: hc}, nil)
	if err == nil {
		t.Cleanup(func() { _ = cs.Close() })
	}
	return cs, err
}

func TestHTTPRequiresTheToken(t *testing.T) {
	url := httpServer(t, httpToken)
	for _, tok := range []string{"", "wrong"} {
		if _, err := httpSession(t, url, tok); err == nil {
			t.Fatalf("token %q: connected without the right token", tok)
		}
		req, _ := http.NewRequest(http.MethodPost, url+"/mcp", strings.NewReader(`{}`))
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized || strings.Contains(string(body), httpToken) {
			t.Fatalf("token %q: HTTP %d %s", tok, resp.StatusCode, body)
		}
	}

	cs, err := httpSession(t, url, httpToken)
	if err != nil {
		t.Fatalf("connect with the token: %v", err)
	}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "harbor_system_info"})
	if err != nil || res.IsError {
		t.Fatalf("system info over HTTP: %v %+v", err, res)
	}
}

func TestHTTPHealthNeedsNoToken(t *testing.T) {
	resp, err := http.Get(httpServer(t, httpToken) + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: HTTP %d", resp.StatusCode)
	}
}

func TestHTTPWithoutToken(t *testing.T) {
	cs, err := httpSession(t, httpServer(t, ""), "")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := cs.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("list tools: %v", err)
	}
}

func TestHTTPRefusesCrossOriginBrowsers(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, httpServer(t, "")+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site request: HTTP %d", resp.StatusCode)
	}
}

func TestIsLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:8080": true, "[::1]:8080": true, "localhost:8080": true,
		":8080": false, "0.0.0.0:8080": false, "10.0.0.5:8080": false, "[::]:8080": false, "nonsense": false,
	} {
		if got := server.IsLoopback(addr); got != want {
			t.Errorf("IsLoopback(%q) = %v", addr, got)
		}
	}
}
