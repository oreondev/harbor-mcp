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

// Package e2e runs the built binary over stdio against a real Harbor named by
// HARBOR_URL, HARBOR_USERNAME and HARBOR_PASSWORD. Run with
// go test -tags=e2e ./test/e2e/; it skips when HARBOR_URL is unset.
package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioAgainstRealHarbor(t *testing.T) {
	if os.Getenv("HARBOR_URL") == "" {
		t.Skip("HARBOR_URL not set")
	}
	bin := filepath.Join(t.TempDir(), "harbor-mcp")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=e2e", "-o", bin, "../../cmd/harbor-mcp")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.Command(bin)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "e2e"}, nil).
		Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v\nstderr: %s", err, stderr.String())
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "harbor_system_info"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	b, _ := json.Marshal(res.StructuredContent)
	t.Logf("harbor_system_info: %s", b)
	var info struct {
		HarborVersion string `json:"harborVersion"`
		User          string `json:"user"`
	}
	if err := json.Unmarshal(b, &info); err != nil || !strings.HasPrefix(info.HarborVersion, "v2.") ||
		info.User != os.Getenv("HARBOR_USERNAME") {
		t.Fatalf("unexpected result %s: %v", b, err)
	}
	if strings.Contains(stderr.String(), os.Getenv("HARBOR_PASSWORD")) {
		t.Fatal("the password appears in the server's log")
	}
}
