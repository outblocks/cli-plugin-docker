package main

import (
	"github.com/outblocks/cli-plugin-docker/plugin"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func main() {
	s := plugin_go.NewServer()
	p := plugin.NewPlugin(s.Log(), s.Env())
	s.Start(p.Handler())
}
