package plugin

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/outblocks/cli-plugin-docker/templates"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/outblocks/outblocks-plugin-go/util/command"
)

const (
	DefaultDockerfile   = "Dockerfile"
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
	*apiv1.AppRun
	Hosts      map[string]string
	Type       AppType
	Options    *AppRunOptions
	Production bool
}

type AppRunOptions struct {
	DockerEntrypoint *command.StringCommand     `json:"docker_entrypoint"`
	DockerCommand    *command.StringCommand     `json:"docker_command"`
	DockerPort       int                        `json:"docker_port"`
	DockerWorkdir    *string                    `json:"docker_workdir"`
	Dockerfile       *string                    `json:"dockerfile"`
	Container        *types.ServiceAppContainer `json:"container"`
}

func (o *AppRunOptions) Decode(in map[string]any) error {
	return plugin_util.MapstructureJSONDecode(in, o)
}

func detectAppType(app *apiv1.AppRun) AppType {
	if plugin_util.FileExists(filepath.Join(app.App.Dir, "package.json")) {
		if plugin_util.FileExists(filepath.Join(app.App.Dir, "yarn.lock")) {
			return AppTypeNodeYarn
		}

		return AppTypeNodeNPM
	}

	return AppTypeUnknown
}

func NewAppRunInfo(app *apiv1.AppRun, hosts map[string]string, runOpts *RunOptions) (*AppRunInfo, error) {
	opts := &AppRunOptions{}
	if err := opts.Decode(app.Properties.AsMap()); err != nil {
		return nil, err
	}

	return &AppRunInfo{
		AppRun:     app,
		Hosts:      hosts,
		Type:       detectAppType(app),
		Options:    opts,
		Production: runOpts.Production,
	}, nil
}

func (a *AppRunInfo) SanitizedName() string {
	return plugin_util.LimitString(plugin_util.SanitizeName(a.App.Name, false, false), 51)
}

func (a *AppRunInfo) Name() string {
	return fmt.Sprintf("%s_%s", a.App.Type, a.SanitizedName())
}

func (a *AppRunInfo) dockerPath() string {
	return "/devapp"
}

func (a *AppRunInfo) WorkDir() string {
	if a.Options.DockerWorkdir != nil {
		return *a.Options.DockerWorkdir
	}

	if a.Production {
		return ""
	}

	return "/devapp"
}

func (a *AppRunInfo) Dockerfile() string {
	if a.Options.Dockerfile != nil {
		return *a.Options.Dockerfile
	}

	if a.Production {
		return DefaultDockerfile
	}

	return AppRunDockerfile
}

func (a *AppRunInfo) ContainerPort() int {
	if a.Options.DockerPort != 0 {
		return a.Options.DockerPort
	}

	if a.Options.Container != nil && a.Options.Container.Port != 0 {
		return a.Options.Container.Port
	}

	return int(a.Port)
}

func (a *AppRunInfo) DockerfilePath() string {
	return filepath.Join(a.App.Dir, a.Dockerfile())
}

func (a *AppRunInfo) DockerComposePath() string {
	return filepath.Join(a.App.Dir, fmt.Sprintf("%s.%s", a.App.Id, AppRunDockerCompose))
}

func (a *AppRunInfo) Volumes() map[string]string {
	if a.Production {
		return nil
	}

	volumes := map[string]string{
		".": a.dockerPath(),
	}

	switch a.Type {
	case AppTypeNodeNPM, AppTypeNodeYarn:
		volumes[a.Name()+"_node_modules"] = a.dockerPath() + "/node_modules"
	case AppTypeUnknown:
	}

	return volumes
}

func (a *AppRunInfo) Env() map[string]string {
	prefix := types.AppEnvPrefix(a.App)
	m := make(map[string]string)

	for k, v := range a.AppRun.App.Env {
		if strings.HasPrefix(k, prefix) {
			continue
		}

		m[k] = v
	}

	return m
}

func (a *AppRunInfo) Vars() map[string]string {
	return nil
}

func (a *AppRunInfo) DockerCommand() []string {
	cmd := command.NewStringCommandFromArray(a.Command)

	if !a.Options.DockerCommand.IsEmpty() {
		cmd = a.Options.DockerCommand
	} else if !a.Options.Container.Command.IsEmpty() {
		cmd = a.Options.Container.Command
	}

	if a.Production {
		return cmd.ArrayOrShell()
	}

	switch a.Type {
	case AppTypeNodeYarn:
		return []string{"sh", "-c", fmt.Sprintf("yarn install && %s", cmd.Flatten())}
	case AppTypeNodeNPM:
		return []string{"sh", "-c", fmt.Sprintf("npm install && %s", cmd.Flatten())}
	case AppTypeUnknown:
	}

	return cmd.ArrayOrShell()
}

func (a *AppRunInfo) DockerEntrypoint() []string {
	var cmd *command.StringCommand

	if !a.Options.DockerEntrypoint.IsEmpty() {
		cmd = a.Options.DockerEntrypoint
	} else if !a.Options.Container.Entrypoint.IsEmpty() {
		cmd = a.Options.Container.Entrypoint
	}

	return cmd.ArrayOrShell()
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
		return nil, fmt.Errorf("%s app '%s': unsupported app for dockerfile generation\nsupports: npm and yarn based node apps\ncreate dockerfile Dockerfile.dev manually", a.App.Type, a.App.Name)
	}

	err := templ.Execute(&dockerfileYAML, a)

	return dockerfileYAML.Bytes(), err
}
