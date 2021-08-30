package config

import (
	dockerclient "github.com/docker/docker/client"
)

func NewDockerClient() (*dockerclient.Client, error) {
	return dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
}
