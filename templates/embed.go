package templates

import (
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig"
	"gopkg.in/yaml.v2"
)

var (
	//go:embed app.docker-compose.yaml.tpl
	DockerComposeApp string

	//go:embed dep.docker-compose.yaml.tpl
	DockerComposeDependency string

	//go:embed app-node.Dockerfile.tpl
	DockerfileAppNode string
)

func funcMap() template.FuncMap {
	return template.FuncMap{
		"toYaml": toYaml,
	}
}

func toYaml(v any) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}

	return string(data)
}

func lazyInit(name, tmpl string) func() *template.Template {
	var templ *template.Template

	return func() *template.Template {
		if templ == nil {
			templ = template.Must(template.New(name).Funcs(sprig.TxtFuncMap()).Funcs(funcMap()).Parse(tmpl))
		}

		return templ
	}
}

var (
	DockerComposeAppTemplate        = lazyInit("docker_compose:app", DockerComposeApp)
	DockerComposeDependencyTemplate = lazyInit("docker_compose:dep", DockerComposeDependency)
	DockerfileAppNodeTemplate       = lazyInit("dockerfile:app_node", DockerfileAppNode)
)
