package plugin

import (
	dockerclient "github.com/docker/docker/client"
	plugin "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/env"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/log"
)

type Plugin struct {
	log     log.Logger
	env     env.Enver
	hostCli apiv1.HostServiceClient

	cli              *dockerclient.Client
	dockerComposeCmd []string
}

func NewPlugin() *Plugin {
	return &Plugin{}
}

var (
	_ plugin.RunPluginHandler = (*Plugin)(nil)
)
