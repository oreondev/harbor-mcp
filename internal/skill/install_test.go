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

package skill

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func review(body string) *Skill {
	return &Skill{Name: "review", Files: []File{
		{Path: "SKILL.md", Data: []byte(body)},
		{Path: "scripts/diff.sh", Data: []byte("git diff\n"), Executable: true},
	}}
}

func TestInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills") // created by Install
	m := Marker{Repository: "skills/review", Digest: "sha256:1", Version: "1.0.0", Signed: true}
	target, replaced, err := Install(dir, review("v1"), m)
	if err != nil || replaced || target != filepath.Join(dir, "review") {
		t.Fatalf("install: %q %v %v", target, replaced, err)
	}
	if b, _ := os.ReadFile(filepath.Join(target, "SKILL.md")); string(b) != "v1" {
		t.Fatalf("SKILL.md: %q", b)
	}
	if fi, err := os.Stat(filepath.Join(target, "scripts/diff.sh")); err != nil || fi.Mode().Perm()&0o100 == 0 {
		t.Fatalf("script mode: %v %v", fi, err)
	}
	var got Marker
	b, _ := os.ReadFile(filepath.Join(target, MarkerFile))
	if err := json.Unmarshal(b, &got); err != nil || got != m {
		t.Fatalf("marker: %s %v", b, err)
	}

	// A new version of the same repository replaces it; leftover files go.
	must(t, os.WriteFile(filepath.Join(target, "local-note"), []byte("x"), 0o644))
	m.Digest, m.Version = "sha256:2", "1.1.0"
	if _, replaced, err = Install(dir, review("v2"), m); err != nil || !replaced {
		t.Fatalf("replace: %v %v", replaced, err)
	}
	if b, _ := os.ReadFile(filepath.Join(target, "SKILL.md")); string(b) != "v2" {
		t.Fatalf("SKILL.md after replace: %q", b)
	}
	entries, _ := os.ReadDir(dir)
	if _, err := os.Stat(filepath.Join(target, "local-note")); err == nil || len(entries) != 1 {
		t.Fatalf("want only the new install left, got %d entries in %s", len(entries), dir)
	}
	if _, err := os.Stat(filepath.Join(target, ".old")); err == nil {
		t.Fatal("old install left behind")
	}

	// The same name from another repository is refused.
	m.Repository = "other/review"
	if _, _, err := Install(dir, review("v3"), m); !errors.Is(err, ErrNotOurs) {
		t.Fatalf("other repository: want ErrNotOurs, got %v", err)
	}
}

func TestInstallRefuses(t *testing.T) {
	m := Marker{Repository: "skills/review", Digest: "sha256:1"}
	for name, setup := range map[string]func(dir string){
		"a directory it did not install": func(dir string) { must(t, os.MkdirAll(filepath.Join(dir, "review"), 0o755)) },
		"a file":                         func(dir string) { must(t, os.WriteFile(filepath.Join(dir, "review"), nil, 0o644)) },
		"a symlink": func(dir string) {
			must(t, os.Mkdir(filepath.Join(dir, "elsewhere"), 0o755))
			must(t, os.Symlink(filepath.Join(dir, "elsewhere"), filepath.Join(dir, "review")))
		},
		"a marker that is a symlink out": func(dir string) {
			outside := filepath.Join(t.TempDir(), MarkerFile)
			must(t, os.WriteFile(outside, []byte(`{"repository":"skills/review"}`), 0o644))
			must(t, os.Mkdir(filepath.Join(dir, "review"), 0o755))
			must(t, os.Symlink(outside, filepath.Join(dir, "review", MarkerFile)))
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			setup(dir)
			if _, _, err := Install(dir, review("v1"), m); err == nil {
				t.Fatal("want an error")
			}
			if _, err := os.Stat(filepath.Join(dir, "review", "SKILL.md")); err == nil {
				t.Fatal("wrote into the existing entry")
			}
		})
	}

	dir := t.TempDir()
	s := review("v1")
	s.Files = append(s.Files, File{Path: MarkerFile, Data: []byte(`{}`)})
	if _, _, err := Install(dir, s, m); err == nil {
		t.Fatal("skill with its own marker: want an error")
	}
	if _, _, err := Install("relative", review("v1"), m); err == nil {
		t.Fatal("relative directory: want an error")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
