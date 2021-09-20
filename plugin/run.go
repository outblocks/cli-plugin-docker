package plugin

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/outblocks/cli-plugin-docker/templates"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

const (
	commandCleanupTimeout = 10 * time.Second
)

type RunOptions struct {
	NoCache    bool `mapstructure:"docker-no-cache"`
	Rebuild    bool `mapstructure:"docker-rebuild"`
	Regenerate bool `mapstructure:"docker-regenerate"`
}

func (o *RunOptions) Decode(in interface{}) error {
	return mapstructure.Decode(in, o)
}

func (p *Plugin) prepareApps(apps []*types.AppRun, hosts map[string]string) ([]*AppRun, error) {
	appInfos := make([]*AppRun, len(apps))

	var err error

	for i, app := range apps {
		appInfos[i], err = NewAppInfo(app, hosts)
		if err != nil {
			return nil, err
		}
	}

	return appInfos, nil
}

func (p *Plugin) generateDockerFiles(apps []*AppRun, opts *RunOptions) error {
	for _, app := range apps {
		dockerComposePath := app.DockerComposePath()
		dockerfilePath := app.DockerfilePath()

		// Generate docker compose.
		if !plugin_util.FileExists(dockerComposePath) || opts.Regenerate {
			var dockerComposeYAML bytes.Buffer

			err := templates.DockerComposeAppTemplate().Execute(&dockerComposeYAML, app)
			if err != nil {
				return err
			}

			err = plugin_util.WriteFile(dockerComposePath, dockerComposeYAML.Bytes(), 0644)
			if err != nil {
				return err
			}
		}

		// Generate dockerfile.
		if !plugin_util.FileExists(dockerfilePath) || opts.Regenerate {
			dockerfileYAML, err := app.DockerfileYAML()
			if err != nil {
				return err
			}

			err = plugin_util.WriteFile(dockerfilePath, dockerfileYAML, 0644)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Plugin) runCommand(ctx context.Context, cmdStr string, env []string) error {
	cmd := plugin_util.NewCmdAsUser(cmdStr)

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

func (p *Plugin) RunInteractive(ctx context.Context, r *plugin_go.RunRequest, stream *plugin_go.ReceiverStream) error {
	opts := &RunOptions{}
	if err := opts.Decode(r.Args); err != nil {
		return err
	}

	apps, err := p.prepareApps(r.Apps, r.Hosts)
	if err != nil {
		return err
	}

	if err := p.generateDockerFiles(apps, opts); err != nil {
		return err
	}

	var commonEnv []string

	dockerComposeFiles := make([]string, len(apps))
	appMatchers := make([]*regexp.Regexp, len(apps))

	for i, app := range apps {
		envPrefix := app.App.EnvPrefix()

		for k, v := range app.AppRun.Env {
			if strings.HasPrefix(k, envPrefix) {
				commonEnv = append(commonEnv, fmt.Sprintf("%s=%s", k, v))
			}
		}

		dockerComposeFiles[i] = app.DockerComposePath()
		appMatchers[i] = regexp.MustCompile(fmt.Sprintf(`^%s(_\d)?\s+\|\s`, app.Name()))
	}

	// Run docker-compose build if needed.
	if opts.Rebuild {
		cmdStr := fmt.Sprintf("%s -f %s build", p.dockerComposeCmd, strings.Join(dockerComposeFiles, "-f "))

		if opts.NoCache {
			cmdStr += " --no-cache"
		}

		err = p.runCommand(ctx, cmdStr, commonEnv)
		if err != nil {
			return err
		}
	}

	// Run combined docker-compose.
	cmdStr := fmt.Sprintf("%s -f %s up --no-color", p.dockerComposeCmd, strings.Join(dockerComposeFiles, "-f "))
	cmd := plugin_util.NewCmdAsUser(cmdStr)

	cmd.Env = append(os.Environ(), commonEnv...)
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
			t := s.Text()

			out := matchAppOutput(appMatchers, apps, t)
			if out != nil {
				_ = stream.Send(out)

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

		_ = cmd.Process.Signal(syscall.SIGINT)

		go func() {
			time.Sleep(commandCleanupTimeout)

			_ = cmd.Process.Signal(syscall.SIGKILL)
		}()
	}()

	err = stream.Send(&plugin_go.RunningResponse{})
	if err != nil {
		return err
	}

	wg.Wait()

	return nil
}
