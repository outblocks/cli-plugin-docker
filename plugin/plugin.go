package plugin

import (
	dockerclient "github.com/docker/docker/client"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/env"
	"github.com/outblocks/outblocks-plugin-go/log"
)

type Plugin struct {
	log log.Logger
	env env.Enver

	cli              *dockerclient.Client
	dockerComposeCmd string
}

func NewPlugin(logger log.Logger, enver env.Enver) *Plugin {
	return &Plugin{
		log: logger,
		env: enver,
	}
}

func (p *Plugin) Handler() *plugin_go.ReqHandler {
	return &plugin_go.ReqHandler{
		Init:           p.Init,
		Start:          p.Start,
		RunInteractive: p.RunInteractive,
	}
}
