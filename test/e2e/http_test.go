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

package e2e

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const e2eToken = "e2e-http-token-value"

type bearer string

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}

// freeAddr returns a loopback address with a port nobody listens on.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().String()
}

func TestHTTPTransportAgainstRealHarbor(t *testing.T) {
	bin := binary(t)
	addr := freeAddr(t)
	cmd := exec.Command(bin, "-http", addr)
	cmd.Env = append(os.Environ(), "HARBOR_MCP_HTTP_TOKEN="+e2eToken)
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	})
	base := "http://" + addr
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become healthy: %v\nstderr: %s", err, stderr.String())
		}
		time.Sleep(200 * time.Millisecond)
	}

	resp, err := http.Post(base+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("request without token: HTTP %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "e2e"}, nil).Connect(ctx,
		&mcp.StreamableClientTransport{Endpoint: base + "/mcp", HTTPClient: &http.Client{Transport: bearer(e2eToken)}}, nil)
	if err != nil {
		t.Fatalf("connect: %v\nstderr: %s", err, stderr.String())
	}
	defer func() { _ = cs.Close() }()
	if info := call(t, cs, "harbor_system_info", nil); info["user"] != os.Getenv("HARBOR_USERNAME") {
		t.Fatalf("system info over HTTP: %v", info)
	}
	call(t, cs, "harbor_list_projects", nil)

	if log := stderr.String(); strings.Contains(log, e2eToken) || strings.Contains(log, os.Getenv("HARBOR_PASSWORD")) {
		t.Fatal("a secret appears in the server's log")
	}
}

func TestHTTPRefusesToStartWithoutTokenBeyondLoopback(t *testing.T) {
	cmd := exec.Command(binary(t), "-http", ":0")
	cmd.Env = append(os.Environ(), "HARBOR_MCP_HTTP_TOKEN=")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "HARBOR_MCP_HTTP_TOKEN is required") {
		t.Fatalf("started without a token on every interface: %v\n%s", err, out)
	}
}
