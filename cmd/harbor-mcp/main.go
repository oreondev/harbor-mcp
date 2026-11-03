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

// Command harbor-mcp is a Model Context Protocol server for Harbor. It speaks
// MCP over stdio; configuration comes from the environment (see docs/DESIGN.md).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
	"golang.org/x/time/rate"

	"github.com/oreondev/harbor-mcp/internal/config"
	"github.com/oreondev/harbor-mcp/internal/server"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	// stdout carries the protocol; everything else goes to stderr.
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("harbor-mcp stopped", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
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
		Version: version, AllowWrite: cfg.AllowWrite, Secrets: []string{cfg.Password, cfg.HTTPToken},
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Info("harbor-mcp started", "version", version, "harbor", cfg.URL, "user", cfg.Username, "transport", "stdio")
	return s.Run(ctx, &mcp.StdioTransport{})
}
