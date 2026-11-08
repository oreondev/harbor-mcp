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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
	"testing"
)

type entry struct {
	name string
	typ  byte
	body string
	size int64 // header size when not len(body)
	mode int64
}

func pack(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o644, Size: int64(len(e.body))}
		if e.mode != 0 {
			h.Mode = e.mode
		}
		if e.typ == 0 {
			h.Typeflag = tar.TypeReg
		}
		if h.Typeflag != tar.TypeReg {
			h.Size = 0
		}
		if e.typ == tar.TypeSymlink || e.typ == tar.TypeLink {
			h.Linkname = "/etc/passwd"
		}
		if e.size != 0 {
			h.Size = e.size
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			body := e.body
			if e.size != 0 {
				body = strings.Repeat("a", int(e.size))
			}
			if _, err := tw.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const skillMD = "---\nname: review\ndescription: Reviews code.\n---\n\nReview it.\n"

func TestUnpack(t *testing.T) {
	layer := pack(t,
		entry{name: "review/", typ: tar.TypeDir},
		entry{name: "review/SKILL.md", body: skillMD},
		entry{name: "review/scripts/", typ: tar.TypeDir},
		entry{name: "review/scripts/run.sh", body: "echo hi\n", mode: 0o755},
	)
	s, err := Unpack(layer, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "review" || len(s.Files) != 2 || s.Files[0].Path != "SKILL.md" || s.Files[1].Path != "scripts/run.sh" {
		t.Fatalf("unpacked: %+v", s)
	}
	if !s.Files[1].Executable || s.Files[0].Executable {
		t.Fatalf("executable bits: %+v", s.Files)
	}
	if string(s.File("SKILL.md").Data) != skillMD {
		t.Fatalf("SKILL.md: %q", s.File("SKILL.md").Data)
	}
	if _, err := Unpack(layer, "other"); err == nil {
		t.Fatal("a skill whose directory is not the expected name was accepted")
	}
}

func TestUnpackRefusesHostileLayers(t *testing.T) {
	md := entry{name: "review/SKILL.md", body: skillMD}
	many := make([]entry, 0, 1+MaxEntries)
	many = append(many, md)
	for i := range MaxEntries {
		many = append(many, entry{name: fmt.Sprintf("review/f%d", i), body: "x"})
	}
	cases := map[string][]entry{
		"dot-dot":          {md, {name: "review/../../etc/cron.d/x", body: "x"}},
		"absolute":         {md, {name: "/etc/passwd", body: "x"}},
		"backslash":        {md, {name: "review/..\\..\\x", body: "x"}},
		"symlink":          {md, {name: "review/link", typ: tar.TypeSymlink}},
		"hard link":        {md, {name: "review/link", typ: tar.TypeLink}},
		"device":           {md, {name: "review/dev", typ: tar.TypeChar}},
		"second top dir":   {md, {name: "other/SKILL.md", body: skillMD}},
		"file as top":      {{name: "review", body: "x"}},
		"no SKILL.md":      {{name: "review/README.md", body: "x"}},
		"duplicate":        {md, md},
		"bad name":         {{name: "Review/SKILL.md", body: skillMD}},
		"too many entries": many,
		"tar bomb":         {md, {name: "review/big", size: MaxUnpackedBytes + 1}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			if s, err := Unpack(pack(t, entries...), ""); err == nil {
				t.Fatalf("accepted: %+v", s)
			}
		})
	}
	if _, err := Unpack([]byte("not gzip"), ""); err == nil {
		t.Fatal("a layer that is not gzip was accepted")
	}
	if _, err := Unpack(make([]byte, MaxLayerBytes+1), ""); err == nil {
		t.Fatal("a layer above the limit was accepted")
	}
}
