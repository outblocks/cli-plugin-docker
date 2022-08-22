package plugin

import (
	"context"
	"fmt"

	"github.com/outblocks/cli-plugin-docker/internal/config"
	"github.com/outblocks/outblocks-plugin-go/env"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/log"
	"github.com/outblocks/outblocks-plugin-go/util/command"
)

func (p *Plugin) findDockerComposeCmd() []string {
	cmd := command.NewCmdAsUser("docker compose version")
	if cmd.Run() == nil {
		return []string{"docker", "compose"}
	}

	cmd = command.NewCmdAsUser("docker-compose version")
	if cmd.Run() == nil {
		return []string{"docker-compose"}
	}

	return nil
}

func (p *Plugin) Init(ctx context.Context, e env.Enver, l log.Logger, cli apiv1.HostServiceClient) error {
	p.env = e
	p.hostCli = cli
	p.log = l

	return nil
}

func (p *Plugin) ProjectInit(ctx context.Context, r *apiv1.ProjectInitRequest) (*apiv1.ProjectInitResponse, error) {
	return &apiv1.ProjectInitResponse{}, nil
}

func (p *Plugin) Start(ctx context.Context, r *apiv1.StartRequest) (*apiv1.StartResponse, error) {
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
	if p.dockerComposeCmd == nil {
		return nil, fmt.Errorf("docker-compose not found")
	}

	return &apiv1.StartResponse{}, nil
}
