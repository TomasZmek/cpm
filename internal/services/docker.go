package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

// DiscoveredPort represents a single port mapping on a discovered container.
type DiscoveredPort struct {
	PrivatePort uint16
	PublicPort  uint16
}

// DiscoveredContainer represents a running container eligible for auto-discovery.
// Containers with multiple ports are returned as multiple entries (one per port).
type DiscoveredContainer struct {
	Name  string
	State string
	IP    string
	Ports []DiscoveredPort
}

// DockerService handles Docker container operations
type DockerService struct {
	containerName string
	client        *client.Client
}

// NewDockerService creates a new Docker service
func NewDockerService(containerName string) *DockerService {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		// Log error but continue - Docker might not be available
		log.Printf("Warning: Could not connect to Docker: %v", err)
	}

	return &DockerService{
		containerName: containerName,
		client:        cli,
	}
}

// IsAvailable checks if Docker is available
func (d *DockerService) IsAvailable() bool {
	return d.client != nil
}

// Ping verifies that a Docker daemon is actually reachable (not just that a
// client was constructed). Used at startup to auto-register the local host.
func (d *DockerService) Ping() bool {
	if d.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := d.client.ContainerList(ctx, client.ContainerListOptions{}); err != nil {
		return false
	}
	return true
}

// GetContainerID finds the container ID by name
func (d *DockerService) GetContainerID() (string, error) {
	if d.client == nil {
		return "", fmt.Errorf("Docker client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := d.client.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range result.Items {
		for _, name := range c.Names {
			// Container names start with /
			if name == "/"+d.containerName || name == d.containerName {
				return c.ID, nil
			}
		}
	}

	return "", fmt.Errorf("container '%s' not found", d.containerName)
}

// IsContainerRunning checks if the Caddy container is running
func (d *DockerService) IsContainerRunning() bool {
	if d.client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	containerID, err := d.GetContainerID()
	if err != nil {
		return false
	}

	inspect, err := d.client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return false
	}

	return inspect.Container.State.Running
}

// ReloadCaddy reloads Caddy configuration
func (d *DockerService) ReloadCaddy() error {
	output, err := d.ExecCommandWithOutput("caddy", "reload", "--config", "/etc/caddy/Caddyfile")
	if err != nil {
		return fmt.Errorf("reload failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// ReloadCaddyWithOutput reloads Caddy and returns output for debugging
func (d *DockerService) ReloadCaddyWithOutput() (string, error) {
	return d.ExecCommandWithOutput("caddy", "reload", "--config", "/etc/caddy/Caddyfile")
}

// ReloadCaddyForceWithOutput forces a config reload even when the config is
// unchanged, so Caddy re-provisions and re-issues any missing certificates.
func (d *DockerService) ReloadCaddyForceWithOutput() (string, error) {
	return d.ExecCommandWithOutput("caddy", "reload", "--config", "/etc/caddy/Caddyfile", "--force")
}

// ValidateConfig validates Caddy configuration
func (d *DockerService) ValidateConfig() error {
	output, err := d.ExecCommandWithOutput("caddy", "validate", "--config", "/etc/caddy/Caddyfile")
	if err != nil {
		return fmt.Errorf("validation failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// ValidateConfigWithOutput validates Caddy configuration and returns output
func (d *DockerService) ValidateConfigWithOutput() (string, error) {
	return d.ExecCommandWithOutput("caddy", "validate", "--config", "/etc/caddy/Caddyfile")
}

// ExecCommand executes a command inside the container (legacy, returns only error)
func (d *DockerService) ExecCommand(cmd ...string) error {
	_, err := d.ExecCommandWithOutput(cmd...)
	return err
}

// ExecCommandWithOutput executes a command inside the container and returns output
// ExecCommandWithOutput runs a command in the Caddy container. If the Docker
// connection has gone stale (e.g. Docker Desktop restarted), it transparently
// reconnects and retries once.
func (d *DockerService) ExecCommandWithOutput(cmd ...string) (string, error) {
	out, err := d.execCommandOnce(cmd...)
	if err != nil && isDockerUnreachable(err) {
		d.reconnect()
		out, err = d.execCommandOnce(cmd...)
	}
	return out, err
}

// reconnect recreates the Docker client (used after the previous connection
// dropped, e.g. Docker Desktop restarted).
func (d *DockerService) reconnect() {
	if cli, e := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation()); e == nil {
		d.client = cli
	}
}

// isDockerUnreachable reports whether err is a Docker daemon connection failure.
func isDockerUnreachable(err error) bool {
	if err == nil {
		return false
	}
	m := err.Error()
	return strings.Contains(m, "connect to the docker API") ||
		strings.Contains(m, "Cannot connect to the Docker daemon") ||
		strings.Contains(m, "pipe/docker_engine") ||
		strings.Contains(m, "docker API at") ||
		strings.Contains(m, "Docker client not available")
}

func (d *DockerService) execCommandOnce(cmd ...string) (string, error) {
	if d.client == nil {
		return "", fmt.Errorf("Docker client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerID, err := d.GetContainerID()
	if err != nil {
		return "", err
	}

	execConfig := client.ExecCreateOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := d.client.ExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := d.client.ExecAttach(ctx, execID.ID, client.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach exec: %w", err)
	}
	defer resp.Close()

	// Read output
	output, _ := io.ReadAll(resp.Reader)
	outputStr := cleanDockerOutput(string(output))

	// Check exit code
	inspect, err := d.client.ExecInspect(ctx, execID.ID, client.ExecInspectOptions{})
	if err != nil {
		return outputStr, fmt.Errorf("failed to inspect exec: %w", err)
	}

	if inspect.ExitCode != 0 {
		return outputStr, fmt.Errorf("command failed with exit code %d", inspect.ExitCode)
	}

	return outputStr, nil
}

// cleanDockerOutput removes Docker log header bytes from output.
// Docker multiplexes stdout and stderr streams. Each chunk has an 8-byte header:
// [0] is STREAM_TYPE (0=stdin, 1=stdout, 2=stderr)
// [1..3] are 0
// [4..7] is SIZE (big-endian uint32)
func cleanDockerOutput(output string) string {
	var buf bytes.Buffer
	data := []byte(output)

	for len(data) > 0 {
		if len(data) < 8 {
			buf.Write(data)
			break
		}

		streamType := data[0]
		// Docker stream types: 0=stdin, 1=stdout, 2=stderr
		if streamType > 2 || data[1] != 0 || data[2] != 0 || data[3] != 0 {
			// Not a valid docker header, treat rest as raw text
			buf.Write(data)
			break
		}

		// import "encoding/binary" is needed, we'll add it
		size := uint32(data[4])<<24 | uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7])
		data = data[8:]

		if len(data) < int(size) {
			buf.Write(data)
			break
		}

		buf.Write(data[:size])
		data = data[size:]
	}

	return buf.String()
}

// GetLogs retrieves container logs
func (d *DockerService) GetLogs(lines int) ([]string, error) {
	if d.client == nil {
		return nil, fmt.Errorf("Docker client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	containerID, err := d.GetContainerID()
	if err != nil {
		return nil, err
	}

	options := client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", lines),
		Timestamps: true,
	}

	logs, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}
	defer logs.Close()

	content, err := io.ReadAll(logs)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs: %w", err)
	}

	// Use the fixed cleanDockerOutput instead of manual line splitting
	cleanedContent := cleanDockerOutput(string(content))
	rawLines := strings.Split(cleanedContent, "\n")
	
	var cleanLines []string
	for _, line := range rawLines {
		if len(line) > 0 {
			cleanLines = append(cleanLines, line)
		}
	}

	return cleanLines, nil
}

// ContainerStates returns a map of container name -> state ("running", "exited", ...)
// for all containers (running and stopped). Used to show backend health per site.
func (d *DockerService) ContainerStates() (map[string]string, error) {
	if d.client == nil {
		return nil, fmt.Errorf("Docker client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := d.client.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	states := make(map[string]string)
	for _, c := range result.Items {
		for _, n := range c.Names {
			states[strings.TrimPrefix(n, "/")] = string(c.State)
		}
	}
	return states, nil
}

// ListDiscoverableContainers returns running containers suitable for auto-discovery.
// Caddy (d.containerName) and the CPM container itself are excluded.
// Containers with N ports are returned as N separate entries (one per port).
func (d *DockerService) ListDiscoverableContainers() ([]DiscoveredContainer, error) {
	if d.client == nil {
		return nil, fmt.Errorf("Docker client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	selfHostname, _ := os.Hostname()

	result, err := d.client.ContainerList(ctx, client.ContainerListOptions{All: false})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	type portKey struct {
		name        string
		privatePort uint16
	}
	seen := make(map[portKey]bool)

	var discovered []DiscoveredContainer
	for _, c := range result.Items {
		if c.State != "running" {
			continue
		}

		name := ""
		for _, n := range c.Names {
			name = strings.TrimPrefix(n, "/")
			break
		}

		if name == d.containerName {
			continue
		}

		if selfHostname != "" && strings.HasPrefix(c.ID, selfHostname) {
			continue
		}

		// Container IP (first network with an address).
		ip := ""
		if c.NetworkSettings != nil {
			for _, ep := range c.NetworkSettings.Networks {
				if ep.IPAddress.IsValid() {
					ip = ep.IPAddress.String()
					break
				}
			}
		}

		if len(c.Ports) == 0 {
			key := portKey{name: name, privatePort: 0}
			if !seen[key] {
				seen[key] = true
				discovered = append(discovered, DiscoveredContainer{
					Name:  name,
					State: "running",
					IP:    ip,
					Ports: []DiscoveredPort{},
				})
			}
			continue
		}

		for _, p := range c.Ports {
			key := portKey{name: name, privatePort: p.PrivatePort}
			if seen[key] {
				continue
			}
			seen[key] = true
			discovered = append(discovered, DiscoveredContainer{
				Name:  name,
				State: "running",
				IP:    ip,
				Ports: []DiscoveredPort{{PrivatePort: p.PrivatePort, PublicPort: p.PublicPort}},
			})
		}
	}

	return discovered, nil
}

// GetContainerStatus returns the container status
func (d *DockerService) GetContainerStatus() string {
	if d.client == nil {
		return "unknown"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	containerID, err := d.GetContainerID()
	if err != nil {
		return "not found"
	}

	inspect, err := d.client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return "error"
	}

	return string(inspect.Container.State.Status)
}
