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
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (t *tools) register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "harbor_system_info",
		Description: "Harbor version, authentication mode and URLs, and the user or robot account the server acts as (with whether it is a Harbor administrator).",
		Annotations: readOnly("Harbor system information"),
	}, t.systemInfo)
	t.registerProjects(s)
	t.registerAccess(s)
	t.registerReplication(s)
	t.registerAdmin(s)
	t.registerSkills(s)
}

// SystemInfo is the result of harbor_system_info.
type SystemInfo struct {
	HarborVersion string `json:"harborVersion" jsonschema:"Harbor version, for example v2.15.4"`
	ExternalURL   string `json:"externalURL" jsonschema:"the URL Harbor is reached at"`
	RegistryURL   string `json:"registryURL,omitempty" jsonschema:"the registry host images are pushed to and pulled from"`
	AuthMode      string `json:"authMode" jsonschema:"db_auth, ldap_auth, oidc_auth, http_auth or uaa_auth"`
	User          string `json:"user" jsonschema:"the user or robot the server acts as"`
	Admin         bool   `json:"admin" jsonschema:"whether that user is a Harbor administrator"`
	Robot         bool   `json:"robot" jsonschema:"whether the server acts as a robot account; its permissions decide what the tools see"`
}

func (t *tools) systemInfo(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, SystemInfo, error) {
	info, err := t.hc.SystemInfo(ctx)
	if err != nil {
		return nil, SystemInfo{}, t.fail("read Harbor system information", err)
	}
	out := SystemInfo{HarborVersion: info.HarborVersion, ExternalURL: info.ExternalURL, RegistryURL: info.RegistryURL,
		AuthMode: info.AuthMode}
	u, err := t.hc.CurrentUser(ctx)
	if apiErr, ok := asAPIError(err); ok && apiErr.StatusCode == http.StatusPreconditionFailed {
		// Harbor has no current user for a robot account.
		out.User, out.Robot = t.username, true
		return nil, out, nil
	}
	if err != nil {
		return nil, SystemInfo{}, t.fail("read the current Harbor user", err)
	}
	out.User, out.Admin = u.Username, u.Admin
	return nil, out, nil
}
