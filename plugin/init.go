package plugin

import (
	"context"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (p *Plugin) ProjetInit(ctx context.Context, r *plugin_go.ProjectInitRequest) (plugin_go.Response, error) {
	return &plugin_go.EmptyResponse{}, nil
}
