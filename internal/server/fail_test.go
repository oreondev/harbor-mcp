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

package server

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

func TestFailRedactsSecretsAndAddsHints(t *testing.T) {
	tl := &tools{secrets: nonEmpty([]string{"", "pw-123", "tok-456"})}
	err := tl.fail("list projects", &harbor.APIError{StatusCode: http.StatusForbidden, Message: "user pw-123 with token tok-456 denied"})
	msg := err.Error()
	for _, want := range []string{"list projects", "Harbor answered 403", "[redacted]", "harbor_system_info"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%q does not contain %q", msg, want)
		}
	}
	if strings.Contains(msg, "pw-123") || strings.Contains(msg, "tok-456") {
		t.Fatalf("secret in %q", msg)
	}
	plain := tl.fail("read", errors.New("dial tcp: connection refused"))
	if plain.Error() != "read: dial tcp: connection refused" {
		t.Fatalf("non-API error: %q", plain)
	}
}
