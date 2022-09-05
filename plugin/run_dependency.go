package plugin

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/outblocks/outblocks-plugin-go/env"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"github.com/outblocks/outblocks-plugin-go/util/command"
)

type DependencyType int

const (
	DependencyTypeUnknown DependencyType = iota + 1
	DependencyTypePostgreSQL
	DependencyTypeMySQL
	DependencyTypeGCPStorage
)

type DependencyRunInfo struct {
	*apiv1.DependencyRun
	Hosts           map[string]string
	Type            DependencyType
	DatabaseOptions *types.DatabaseDepOptions
	StorageOptions  *types.StorageDepOptions
	Options         *DependencyRunOptions

	env env.Enver
}

type DependencyRunOptions struct {
	DockerCommand    *command.StringCommand `json:"docker_command"`
	DockerEntrypoint *command.StringCommand `json:"docker_entrypoint"`
	DockerEnv        map[string]string      `json:"docker_env"`
	DockerImage      string                 `json:"docker_image"`
	DockerPort       int                    `json:"docker_port"`
	DockerWorkdir    *string                `json:"docker_workdir"`
	DockerData       string                 `json:"docker_data"`
}

func (o *DependencyRunOptions) Decode(in map[string]interface{}) error {
	return plugin_util.MapstructureJSONDecode(in, o)
}

func detectDependencyType(dep *apiv1.DependencyRun) DependencyType {
	switch dep.Dependency.Type {
	case "postgresql":
		return DependencyTypePostgreSQL
	case "mysql":
		return DependencyTypeMySQL
	case "storage":
		if dep.Dependency.DeployPlugin == "gcp" {
			return DependencyTypeGCPStorage
		}
	}

	return DependencyTypeUnknown
}

func NewDependencyRunInfo(dep *apiv1.DependencyRun, hosts map[string]string, e env.Enver) (*DependencyRunInfo, error) {
	databaseOpts, err := types.NewDatabaseDepOptions(dep.Dependency.Properties.AsMap())
	if err != nil {
		return nil, err
	}

	storageOpts, err := types.NewStorageDepOptions(dep.Dependency.Properties.AsMap())
	if err != nil {
		return nil, err
	}

	runOpts := &DependencyRunOptions{}
	if err := runOpts.Decode(dep.Dependency.Properties.AsMap()); err != nil {
		return nil, err
	}

	if runOpts.DockerData == "" {
		runOpts.DockerData = "data"
	}

	return &DependencyRunInfo{
		DependencyRun:   dep,
		Hosts:           hosts,
		Type:            detectDependencyType(dep),
		DatabaseOptions: databaseOpts,
		StorageOptions:  storageOpts,
		Options:         runOpts,

		env: e,
	}, nil
}

func (a *DependencyRunInfo) SanitizedName() string {
	return plugin_util.LimitString(plugin_util.SanitizeName(a.Dependency.Name, false, false), 51)
}

func (a *DependencyRunInfo) DockerImage() string {
	if a.Options.DockerImage != "" {
		return a.Options.DockerImage
	}

	switch a.Type {
	case DependencyTypePostgreSQL:
		return fmt.Sprintf("postgres:%s", a.DatabaseOptions.Version)
	case DependencyTypeMySQL:
		return fmt.Sprintf("mysql:%s", a.DatabaseOptions.Version)
	case DependencyTypeGCPStorage:
		return "fsouza/fake-gcs-server"
	case DependencyTypeUnknown:
	}

	panic("unsupported dependency")
}

func (a *DependencyRunInfo) Name() string {
	return fmt.Sprintf("dep_%s", a.SanitizedName())
}

func (a *DependencyRunInfo) DockerPath() string {
	return ""
}

func (a *DependencyRunInfo) WorkDir() string {
	if a.Options.DockerWorkdir != nil {
		return *a.Options.DockerWorkdir
	}

	return ""
}

func (a *DependencyRunInfo) ContainerPort() int {
	if a.Options.DockerPort != 0 {
		return a.Options.DockerPort
	}

	switch a.Type {
	case DependencyTypePostgreSQL:
		return 5432
	case DependencyTypeMySQL:
		return 3306
	case DependencyTypeGCPStorage:
		return 4443
	case DependencyTypeUnknown:
		panic("unsupported dependency")
	}

	return int(a.Port)
}

func (a *DependencyRunInfo) DockerComposePath() string {
	return filepath.Join(a.env.PluginProjectCacheDir(), fmt.Sprintf("%s.%s", a.Dependency.Id, AppRunDockerCompose))
}

func (a *DependencyRunInfo) Volumes() map[string]string {
	switch a.Type {
	case DependencyTypePostgreSQL:
		return map[string]string{a.Options.DockerData: "/var/lib/postgresql/data"}
	case DependencyTypeMySQL:
		return map[string]string{a.Options.DockerData: "/var/lib/mysql"}
	case DependencyTypeGCPStorage:
		return map[string]string{a.Options.DockerData: "/data"}

	case DependencyTypeUnknown:
	}

	return nil
}

func (a *DependencyRunInfo) Env() map[string]string {
	m := make(map[string]string)

	user := "outblocks"
	password := "outblocks"
	db := "app"

	switch a.Type {
	case DependencyTypePostgreSQL:
		m["POSTGRES_USER"] = user
		m["POSTGRES_PASSWORD"] = password
		m["POSTGRES_DB"] = db

	case DependencyTypeMySQL:
		m["MYSQL_USER"] = user
		m["MYSQL_PASSWORD"] = password
		m["MYSQL_DATABASE"] = db

	case DependencyTypeGCPStorage:
	case DependencyTypeUnknown:
	}

	for k, v := range a.Options.DockerEnv {
		m[k] = v
	}

	return m
}

func (a *DependencyRunInfo) Vars() map[string]string {
	m := make(map[string]string)
	e := a.Env()

	switch a.Type {
	case DependencyTypePostgreSQL:
		m["user"] = e["POSTGRES_USER"]
		m["password"] = e["POSTGRES_PASSWORD"]
		m["database"] = e["POSTGRES_DB"]

	case DependencyTypeMySQL:
		m["user"] = e["MYSQL_USER"]
		m["password"] = e["MYSQL_PASSWORD"]
		m["database"] = e["MYSQL_DATABASE"]

	case DependencyTypeGCPStorage:
		m["name"] = a.StorageOptions.Name
		m["url"] = fmt.Sprintf("http://%s:%d/%s", a.Name(), a.ContainerPort(), a.StorageOptions.Name)
		m["local:url"] = fmt.Sprintf("http://localhost:%d/%s", a.Port, a.StorageOptions.Name)
		m["endpoint"] = fmt.Sprintf("http://%s:%d/storage/v1/", a.Name(), a.ContainerPort())
		m["local:endpoint"] = fmt.Sprintf("http://localhost:%d/storage/v1/", a.Port)

	case DependencyTypeUnknown:
	}

	m["post"] = strconv.Itoa(a.ContainerPort())
	m["local:port"] = strconv.Itoa(int(a.Port))
	m["host"] = a.Name()
	m["local:host"] = "localhost"

	return m
}

func (a *DependencyRunInfo) DockerCommand() []string {
	var cmd *command.StringCommand

	if a.Type == DependencyTypeGCPStorage {
		cmd = command.NewStringCommandFromArray([]string{"-scheme", "http"})
	}

	if !a.Options.DockerCommand.IsEmpty() {
		cmd = a.Options.DockerCommand
	}

	return cmd.ArrayOrShell()
}

func (a *DependencyRunInfo) DockerEntrypoint() []string {
	var cmd *command.StringCommand

	if !a.Options.DockerEntrypoint.IsEmpty() {
		cmd = a.Options.DockerEntrypoint
	}

	return cmd.ArrayOrShell()
}
