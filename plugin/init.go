package plugin

import (
	"context"

	comm "github.com/outblocks/outblocks-plugin-go"
)

func (p *Plugin) Init(ctx context.Context, r *comm.InitRequest) (comm.Response, error) {
	return &comm.EmptyResponse{}, nil
}