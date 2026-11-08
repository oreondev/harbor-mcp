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
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// MarkerFile is written into every installed skill directory. Install
// replaces only directories that carry it.
const MarkerFile = ".harbor-skill.json"

// Marker records where an installed skill came from.
type Marker struct {
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
	Version    string `json:"version,omitempty"`
	Signed     bool   `json:"signed"`
}

// ErrNotOurs means the target directory exists and was not installed by
// Install, or was installed from another repository.
var ErrNotOurs = errors.New("not installed from this repository")

// Install writes s to dir/<name>/ with m as its marker and returns that
// path and whether an earlier install was replaced. The skill is written to
// a temporary directory first and renamed into place, so a failed install
// leaves the old one as it was.
func Install(dir string, s *Skill, m Marker) (target string, replaced bool, err error) {
	if !filepath.IsAbs(dir) || !ValidName(s.Name) {
		return "", false, fmt.Errorf("install %q into %q: want an absolute directory and a valid name", s.Name, dir)
	}
	if s.File(MarkerFile) != nil {
		return "", false, fmt.Errorf("skill %s carries its own %s", s.Name, MarkerFile)
	}
	target = filepath.Join(dir, s.Name)
	if replaced, err = check(target, m.Repository); err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	tmp, err := os.MkdirTemp(dir, ".harbor-skill-")
	if err != nil {
		return "", false, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := write(tmp, s, m); err != nil {
		return "", false, err
	}
	if replaced {
		// The old install moves inside the new one and is removed from there.
		old := filepath.Join(tmp, ".old")
		if err := os.Rename(target, old); err != nil {
			return "", false, err
		}
		if err := os.Rename(tmp, target); err != nil {
			_ = os.Rename(old, target)
			return "", false, err
		}
		return target, true, os.RemoveAll(filepath.Join(target, ".old"))
	}
	return target, false, os.Rename(tmp, target)
}

// check reports whether target holds an install of repo that may be
// replaced; any other existing entry is an error.
func check(target, repo string) (bool, error) {
	fi, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !fi.IsDir() {
		return false, fmt.Errorf("%s exists and is not a directory: %w", target, ErrNotOurs)
	}
	root, err := os.OpenRoot(target)
	if err != nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	b, err := root.ReadFile(MarkerFile)
	if errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("%s exists and has no %s, so it was not installed by harbor-mcp: %w", target, MarkerFile, ErrNotOurs)
	}
	if err != nil {
		return false, err
	}
	var old Marker
	if err := json.Unmarshal(b, &old); err != nil || old.Repository != repo {
		return false, fmt.Errorf("%s was installed from %q: %w", target, old.Repository, ErrNotOurs)
	}
	return true, nil
}

// write puts the files and the marker into dir through an os.Root, so no
// path can leave it.
func write(dir string, s *Skill, m Marker) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	for _, f := range s.Files {
		if d := path.Dir(f.Path); d != "." {
			if err := root.MkdirAll(d, 0o755); err != nil {
				return err
			}
		}
		mode := fs.FileMode(0o644)
		if f.Executable {
			mode = 0o755
		}
		if err := root.WriteFile(f.Path, f.Data, mode); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return root.WriteFile(MarkerFile, append(b, '\n'), 0o644)
}
