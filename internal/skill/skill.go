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

// Package skill unpacks agent skills stored as OCI artifacts (one tar+gzip
// layer rooted at <name>/, agents-skills-oci-artifacts-spec v0.1.0) in
// memory, refusing anything that could escape the skill's directory or
// exhaust memory.
package skill

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Limits on one skill.
const (
	MaxLayerBytes    = 1 << 20 // compressed layer
	MaxUnpackedBytes = 4 << 20
	MaxEntries       = 500
)

// File is one regular file of a skill, with its path inside the skill
// directory (SKILL.md, scripts/run.sh).
type File struct {
	Path       string
	Data       []byte
	Executable bool
}

// Skill is an unpacked skill.
type Skill struct {
	Name  string
	Files []File
}

// nameRe is the Agent Skills name rule: lowercase letters, digits and
// hyphens, at most 64 characters.
var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidName reports whether name is a valid skill name.
func ValidName(name string) bool { return len(name) <= 64 && nameRe.MatchString(name) }

// Unpack reads a tar+gzip skill layer. Every entry must be a regular file or
// a directory under one top-level directory, which names the skill and must
// equal want when want is not empty. Links, devices, absolute paths and ".."
// are refused, as are more than MaxEntries entries or MaxUnpackedBytes.
func Unpack(layer []byte, want string) (*Skill, error) {
	if len(layer) > MaxLayerBytes {
		return nil, fmt.Errorf("skill layer is %d bytes, more than %d", len(layer), MaxLayerBytes)
	}
	zr, err := gzip.NewReader(bytes.NewReader(layer))
	if err != nil {
		return nil, fmt.Errorf("skill layer is not gzip: %w", err)
	}
	tr := tar.NewReader(zr)
	s := &Skill{Name: want}
	var total int64
	for n := 0; ; n++ {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("skill layer: %w", err)
		}
		if n >= MaxEntries {
			return nil, fmt.Errorf("skill has more than %d entries", MaxEntries)
		}
		top, rel, err := split(h.Name)
		if err != nil {
			return nil, err
		}
		if s.Name == "" {
			s.Name = top
		}
		if top != s.Name {
			return nil, fmt.Errorf("skill entry %q is outside %s/", h.Name, s.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return nil, fmt.Errorf("skill entry %q is not a regular file or directory", h.Name)
		}
		if rel == "" {
			return nil, fmt.Errorf("skill entry %q is a file in place of the skill directory", h.Name)
		}
		total += h.Size
		if h.Size < 0 || total > MaxUnpackedBytes {
			return nil, fmt.Errorf("skill unpacks to more than %d bytes", MaxUnpackedBytes)
		}
		data, err := io.ReadAll(io.LimitReader(tr, h.Size))
		if err != nil {
			return nil, fmt.Errorf("skill entry %q: %w", h.Name, err)
		}
		if slices.ContainsFunc(s.Files, func(f File) bool { return f.Path == rel }) {
			return nil, fmt.Errorf("skill entry %q appears twice", h.Name)
		}
		s.Files = append(s.Files, File{Path: rel, Data: data, Executable: h.Mode&0o111 != 0})
	}
	if !ValidName(s.Name) {
		return nil, fmt.Errorf("skill name %q is not valid (lowercase letters, digits and hyphens)", s.Name)
	}
	if s.File("SKILL.md") == nil {
		return nil, fmt.Errorf("skill %s has no SKILL.md", s.Name)
	}
	slices.SortFunc(s.Files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return s, nil
}

// split returns the top-level directory of a tar entry name and the path
// below it, refusing names that are absolute or climb out.
func split(name string) (top, rel string, err error) {
	clean := path.Clean(strings.TrimPrefix(name, "./"))
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || clean == "." ||
		clean == ".." || strings.HasPrefix(clean, "../") || slices.Contains(strings.Split(name, "/"), "..") {
		return "", "", fmt.Errorf("skill entry %q has an unsafe path", name)
	}
	top, rel, _ = strings.Cut(clean, "/")
	return top, rel, nil
}

// File returns the file at p, or nil.
func (s *Skill) File(p string) *File {
	for i := range s.Files {
		if s.Files[i].Path == p {
			return &s.Files[i]
		}
	}
	return nil
}
