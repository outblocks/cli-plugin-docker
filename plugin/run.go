package plugin

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/outblocks/cli-plugin-docker/templates"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/outblocks/outblocks-plugin-go/util/command"
)

const (
	commandCleanupTimeout = 10 * time.Second
)

type RunOptions struct {
	NoCache    bool `mapstructure:"docker-no-cache"`
	Rebuild    bool `mapstructure:"docker-rebuild"`
	Regenerate bool `mapstructure:"docker-regenerate"`
}

func (o *RunOptions) Decode(in map[string]interface{}) error {
	return mapstructure.WeakDecode(in, o)
}

func (p *Plugin) prepareApps(apps []*apiv1.AppRun, hosts map[string]string) ([]*AppRunInfo, error) {
	appInfos := make([]*AppRunInfo, len(apps))

	var err error

	for i, app := range apps {
		appInfos[i], err = NewAppRunInfo(app, hosts)
		if err != nil {
			return nil, err
		}
	}

	return appInfos, nil
}

func (p *Plugin) prepareDependencies(deps []*apiv1.DependencyRun, hosts map[string]string) ([]*DependencyRunInfo, error) {
	depInfos := make([]*DependencyRunInfo, len(deps))

	var err error

	for i, dep := range deps {
		depInfos[i], err = NewDependencyRunInfo(dep, hosts, p.env)
		if err != nil {
			return nil, err
		}
	}

	return depInfos, nil
}

func (p *Plugin) generateDockerFiles(apps []*AppRunInfo, deps []*DependencyRunInfo, opts *RunOptions) error {
	for _, app := range apps {
		dockerComposePath := app.DockerComposePath()
		dockerfilePath := app.DockerfilePath()

		// Generate docker compose.
		var dockerComposeYAML bytes.Buffer

		err := templates.DockerComposeAppTemplate().Execute(&dockerComposeYAML, app)
		if err != nil {
			return err
		}

		err = os.WriteFile(dockerComposePath, dockerComposeYAML.Bytes(), 0o644)
		if err != nil {
			return err
		}

		// Generate dockerfile.
		if app.Type == AppTypeUnknown && plugin_util.FileExists(dockerfilePath) {
			continue
		}

		if !plugin_util.FileExists(dockerfilePath) || opts.Regenerate {
			dockerfileYAML, err := app.DockerfileYAML()
			if err != nil {
				return err
			}

			err = os.WriteFile(dockerfilePath, dockerfileYAML, 0o644)
			if err != nil {
				return err
			}
		}
	}

	for _, dep := range deps {
		dockerComposePath := dep.DockerComposePath()

		// Generate docker compose.
		var dockerComposeYAML bytes.Buffer

		err := templates.DockerComposeDependencyTemplate().Execute(&dockerComposeYAML, dep)
		if err != nil {
			return err
		}

		err = os.WriteFile(dockerComposePath, dockerComposeYAML.Bytes(), 0o644)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Plugin) runCommand(ctx context.Context, cmdStr string, env []string) error {
	cmd := command.NewCmdAsUser(cmdStr)

	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = p.env.ProjectDir()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	// Process stdout and stderr.
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		s := bufio.NewScanner(stdoutPipe)

		for s.Scan() {
			p.log.Infoln(s.Text())
		}

		wg.Done()
	}()

	go func() {
		s := bufio.NewScanner(stderrPipe)
		for s.Scan() {
			p.log.Infoln(s.Text())
		}

		wg.Done()
	}()

	go func() {
		<-ctx.Done()

		_ = cmd.Process.Signal(syscall.SIGINT)

		go func() {
			time.Sleep(commandCleanupTimeout)

			_ = cmd.Process.Signal(syscall.SIGKILL)
		}()
	}()

	wg.Wait()

	return nil
}
func (p *Plugin) Run(r *apiv1.RunRequest, stream apiv1.RunPluginService_RunServer) error {
	ctx := stream.Context()

	opts := &RunOptions{}
	if err := opts.Decode(r.Args.AsMap()); err != nil {
		return err
	}

	vars := make(map[string]*apiv1.RunVars)

	apps, err := p.prepareApps(r.Apps, r.Hosts)
	if err != nil {
		return err
	}

	deps, err := p.prepareDependencies(r.Dependencies, r.Hosts)
	if err != nil {
		return err
	}

	if err := p.generateDockerFiles(apps, deps, opts); err != nil {
		return err
	}

	var commonEnv []string

	dockerComposeFiles := make([]string, len(apps)+len(deps))
	appMatchers := make([]*regexp.Regexp, len(apps))
	depMatchers := make([]*regexp.Regexp, len(deps))

	for i, app := range apps {
		envPrefix := types.AppEnvPrefix(app.App)

		for k, v := range app.AppRun.App.Env {
			if strings.HasPrefix(k, envPrefix) {
				commonEnv = append(commonEnv, fmt.Sprintf("%s=%s", k, v))
			}
		}

		dockerComposeFiles[i] = app.DockerComposePath()
		appMatchers[i] = regexp.MustCompile(fmt.Sprintf(`^(.*-)?%s([-_]\d)?\s+\|\s`, app.Name()))
		vars[app.App.Id] = &apiv1.RunVars{Vars: app.Vars()}
	}

	for i, dep := range deps {
		dockerComposeFiles[i] = dep.DockerComposePath()
		depMatchers[i] = regexp.MustCompile(fmt.Sprintf(`^(.*-)?%s([-_]\d)?\s+\|\s`, dep.Name()))
		vars[dep.Dependency.Id] = &apiv1.RunVars{Vars: dep.Vars()}
	}

	// Run docker-compose build if needed.
	if opts.Rebuild {
		cmdStr := fmt.Sprintf("%s -f %s build", strings.Join(p.dockerComposeCmd, " "), strings.Join(dockerComposeFiles, " -f "))

		if opts.NoCache {
			cmdStr += " --no-cache"
		}

		err = p.runCommand(ctx, cmdStr, commonEnv)
		if err != nil {
			return err
		}
	}

	// Run combined docker-compose.
	cmdArgs := []string{}

	for _, f := range dockerComposeFiles {
		cmdArgs = append(cmdArgs, "-f", f)
	}

	cmdArgs = append(cmdArgs, "up", "--no-color", "--remove-orphans")

	cmd, err := command.New(
		exec.Command(p.dockerComposeCmd[0], append(p.dockerComposeCmd[1:], cmdArgs...)...),
		command.WithDir(p.env.ProjectDir()), command.WithEnv(commonEnv))
	if err != nil {
		return err
	}

	stdoutPipe := cmd.Stdout()
	stderrPipe := cmd.Stderr()

	err = cmd.Run()
	if err != nil {
		return err
	}

	// Process stdout and stderr.
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		s := bufio.NewScanner(stdoutPipe)

		for s.Scan() {
			t := s.Text()

			out := matchOutput(appMatchers, depMatchers, apps, deps, t)
			if out != nil {
				_ = stream.Send(&apiv1.RunResponse{
					Response: &apiv1.RunResponse_Output{
						Output: out,
					},
				})

				continue
			}

			p.log.Infoln(t)
		}

		wg.Done()
	}()

	go func() {
		s := bufio.NewScanner(stderrPipe)
		for s.Scan() {
			p.log.Infoln(s.Text())
		}

		wg.Done()
	}()

	go func() {
		<-ctx.Done()

		_ = cmd.Stop(commandCleanupTimeout)
		_ = cmd.Wait()
	}()

	err = stream.Send(&apiv1.RunResponse{
		Response: &apiv1.RunResponse_Start{
			Start: &apiv1.RunStartResponse{
				Vars: vars,
			},
		},
	})
	if err != nil {
		return err
	}

	wg.Wait()

	return nil
}

func matchOutput(appMatchers, depMatchers []*regexp.Regexp, apps []*AppRunInfo, deps []*DependencyRunInfo, t string) *apiv1.RunOutputResponse {
	for i, m := range appMatchers {
		idx := m.FindStringIndex(t)
		if idx != nil {
			return &apiv1.RunOutputResponse{
				Source:  apiv1.RunOutputResponse_SOURCE_APP,
				Id:      apps[i].App.Id,
				Name:    apps[i].App.Name,
				Message: t[idx[1]:],
			}
		}
	}

	for i, m := range depMatchers {
		idx := m.FindStringIndex(t)
		if idx != nil {
			return &apiv1.RunOutputResponse{
				Source:  apiv1.RunOutputResponse_SOURCE_DEPENDENCY,
				Id:      deps[i].Dependency.Id,
				Name:    deps[i].Dependency.Name,
				Message: t[idx[1]:],
			}
		}
	}

	return nil
}
