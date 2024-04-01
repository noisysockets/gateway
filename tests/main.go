// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/noisysockets/noisysockets"
	"github.com/noisysockets/noisysockets/config"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	pwd, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get working directory", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	testNet, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		logger.Error("Failed to create Docker network", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := testNet.Remove(ctx); err != nil {
			logger.Error("Failed to remove Docker network", "error", err)
		}
	}()

	// Spin up an nginx server.
	nginxReq := testcontainers.ContainerRequest{
		Image:        "nginx:latest",
		ExposedPorts: []string{"80/tcp"},
		Networks:     []string{testNet.Name},
		NetworkAliases: map[string][]string{
			testNet.Name: {"web"},
		},
		WaitingFor: wait.ForListeningPort("80/tcp"),
	}

	nginxC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: nginxReq,
		Started:          true,
	})
	if err != nil {
		logger.Error("Failed to start nginx container", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := nginxC.Terminate(ctx); err != nil {
			logger.Error("Failed to terminate nginx container", "error", err)
		}
	}()

	// Spin up a WireGuard gateway.
	wgReq := testcontainers.ContainerRequest{
		Image:        "ghcr.io/noisysockets/gateway:latest-dev",
		ExposedPorts: []string{"51820/udp", "53/tcp"},
		Files: []testcontainers.ContainerFile{
			{HostFilePath: filepath.Join(pwd, "testdata/wg0.conf"), ContainerFilePath: "/etc/wireguard/wg0.conf", FileMode: 0o400},
		},
		Networks: []string{testNet.Name},
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.CapAdd = []string{"NET_ADMIN"}

			hostConfig.Sysctls = map[string]string{
				"net.ipv4.ip_forward":              "1",
				"net.ipv4.conf.all.src_valid_mark": "1",
			}

			hostConfig.Binds = append(hostConfig.Binds, "/dev/net/tun:/dev/net/tun")
		},
		// Rely on the fact dnsmasq is started after the interface is up.
		WaitingFor: wait.ForListeningPort("53/tcp"),
	}

	wgC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: wgReq,
		Started:          true,
	})
	if err != nil {
		logger.Error("Failed to start WireGuard gateway container", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := wgC.Terminate(ctx); err != nil {
			logger.Error("Failed to terminate WireGuard gateway container", "error", err)
		}
	}()

	outputDir, err := os.MkdirTemp("", "noisysockets")
	if err != nil {
		logger.Error("Failed to create temporary directory", "error", err)
		os.Exit(1)
	}
	defer os.RemoveAll(outputDir)

	configPath := filepath.Join(outputDir, "noisysockets.yaml")

	if err := generateConfig(ctx, configPath, wgC); err != nil {
		logger.Error("Failed to render noisysockets configuration", "error", err)
		os.Exit(1)
	}

	conf, err := config.FromYAML(configPath)
	if err != nil {
		logger.Error("Failed to load noisysockets configuration", "error", err)
		os.Exit(1)
	}

	net, err := noisysockets.NewNetwork(logger, conf)
	if err != nil {
		logger.Error("Failed to create noisysockets network", "error", err)
		os.Exit(1)
	}
	defer net.Close()

	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: net.Dial,
		},
	}

	resp, err := httpClient.Get("http://web/")
	if err != nil {
		logger.Error("Failed to make HTTP request", "error", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Unexpected HTTP status code", "status", resp.StatusCode)
		os.Exit(1)
	}
}

func generateConfig(ctx context.Context, configPath string, wgC testcontainers.Container) error {
	wgHost, err := wgC.Host(ctx)
	if err != nil {
		return err
	}

	wgPort, err := wgC.MappedPort(ctx, "51820/udp")
	if err != nil {
		return err
	}

	var renderedConfig strings.Builder
	tmpl := template.Must(template.ParseFiles("testdata/noisysockets.yaml.tmpl"))
	if err := tmpl.Execute(&renderedConfig, struct {
		Endpoint string
	}{
		Endpoint: wgHost + ":" + wgPort.Port(),
	}); err != nil {
		return err
	}

	return os.WriteFile(configPath, []byte(renderedConfig.String()), 0o400)
}
