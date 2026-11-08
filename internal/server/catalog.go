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

package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

// CatalogSkillsDir is the skills directory Catalog registers the install
// tool with; nothing is written to it.
const CatalogSkillsDir = "/skills"

// Catalog returns the tool definitions, as a client sees them, without a
// Harbor: the docs and the directory listings are generated from it. It
// includes the tools that need opting in.
func Catalog(ctx context.Context, version string) ([]*mcp.Tool, error) {
	hc, err := harbor.New(harbor.Options{URL: "https://harbor.invalid"})
	if err != nil {
		return nil, err
	}
	st, ct := mcp.NewInMemoryTransports()
	ss, err := New(hc, Options{Version: version, SkillsDir: CatalogSkillsDir}).Connect(ctx, st, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "catalog", Version: version}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cs.Close() }()
	var out []*mcp.Tool
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		out = append(out, tool)
	}
	return out, nil
}
