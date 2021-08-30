package plugin

import (
	"context"
	"fmt"

	"github.com/outblocks/cli-plugin-docker/internal/config"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func (p *Plugin) findDockerComposeCmd() string {
	cmd := plugin_util.NewCmdAsUser("docker compose version")
	if cmd.Run() == nil {
		return "docker compose"
	}

	cmd = plugin_util.NewCmdAsUser("docker-compose version")
	if cmd.Run() == nil {
		return "docker-compose"
	}

	return ""
}

func (p *Plugin) Start(ctx context.Context, r *plugin_go.StartRequest) (plugin_go.Response, error) {
	var err error

	// Check docker connection.
	p.cli, err = config.NewDockerClient()
	if err != nil {
		return nil, fmt.Errorf("error setting up docker client")
	}

	_, err = p.cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("error connecting to docker: %w", err)
	}

	p.dockerComposeCmd = p.findDockerComposeCmd()
	if p.dockerComposeCmd == "" {
		return nil, fmt.Errorf("docker-compose not found")
	}

	return &plugin_go.EmptyResponse{}, nil
}
