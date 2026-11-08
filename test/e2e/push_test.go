//go:build e2e

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

package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

// Pushing a tiny image straight to Harbor's registry API, without docker; the
// same approach as harbor-resource-operator's e2e suite.

func harborURL() string { return os.Getenv("HARBOR_URL") }

// registryToken asks Harbor's token service for a bearer token with scope.
func registryToken(user, pass, scope string) (string, error) {
	q := url.Values{"service": {"harbor-registry"}, "scope": {scope}}
	req, err := http.NewRequest(http.MethodGet, harborURL()+"/service/token?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(user, pass)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token: HTTP %d: %s", resp.StatusCode, data)
	}
	var tok struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	return tok.Token, nil
}

// pushTestImage pushes a single-layer OCI image with unique content to repo:tag.
func pushTestImage(user, pass, repo, tag string) error {
	tok, err := registryToken(user, pass, "repository:"+repo+":pull,push")
	if err != nil {
		return err
	}
	layer, diffID, err := tinyLayer(fmt.Sprintf("%s:%s %d", repo, tag, time.Now().UnixNano()))
	if err != nil {
		return err
	}
	cfg, _ := json.Marshal(map[string]any{
		"architecture": "amd64", "os": "linux",
		"rootfs": map[string]any{"type": "layers", "diff_ids": []string{diffID}},
	})
	for _, blob := range [][]byte{layer, cfg} {
		if err := pushBlob(tok, repo, blob); err != nil {
			return err
		}
	}
	manifest, _ := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"config":        descriptor("application/vnd.oci.image.config.v1+json", cfg),
		"layers":        []any{descriptor("application/vnd.oci.image.layer.v1.tar+gzip", layer)},
	})
	return putManifest(tok, repo, tag, manifest)
}

func putManifest(tok, repo, ref string, manifest []byte) error {
	req, _ := http.NewRequest(http.MethodPut, harborURL()+"/v2/"+repo+"/manifests/"+ref, bytes.NewReader(manifest))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
	return expectStatus(http.DefaultClient.Do(req))(http.StatusCreated)
}

// pushSkill pushes an agent skill as ORAS would (Agent Skills OCI artifacts
// v0.1.0: one tar+gzip layer rooted at <name>/) to repo:tag and returns the
// manifest digest. files are paths inside the skill directory.
func pushSkill(user, pass, repo, tag, name string, files map[string]string) (string, error) {
	tok, err := registryToken(user, pass, "repository:"+repo+":pull,push")
	if err != nil {
		return "", err
	}
	rooted := map[string]string{}
	for p, body := range files {
		rooted[name+"/"+p] = body
	}
	layer, _, err := tarGz(rooted)
	if err != nil {
		return "", err
	}
	cfg, _ := json.Marshal(map[string]string{"name": name, "description": "e2e skill " + tag})
	for _, blob := range [][]byte{layer, cfg} {
		if err := pushBlob(tok, repo, blob); err != nil {
			return "", err
		}
	}
	manifest, _ := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"artifactType":  harbor.SkillArtifactType,
		"config":        descriptor(harbor.SkillConfigType, cfg),
		"layers":        []any{descriptor(harbor.SkillContentType, layer)},
		"annotations": map[string]string{harbor.SkillNameAnnotation: name, "org.opencontainers.image.version": tag,
			"org.opencontainers.image.created": time.Now().UTC().Format(time.RFC3339Nano)},
	})
	return digestOf(manifest), putManifest(tok, repo, tag, manifest)
}

// pushCosignSignature attaches a signature to repo@digest laid out as cosign
// does (tag sha256-<hex>.sig, simple-signing layer). Harbor records it as an
// accessory without checking the signature itself.
func pushCosignSignature(user, pass, repo, digest string) error {
	tok, err := registryToken(user, pass, "repository:"+repo+":pull,push")
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"critical": map[string]any{
		"identity": map[string]string{"docker-reference": repo}, "image": map[string]string{"docker-manifest-digest": digest},
		"type": "cosign container image signature"}})
	cfg, _ := json.Marshal(map[string]any{"architecture": "", "os": "", "config": map[string]any{},
		"rootfs": map[string]any{"type": "layers", "diff_ids": []string{digestOf(payload)}}})
	for _, blob := range [][]byte{payload, cfg} {
		if err := pushBlob(tok, repo, blob); err != nil {
			return err
		}
	}
	layer := descriptor("application/vnd.dev.cosign.simplesigning.v1+json", payload)
	layer["annotations"] = map[string]string{"dev.cosignproject.cosign/signature": "ZTJl"}
	manifest, _ := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"config":        descriptor("application/vnd.oci.image.config.v1+json", cfg),
		"layers":        []any{layer},
	})
	return putManifest(tok, repo, strings.Replace(digest, ":", "-", 1)+".sig", manifest)
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func descriptor(mediaType string, b []byte) map[string]any {
	return map[string]any{"mediaType": mediaType, "digest": digestOf(b), "size": len(b)}
}

// tinyLayer returns a gzipped tar with one file and the digest of the plain tar.
func tinyLayer(nonce string) ([]byte, string, error) {
	return tarGz(map[string]string{"e2e.txt": "harbor-mcp e2e " + nonce + "\n"})
}

// tarGz packs files (path to content) into a gzipped tar and returns it with
// the digest of the plain tar.
func tarGz(files map[string]string) ([]byte, string, error) {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			return nil, "", err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, "", err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(tarBuf.Bytes()); err != nil {
		return nil, "", err
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	return gz.Bytes(), digestOf(tarBuf.Bytes()), nil
}

// pushBlob uploads b monolithically: POST to start, PUT with the digest.
func pushBlob(tok, repo string, b []byte) error {
	req, _ := http.NewRequest(http.MethodPost, harborURL()+"/v2/"+repo+"/blobs/uploads/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("start upload to %s: HTTP %d", repo, resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		return err
	}
	// Harbor may answer with its external URL; keep only path and query.
	q := loc.Query()
	q.Set("digest", digestOf(b))
	put, _ := http.NewRequest(http.MethodPut, harborURL()+loc.Path+"?"+q.Encode(), bytes.NewReader(b))
	put.Header.Set("Authorization", "Bearer "+tok)
	put.Header.Set("Content-Type", "application/octet-stream")
	return expectStatus(http.DefaultClient.Do(put))(http.StatusCreated)
}

func expectStatus(resp *http.Response, err error) func(int) error {
	return func(want int) error {
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != want {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("%s %s: HTTP %d, want %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want, data)
		}
		return nil
	}
}

// deleteRepository removes project/repo through the API (repo URL-encoded twice).
func deleteRepository(user, pass, project, repo string) error {
	req, _ := http.NewRequest(http.MethodDelete,
		harborURL()+"/api/v2.0/projects/"+project+"/repositories/"+url.PathEscape(url.PathEscape(repo)), nil)
	req.SetBasicAuth(user, pass)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete repository %s/%s: HTTP %d", project, repo, resp.StatusCode)
	}
	return nil
}
