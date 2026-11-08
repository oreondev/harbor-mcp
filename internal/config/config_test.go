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

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oreondev/harbor-mcp/internal/config"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	pwFile := filepath.Join(dir, "password")
	if err := os.WriteFile(pwFile, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(env(map[string]string{
		"HARBOR_URL": "https://harbor.example.com/", "HARBOR_USERNAME": "robot$mcp", "HARBOR_PASSWORD_FILE": pwFile,
		"HARBOR_MCP_RATE": "2.5", "HARBOR_MCP_ALLOW_WRITE": "true",
		"HARBOR_MCP_SKILLS_DIR": "/home/u/.claude/skills/", "HARBOR_MCP_SKILLS_REQUIRE_SIGNED": "false",
	}))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.URL != "https://harbor.example.com" || c.Username != "robot$mcp" || c.Password != "from-file" ||
		c.Rate != 2.5 || !c.AllowWrite || c.InsecureSkipVerify || c.SkillsDir != "/home/u/.claude/skills" || c.SkillsRequireSigned {
		t.Fatalf("config: %+v", c)
	}

	d, err := config.Load(env(map[string]string{"HARBOR_URL": "http://h", "HARBOR_USERNAME": "u", "HARBOR_PASSWORD": "p"}))
	if err != nil || d.Rate != config.DefaultRate || d.AllowWrite || d.SkillsDir != "" || !d.SkillsRequireSigned {
		t.Fatalf("defaults: %+v, %v", d, err)
	}
}

func TestLoadErrorsNameTheVariableNotTheValue(t *testing.T) {
	_, err := config.Load(env(map[string]string{
		"HARBOR_URL": "harbor.example.com", "HARBOR_USERNAME": "u",
		"HARBOR_PASSWORD": "top-secret", "HARBOR_PASSWORD_FILE": "/nonexistent",
		"HARBOR_MCP_RATE": "-1", "HARBOR_MCP_ALLOW_WRITE": "maybe",
		"HARBOR_MCP_SKILLS_DIR": "skills", "HARBOR_MCP_SKILLS_REQUIRE_SIGNED": "maybe",
	}))
	if err == nil {
		t.Fatal("want errors")
	}
	msg := err.Error()
	for _, want := range []string{"HARBOR_URL must start", "HARBOR_PASSWORD or HARBOR_PASSWORD_FILE", "HARBOR_MCP_RATE", "HARBOR_MCP_ALLOW_WRITE",
		"HARBOR_MCP_SKILLS_DIR: want an absolute", "HARBOR_MCP_SKILLS_REQUIRE_SIGNED"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%q does not mention %q", msg, want)
		}
	}
	if strings.Contains(msg, "top-secret") {
		t.Fatalf("error leaks the password: %q", msg)
	}
	_, err = config.Load(env(map[string]string{
		"HARBOR_URL": "${HARBOR_URL}", "HARBOR_USERNAME": "u", "HARBOR_PASSWORD": "${HARBOR_PASSWORD}",
	}))
	if err == nil || !strings.Contains(err.Error(), "HARBOR_URL is the placeholder ${HARBOR_URL}") ||
		!strings.Contains(err.Error(), "HARBOR_PASSWORD is the placeholder") || strings.Contains(err.Error(), "must start") {
		t.Fatalf("unexpanded placeholders: %v", err)
	}
	if _, err := config.Load(env(map[string]string{})); err == nil || !strings.Contains(err.Error(), "HARBOR_URL is required") {
		t.Fatalf("empty environment: %v", err)
	}
}
