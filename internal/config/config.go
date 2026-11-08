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

// Package config reads the server's settings from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Config is the server's configuration. Password and HTTPToken are secrets:
// never log or return them.
type Config struct {
	URL                string
	Username           string
	Password           string
	CABundle           []byte
	InsecureSkipVerify bool
	// Rate is the request rate to Harbor per second, shared by all tools.
	Rate float64
	// AllowWrite registers the write tools.
	AllowWrite bool
	// HTTPToken is the bearer token every HTTP request must carry in HTTP mode.
	HTTPToken string
	// SkillsDir, when set, registers harbor_install_skill, which installs
	// skills into it. It is an absolute path.
	SkillsDir string
	// SkillsRequireSigned makes harbor_install_skill refuse unsigned skills.
	SkillsRequireSigned bool
}

// DefaultRate is the request rate to Harbor when HARBOR_MCP_RATE is unset.
const DefaultRate = 10

// Load reads the configuration through getenv (os.Getenv in production).
// Every variable except HARBOR_CA_FILE can instead be read from a file named
// by <NAME>_FILE, for mounted Secrets; setting both is an error.
func Load(getenv func(string) string) (Config, error) {
	var c Config
	var errs []error
	get := func(name string) string {
		v, err := value(getenv, name)
		if err != nil {
			errs = append(errs, err)
		}
		if placeholderRe.MatchString(v) {
			// A client that passed its own ${VAR} placeholder through unset,
			// such as a Cursor plugin before Configure.
			errs = append(errs, fmt.Errorf("%s is the placeholder %s: set its value in the MCP client's settings", name, v))
			return ""
		}
		return v
	}
	c.URL = strings.TrimRight(get("HARBOR_URL"), "/")
	c.Username = get("HARBOR_USERNAME")
	c.Password = get("HARBOR_PASSWORD")
	c.HTTPToken = get("HARBOR_MCP_HTTP_TOKEN")
	if c.URL == "" {
		errs = append(errs, errors.New("HARBOR_URL is required"))
	} else if !strings.HasPrefix(c.URL, "https://") && !strings.HasPrefix(c.URL, "http://") {
		errs = append(errs, errors.New("HARBOR_URL must start with https:// or http://"))
	}
	if c.Username == "" || c.Password == "" {
		errs = append(errs, errors.New("HARBOR_USERNAME and HARBOR_PASSWORD are required"))
	}
	if path := getenv("HARBOR_CA_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("HARBOR_CA_FILE: %w", err))
		}
		c.CABundle = b
	}
	var err error
	if c.InsecureSkipVerify, err = boolVar(get("HARBOR_INSECURE_SKIP_VERIFY")); err != nil {
		errs = append(errs, fmt.Errorf("HARBOR_INSECURE_SKIP_VERIFY: %w", err))
	}
	if c.AllowWrite, err = boolVar(get("HARBOR_MCP_ALLOW_WRITE")); err != nil {
		errs = append(errs, fmt.Errorf("HARBOR_MCP_ALLOW_WRITE: %w", err))
	}
	if c.SkillsDir = get("HARBOR_MCP_SKILLS_DIR"); c.SkillsDir != "" {
		if !filepath.IsAbs(c.SkillsDir) {
			errs = append(errs, fmt.Errorf("HARBOR_MCP_SKILLS_DIR: want an absolute path, got %q", c.SkillsDir))
		}
		c.SkillsDir = filepath.Clean(c.SkillsDir)
	}
	c.SkillsRequireSigned = true
	if s := get("HARBOR_MCP_SKILLS_REQUIRE_SIGNED"); s != "" {
		if c.SkillsRequireSigned, err = strconv.ParseBool(s); err != nil {
			errs = append(errs, fmt.Errorf("HARBOR_MCP_SKILLS_REQUIRE_SIGNED: %w", err))
		}
	}
	c.Rate = DefaultRate
	if s := get("HARBOR_MCP_RATE"); s != "" {
		r, perr := strconv.ParseFloat(s, 64)
		if perr != nil || r <= 0 {
			errs = append(errs, fmt.Errorf("HARBOR_MCP_RATE: want a positive number, got %q", s))
		}
		c.Rate = r
	}
	return c, errors.Join(errs...)
}

// placeholderRe matches an unexpanded client variable such as ${HARBOR_URL}.
var placeholderRe = regexp.MustCompile(`^\$\{[A-Za-z0-9_:.-]+\}$`)

// value returns name from the environment or from the file named by name_FILE.
func value(getenv func(string) string, name string) (string, error) {
	v, path := getenv(name), getenv(name+"_FILE")
	if path == "" {
		return v, nil
	}
	if v != "" {
		return "", fmt.Errorf("set %s or %s_FILE, not both", name, name)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s_FILE: %w", name, err)
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

func boolVar(s string) (bool, error) {
	if s == "" {
		return false, nil
	}
	return strconv.ParseBool(s)
}
