// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Container represents a partial Docker container object, focused on network details.
type Container struct {
	ID              string            `json:"Id"`
	Names           []string          `json:"Names"`
	Image           string            `json:"Image"`
	State           string            `json:"State"`
	Status          string            `json:"Status"`
	NetworkSettings NetworkSettings   `json:"NetworkSettings"`
	Labels          map[string]string `json:"Labels"`
}

type NetworkSettings struct {
	Networks map[string]NetworkEndpoint `json:"Networks"`
}

type NetworkEndpoint struct {
	IPAddress  string `json:"IPAddress"`
	Gateway    string `json:"Gateway"`
	MacAddress string `json:"MacAddress"`
	NetworkID  string `json:"NetworkID"`
	EndpointID string `json:"EndpointID"`
}

// DockerClient is a lightweight client for the Docker Unix socket.
type DockerClient struct {
	client     *http.Client
	socketPath string
	mockMode   bool
}

// NewDockerClient creates a new client connected to the default socket.
func NewDockerClient(socketPath string) *DockerClient {
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	return &DockerClient{
		socketPath: socketPath,
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
			Timeout: 5 * time.Second,
		},
	}
}

// NewMockDockerClient creates a client that returns static dummy data (for QA/Dev).
func NewMockDockerClient() *DockerClient {
	return &DockerClient{
		mockMode: true,
	}
}

// ListContainers returns a list of active containers.
func (c *DockerClient) ListContainers(ctx context.Context) ([]Container, error) {
	if c.mockMode {
		return []Container{
			{
				ID:    "1234567890ab",
				Names: []string{"/web-server"},
				Image: "nginx:latest",
				State: "running",
				NetworkSettings: NetworkSettings{
					Networks: map[string]NetworkEndpoint{
						"bridge": {IPAddress: "172.17.0.2"},
					},
				},
			},
			{
				ID:    "abcdef123456",
				Names: []string{"/db-redis"},
				Image: "redis:alpine",
				State: "running",
				NetworkSettings: NetworkSettings{
					Networks: map[string]NetworkEndpoint{
						"bridge": {IPAddress: "172.17.0.3"},
					},
				},
			},
		}, nil
	}
	// http://unix/containers/json?all=1
	req, err := http.NewRequestWithContext(ctx, "GET", "http://unix/containers/json?all=0", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker socket request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return parseContainers(resp.Body)
}

func parseContainers(r io.Reader) ([]Container, error) {
	var containers []Container
	if err := json.NewDecoder(r).Decode(&containers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return containers, nil
}
