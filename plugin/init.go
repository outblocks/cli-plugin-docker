package plugin

import (
	"context"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (p *Plugin) Init(ctx context.Context, r *plugin_go.InitRequest) (plugin_go.Response, error) {
	return &plugin_go.EmptyResponse{}, nil
}
