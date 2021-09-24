package plugin

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/mitchellh/mapstructure"
	"github.com/outblocks/cli-plugin-docker/templates"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

const (
	AppRunDockerfile    = "Dockerfile.dev"
	AppRunDockerCompose = "docker-compose.outblocks.yaml"
)

type AppType int

const (
	AppTypeUnknown AppType = iota + 1
	AppTypeNodeNPM
	AppTypeNodeYarn
)

type AppRunInfo struct {
	*types.AppRun
	Hosts   map[string]string
	Type    AppType
	Options *AppRunOptions
}

type AppRunOptions struct {
	DockerCommand string `mapstructure:"docker_command"`
}

func (o *AppRunOptions) Decode(in interface{}) error {
	return mapstructure.Decode(in, o)
}

func detectAppType(app *types.AppRun) AppType {
	if plugin_util.FileExists(filepath.Join(app.App.Dir, "package.json")) {
		if plugin_util.FileExists(filepath.Join(app.App.Dir, "yarn.lock")) {
			return AppTypeNodeYarn
		}

		return AppTypeNodeNPM
	}

	return AppTypeUnknown
}

func NewAppRunInfo(app *types.AppRun, hosts map[string]string) (*AppRunInfo, error) {
	opts := &AppRunOptions{}
	if err := opts.Decode(app.Properties); err != nil {
		return nil, err
	}

	return &AppRunInfo{
		AppRun:  app,
		Hosts:   hosts,
		Type:    detectAppType(app),
		Options: opts,
	}, nil
}

func (a *AppRunInfo) SanitizedAppName() string {
	return plugin_util.LimitString(plugin_util.SanitizeName(a.App.Name), 51)
}

func (a *AppRunInfo) Name() string {
	return fmt.Sprintf("%s_%s", a.App.Type, a.SanitizedAppName())
}

func (a *AppRunInfo) DockerPath() string {
	return "/app"
}

func (a *AppRunInfo) WorkDir() string {
	return "/app"
}

func (a *AppRunInfo) Dockerfile() string {
	return "Dockerfile.dev"
}

func (a *AppRunInfo) DockerfilePath() string {
	return filepath.Join(a.App.Dir, AppRunDockerfile)
}

func (a *AppRunInfo) DockerComposePath() string {
	return filepath.Join(a.App.Dir, AppRunDockerCompose)
}

func (a *AppRunInfo) Volumes() map[string]string {
	return map[string]string{
		a.Name() + "_node_modules": a.DockerPath() + "/node_modules",
	}
}

func (a *AppRunInfo) Env() map[string]string {
	prefix := a.App.EnvPrefix()
	m := make(map[string]string)

	for k, v := range a.AppRun.Env {
		if strings.HasPrefix(k, prefix) {
			continue
		}

		m[k] = v
	}

	return m
}

func (a *AppRunInfo) DockerCommand() string {
	cmd := a.Options.DockerCommand
	if cmd == "" {
		cmd = a.Command
	}

	switch a.Type {
	case AppTypeNodeYarn:
		return fmt.Sprintf("yarn install && %s", cmd)
	case AppTypeNodeNPM:
		return fmt.Sprintf("npm install && %s", cmd)
	case AppTypeUnknown:
	}

	return cmd
}

func (a *AppRunInfo) DockerfileYAML() ([]byte, error) {
	var (
		dockerfileYAML bytes.Buffer
		templ          *template.Template
	)

	switch a.Type {
	case AppTypeNodeNPM, AppTypeNodeYarn:
		templ = templates.DockerfileAppNodeTemplate()
	case AppTypeUnknown:
		return nil, fmt.Errorf("unsupported app for dockerfile generation")
	}

	err := templ.Execute(&dockerfileYAML, a)

	return dockerfileYAML.Bytes(), err
}

func matchAppOutput(appMatchers []*regexp.Regexp, apps []*AppRunInfo, t string) *plugin_go.RunOutputResponse {
	for i, m := range appMatchers {
		idx := m.FindStringIndex(t)
		if idx != nil {
			return &plugin_go.RunOutputResponse{
				Source:  plugin_go.RunOutpoutSourceApp,
				ID:      apps[i].App.ID,
				Name:    apps[i].App.Name,
				Message: t[idx[1]:],
			}
		}
	}

	return nil
}
