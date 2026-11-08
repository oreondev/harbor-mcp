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

// Command harbor-mcp is a Model Context Protocol server for Harbor. It speaks
// MCP over stdio, or streamable HTTP with -http; configuration comes from the
// environment (see docs/DESIGN.md).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
	"golang.org/x/time/rate"

	"github.com/oreondev/harbor-mcp/internal/config"
	"github.com/oreondev/harbor-mcp/internal/server"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	// Release builds set version with -ldflags; go install builds carry the
	// module version instead.
	if info, ok := debug.ReadBuildInfo(); ok && version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = strings.TrimPrefix(info.Main.Version, "v")
	}
	showVersion := flag.Bool("version", false, "print the version and exit")
	httpAddr := flag.String("http", "", "serve streamable HTTP on this address (for example :8080) instead of stdio")
	listTools := flag.Bool("list-tools", false, "print the tool definitions as JSON and exit (needs no Harbor)")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if *listTools {
		if err := printTools(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	// stdout carries the protocol; everything else goes to stderr.
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log, *httpAddr); err != nil {
		log.Error("harbor-mcp stopped", "error", err)
		os.Exit(1)
	}
}

func printTools() error {
	tools, err := server.Catalog(context.Background(), version)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(tools)
}

func run(log *slog.Logger, httpAddr string) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if httpAddr != "" && cfg.HTTPToken == "" && !server.IsLoopback(httpAddr) {
		return errors.New("HARBOR_MCP_HTTP_TOKEN is required when -http listens beyond loopback")
	}
	if cfg.InsecureSkipVerify {
		log.Warn("TLS verification of Harbor is off (HARBOR_INSECURE_SKIP_VERIFY)")
	}
	hc, err := harbor.New(harbor.Options{
		URL: cfg.URL, Username: cfg.Username, Password: cfg.Password,
		CABundle: cfg.CABundle, InsecureSkipVerify: cfg.InsecureSkipVerify,
		Limiter: rate.NewLimiter(rate.Limit(cfg.Rate), max(1, int(cfg.Rate))),
	})
	if err != nil {
		return err
	}
	s := server.New(hc, server.Options{
		Version: version, Username: cfg.Username, AllowWrite: cfg.AllowWrite, Secrets: []string{cfg.Password, cfg.HTTPToken},
		SkillsDir: cfg.SkillsDir, SkillsRequireSigned: cfg.SkillsRequireSigned,
	})
	if cfg.SkillsDir != "" {
		log.Info("harbor_install_skill installs into HARBOR_MCP_SKILLS_DIR", "dir", cfg.SkillsDir,
			"requireSigned", cfg.SkillsRequireSigned)
		if httpAddr != "" {
			log.Warn("HARBOR_MCP_SKILLS_DIR is set in HTTP mode: skills are installed on this server's machine, not the clients'")
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if httpAddr == "" {
		log.Info("harbor-mcp started", "version", version, "harbor", cfg.URL, "user", cfg.Username, "transport", "stdio")
		return s.Run(ctx, &mcp.StdioTransport{})
	}
	return serveHTTP(ctx, log, s, cfg, httpAddr)
}

// serveHTTP serves s on addr until ctx ends, then drains requests for up to
// ten seconds.
func serveHTTP(ctx context.Context, log *slog.Logger, s *mcp.Server, cfg config.Config, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	hs := &http.Server{Handler: server.HTTPHandler(s, cfg.HTTPToken, log), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	if cfg.HTTPToken == "" {
		log.Warn("no HARBOR_MCP_HTTP_TOKEN: HTTP requests are not authenticated (loopback only)")
	}
	log.Info("harbor-mcp started", "version", version, "harbor", cfg.URL, "user", cfg.Username,
		"transport", "http", "address", ln.Addr().String(), "endpoint", "/mcp", "auth", cfg.HTTPToken != "")
	if err := hs.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
