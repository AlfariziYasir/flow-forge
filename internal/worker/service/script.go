package service

import (
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type ScriptAction struct {
	dockerClient *client.Client
}

func NewScriptAction() (*ScriptAction, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &ScriptAction{dockerClient: cli}, nil
}

func (s *ScriptAction) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	scriptCode, _ := params["script"].(string)
	runtime, _ := params["runtime"].(string)
	if scriptCode == "" {
		return nil, fmt.Errorf("missing 'script' parameter")
	}

	var imageName string
	var cmd []string

	switch runtime {
	case "python3":
		imageName = "python:3.10-slim"
		cmd = []string{"python", "-c", scriptCode}
	case "javascript":
		imageName = "node:18-alpine"
		cmd = []string{"node", "-e", scriptCode}
	case "shell":
		imageName = "ubuntu:22.04"
		cmd = []string{"bash", "-c", scriptCode}
	case "go":
		imageName = "golang:1.21-alpine"
		cmd = []string{"go", "run", "-e", scriptCode}
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	hostConfig := &container.HostConfig{
		AutoRemove: true,
		Resources: container.Resources{
			Memory:   128 * 1024 * 1024,
			NanoCPUs: 500000000,
		},
		NetworkMode: "none",
	}

	res, err := s.dockerClient.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Cmd:   cmd,
		Tty:   false,
	}, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox container: %v", err)
	}

	defer s.dockerClient.ContainerRemove(ctx, res.ID, container.RemoveOptions{Force: true})

	err = s.dockerClient.ContainerStart(ctx, res.ID, container.StartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start sandbox container: %v", err)
	}

	statusCh, errCh := s.dockerClient.ContainerWait(ctx, res.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("error waiting for container: %v", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 && status.StatusCode != 137 {
			return nil, fmt.Errorf("script execution failed with exit code: %d", status.StatusCode)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("script execution timed out and was killed")
	}

	logs, err := s.dockerClient.ContainerLogs(ctx, res.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %v", err)
	}
	defer logs.Close()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, logs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse logs: %v", err)
	}

	resultMap := map[string]any{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	if stderr.Len() > 0 {
		resultMap["has_warnings"] = true
	}

	return resultMap, nil
}
