package main

import (
	"github.com/outblocks/cli-plugin-docker/plugin"
	comm "github.com/outblocks/outblocks-plugin-go"
)

func main() {
	s := comm.NewServer()
	p := plugin.NewPlugin(s.Log(), s.Env())
	s.Start(p.Handler())
}
