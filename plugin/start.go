package plugin

import (
	"context"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (p *Plugin) Start(ctx context.Context, r *plugin_go.StartRequest) (plugin_go.Response, error) {
	return &plugin_go.EmptyResponse{}, nil
}
