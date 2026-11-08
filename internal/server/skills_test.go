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

package server_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor/fake"

	"github.com/oreondev/harbor-mcp/internal/server"
)

// skillLayer packs files (paths under the skill directory) as a skill layer.
func skillLayer(t *testing.T, name string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for p, body := range files {
		must(t, tw.WriteHeader(&tar.Header{Name: name + "/" + p, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(body))
		must(t, err)
	}
	must(t, tw.Close())
	must(t, zw.Close())
	return buf.Bytes()
}

// pushSkill stores a skill as ORAS would push it and returns its digest.
func pushSkill(t *testing.T, srv *fake.Server, project, repo, name, version string, layer []byte, tags ...string) string {
	t.Helper()
	cfg, _ := json.Marshal(map[string]string{"name": name, "description": "Reviews pull requests."})
	cd := srv.AddBlob(project, repo, harbor.SkillConfigType, cfg)
	ld := srv.AddBlob(project, repo, harbor.SkillContentType, layer)
	return srv.AddManifest(project, repo, harbor.Manifest{SchemaVersion: 2, MediaType: "application/vnd.oci.image.manifest.v1+json",
		ArtifactType: harbor.SkillArtifactType, Config: cd, Layers: []harbor.Descriptor{ld},
		Annotations: map[string]string{harbor.SkillNameAnnotation: name, "org.opencontainers.image.version": version}}, tags...)
}

const reviewMD = "---\nname: review\ndescription: Reviews pull requests.\n---\n\nRead the diff first.\n"

func skillsHarbor(t *testing.T) (*fake.Server, string) {
	t.Helper()
	srv := newHarbor(t)
	srv.AddProject("skills", false)
	srv.AddRepository("skills", "review")
	srv.AddRepository("skills", "images-only")
	srv.AddArtifact("skills", "images-only", harbor.Artifact{Type: "IMAGE", Digest: "sha256:img"}, nil)
	files := map[string]string{"SKILL.md": reviewMD, "scripts/diff.sh": "git diff\n"}
	pushSkill(t, srv, "skills", "review", "review", "0.9.0", skillLayer(t, "review", map[string]string{"SKILL.md": "old"}), "0.9.0")
	d := pushSkill(t, srv, "skills", "review", "review", "1.0.0", skillLayer(t, "review", files), "1.0.0", "latest")
	srv.Sign(d)
	return srv, d
}

func TestListSkills(t *testing.T) {
	srv, d := skillsHarbor(t)
	cs := connect(t, srv.URL(), adminUser, adminPass)
	var page server.SkillPage
	if res := call(t, cs, "harbor_list_skills", map[string]any{"project": "skills"}, &page); res.IsError {
		t.Fatal(text(res))
	}
	if page.RepositoriesTotal != 2 || len(page.Items) != 1 {
		t.Fatalf("page: %+v", page)
	}
	s := page.Items[0]
	if s.Name != "review" || s.Repository != "skills/review" || s.Version != "1.0.0" || s.Digest != d || !s.Signed ||
		s.Versions != 2 || s.Description != "Reviews pull requests." || strings.Join(s.Tags, ",") != "1.0.0,latest" {
		t.Fatalf("skill: %+v", s)
	}
}

func TestGetSkill(t *testing.T) {
	srv, d := skillsHarbor(t)
	cs := connect(t, srv.URL(), adminUser, adminPass)
	var got server.SkillDetail
	if res := call(t, cs, "harbor_get_skill", map[string]any{"project": "skills", "repository": "review"}, &got); res.IsError {
		t.Fatal(text(res))
	}
	if got.Name != "review" || got.Digest != d || got.Source != "skills/review@"+d || !got.Signed || got.Version != "1.0.0" ||
		got.Path != "SKILL.md" || got.Content != reviewMD || len(got.Files) != 2 || got.Files[1].Path != "scripts/diff.sh" ||
		got.Notice == "" {
		t.Fatalf("newest skill: %+v", got)
	}

	// An older version by tag, and another file of the skill.
	got = server.SkillDetail{}
	call(t, cs, "harbor_get_skill", map[string]any{"project": "skills", "repository": "skills/review", "reference": "0.9.0"}, &got)
	if got.Version != "0.9.0" || got.Content != "old" || got.Signed {
		t.Fatalf("old version: %+v", got)
	}
	got = server.SkillDetail{}
	call(t, cs, "harbor_get_skill", map[string]any{"project": "skills", "repository": "review", "path": "scripts/diff.sh"}, &got)
	if got.Content != "git diff\n" {
		t.Fatalf("other file: %+v", got)
	}

	for what, args := range map[string]map[string]any{
		"has no file":         {"project": "skills", "repository": "review", "path": "../../etc/passwd"},
		"is not an agent":     {"project": "skills", "repository": "images-only", "reference": "sha256:" + strings.Repeat("0", 64)},
		"invalid tag":         {"project": "skills", "repository": "review", "reference": "1.0.0?x"},
		"has no skill":        {"project": "skills", "repository": "images-only"},
		"Harbor answered 404": {"project": "skills", "repository": "review", "reference": "2.0.0"},
	} {
		res := call(t, cs, "harbor_get_skill", args, nil)
		if !res.IsError || !strings.Contains(text(res), strings.TrimPrefix(what, "is not an agent")) {
			t.Errorf("%v: want an error containing %q, got %q", args, what, text(res))
		}
	}
}

func TestGetSkillUnderTheCosignPolicy(t *testing.T) {
	srv, _ := skillsHarbor(t)
	srv.Locked(func() {
		for _, p := range srv.Projects {
			p.Metadata.EnableContentTrustCosign = "true"
		}
	})
	cs := connect(t, srv.URL(), adminUser, adminPass)
	if res := call(t, cs, "harbor_get_skill", map[string]any{"project": "skills", "repository": "review"}, nil); res.IsError {
		t.Fatalf("signed skill: %s", text(res))
	}
	res := call(t, cs, "harbor_get_skill", map[string]any{"project": "skills", "repository": "review", "reference": "0.9.0"}, nil)
	if !res.IsError || !strings.Contains(text(res), "only allows signed artifacts") {
		t.Fatalf("unsigned skill: want the policy error, got %q", text(res))
	}
}

func TestGetSkillRefusesAHostileLayer(t *testing.T) {
	srv := newHarbor(t)
	srv.AddProject("skills", false)
	srv.AddRepository("skills", "evil")
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	must(t, tw.WriteHeader(&tar.Header{Name: "evil/SKILL.md", Mode: 0o644, Size: 2, Typeflag: tar.TypeReg}))
	_, _ = tw.Write([]byte("hi"))
	must(t, tw.WriteHeader(&tar.Header{Name: "evil/../../.bashrc", Mode: 0o644, Size: 2, Typeflag: tar.TypeReg}))
	_, _ = tw.Write([]byte("rm"))
	must(t, tw.Close())
	must(t, zw.Close())
	pushSkill(t, srv, "skills", "evil", "evil", "1.0.0", buf.Bytes(), "1.0.0")
	cs := connect(t, srv.URL(), adminUser, adminPass)
	res := call(t, cs, "harbor_get_skill", map[string]any{"project": "skills", "repository": "evil"}, nil)
	if !res.IsError || !strings.Contains(text(res), "unsafe path") {
		t.Fatalf("hostile layer: want an unsafe path error, got %q", text(res))
	}
}

func TestInstallSkill(t *testing.T) {
	srv, d := skillsHarbor(t)
	dir := filepath.Join(t.TempDir(), "skills")
	cs := connectWith(t, srv.URL(), adminUser, adminPass, server.Options{SkillsDir: dir, SkillsRequireSigned: true})
	var got server.InstalledSkill
	if res := call(t, cs, "harbor_install_skill", map[string]any{"project": "skills", "repository": "review"}, &got); res.IsError {
		t.Fatal(text(res))
	}
	if got.Name != "review" || got.Path != filepath.Join(dir, "review") || got.Digest != d || got.Source != "skills/review@"+d ||
		!got.Signed || got.Version != "1.0.0" || got.Files != 2 || got.Replaced || got.Notice == "" {
		t.Fatalf("installed: %+v", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "review", "SKILL.md")); string(b) != reviewMD {
		t.Fatalf("SKILL.md on disk: %q", b)
	}
	got = server.InstalledSkill{}
	call(t, cs, "harbor_install_skill", map[string]any{"project": "skills", "repository": "review", "reference": "latest"}, &got)
	if !got.Replaced {
		t.Fatalf("second install: want replaced, got %+v", got)
	}

	// 0.9.0 is not signed.
	res := call(t, cs, "harbor_install_skill", map[string]any{"project": "skills", "repository": "review", "reference": "0.9.0"}, nil)
	if !res.IsError || !strings.Contains(text(res), "HARBOR_MCP_SKILLS_REQUIRE_SIGNED") {
		t.Fatalf("unsigned skill: want a refusal, got %q", text(res))
	}
	cs = connectWith(t, srv.URL(), adminUser, adminPass, server.Options{SkillsDir: dir})
	got = server.InstalledSkill{}
	if res := call(t, cs, "harbor_install_skill", map[string]any{"project": "skills", "repository": "review", "reference": "0.9.0"},
		&got); res.IsError || got.Signed || got.Version != "0.9.0" {
		t.Fatalf("unsigned skill allowed: %+v %s", got, text(res))
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "review", "SKILL.md")); string(b) != "old" {
		t.Fatalf("SKILL.md after installing 0.9.0: %q", b)
	}

	// A directory of the same name that the tool did not install stays.
	must(t, os.MkdirAll(filepath.Join(dir, "mine"), 0o755))
	srv.AddRepository("skills", "mine")
	pushSkill(t, srv, "skills", "mine", "mine", "1.0.0", skillLayer(t, "mine", map[string]string{"SKILL.md": "x"}), "1.0.0")
	res = call(t, cs, "harbor_install_skill", map[string]any{"project": "skills", "repository": "mine"}, nil)
	if !res.IsError || !strings.Contains(text(res), "not installed by harbor-mcp") {
		t.Fatalf("foreign directory: want a refusal, got %q", text(res))
	}
}

func TestInstallSkillIsOptIn(t *testing.T) {
	srv := newHarbor(t)
	for _, o := range []server.Options{{}, {SkillsDir: t.TempDir()}} {
		res, err := connectWith(t, srv.URL(), adminUser, adminPass, o).ListTools(context.Background(), nil)
		must(t, err)
		var tool *mcp.Tool
		for _, x := range res.Tools {
			if x.Name == "harbor_install_skill" {
				tool = x
			}
		}
		switch {
		case o.SkillsDir == "" && tool != nil:
			t.Fatal("registered without a skills directory")
		case o.SkillsDir != "" && tool == nil:
			t.Fatal("not registered with a skills directory")
		case tool != nil && (tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil ||
			!*tool.Annotations.DestructiveHint):
			t.Fatalf("want a destructive, not read-only tool: %+v", tool.Annotations)
		}
	}
}
