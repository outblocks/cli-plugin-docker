package plugin

import (
	"context"

	comm "github.com/outblocks/outblocks-plugin-go"
)

func (p *Plugin) Start(ctx context.Context, r *comm.StartRequest) (comm.Response, error) {
	return &comm.EmptyResponse{}, nil
}
