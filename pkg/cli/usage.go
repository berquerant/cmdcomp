package cli

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

type usageBuilder struct{}

func (u usageBuilder) build() string {
	return strings.Join([]string{
		u.header(),
		u.usage(),
		u.examples(),
		u.configFile(),
		u.trailer(),
	}, "\n\n")
}

func (usageBuilder) header() string {
	return `cmdcomp -- compare the output of two commands with optional preprocessing and customizable diff`
}

func (usageBuilder) code(lang, s string) string {
	return fmt.Sprintf("```%s\n%s```", lang, s)
}

func (u usageBuilder) usage() string {
	return `## Usage

` + u.code("shell", "cmdcomp [flags] -- COMMON_ARGS [-- LEFT_ARGS [-- RIGHT_ARGS]]")
}

func (u usageBuilder) examples() string {
	return `## Examples

` + u.code("shell", `# echo a > leftfile
# echo b > rightfile
# diff leftfile rightfile
cmdcomp -- echo -- a -- b

# echo a > leftfile
# echo b > rightfile
# diff -u leftfile rightfile
cmdcomp -x 'diff -u' -- echo -- a -- b

# echo a > leftfile
# echo b > rightfile
# diff -u leftfile rightfile --label echo___a --label echo___b
cmdcomp -x 'diff -u' -l -- echo -- a -- b

# echo a | sed 's|a|c|' > leftfile
# echo b | sed 's|a|c|' > rightfile
# diff leftfile rightfile
cmdcomp -p 'sed "s|a|c|"' -- echo -- a -- b

# helm template datadog/datadog --version 3.68.0 | yq 'select(.kind=="Secret")' > leftfile
# helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug | yq 'select(.kind=="Secret")' > rightfile
# objdiff -c leftfile rightfile
cmdcomp -p "yq 'select(.kind==\"Secret\")'" -x 'objdiff -c' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

# helm template datadog/datadog --version 3.68.0 | yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json > leftfile
# helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug | yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json > rightfile
# npx jsondiffpatch --format=jsonpatch leftfile rightfile
cmdcomp -p "yq 'select(.kind==\"Deployment\" and .metadata.name==\"release-name-datadog-cluster-agent\")' -o json" -x 'npx jsondiffpatch --format=jsonpatch' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

# helm template datadog/datadog --version 3.68.0 | yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json | gron > leftfile
# helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug | yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json | gron > rightfile
# diff -u --color leftfile rightfile
cmdcomp -p "yq 'select(.kind==\"Deployment\" and .metadata.name==\"release-name-datadog-cluster-agent\")' -o json" -p 'gron' -x 'diff -u --color' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

# helm template ./charts/datadog > leftfile
# git checkout datadog-3.69.3
# helm template ./charts/datadog > rightfile
# objdiff -c leftfile rightfile
cmdcomp -i 'git checkout datadog-3.69.3' -x 'objdiff -c' -- helm template ./charts/datadog

# echo echo -- a > leftfile
# echo echo -- b > rightfile
# diff leftfile rightfile
cmdcomp -d '---' -- echo --- echo -- a --- echo -- b

# cmdcomp --success -- echo -- a -- b > leftfile
# cmdcomp --success -- echo -- a -- c > rightfile
# diff leftfile rightfile
cmdcomp -d '---' -- cmdcomp --success -- echo -- a -- --- b --- c

# helm show values datadog/datadog --version 3.69.3 | yq -o json | gron > leftfile
# helm show values datadog/datadog --version 3.164.1 | yq -o json | gron > rightfile
# diff -u --color leftfile rightfile
cmdcomp -x 'diff -u --color' -p 'yq -o json' -p 'gron' -- helm show values datadog/datadog --version -- 3.69.3 -- 3.164.1
`)
}

func (u usageBuilder) configFile() string {
	b, _ := yaml.MarshalWithOptions(newConfigExample(), yaml.Indent(2), yaml.IndentSequence(true))
	return `## Config file

### Format

` + u.code("yaml", string(b)) + `

### Usage

` + u.code("shell", `# use "example" preset
# overriding "diff" option
cmdcomp --config CONFIG_PATH --preset example -x 'diff -u'
`) + `

### Examples

` + u.code("yaml", `presets:
  sentry:
    config:
      diff: objdiff -cv
      commonArgs:  ["helm", "template", "sentry/sentry", "--version", "$VERSION"]
`) + `

then

` + u.code("shell", `# helm template sentry/sentry --version 28.0.3 > leftfile
# helm template sentry/sentry --version 29.5.1 > rightfile
# objdiff -cv leftfile rightfile
cmdcomp --config CONFIG --preset sentry --leftEnv 'VERSION=28.0.3' --rightEnv 'VERSION=29.5.1'
`)
}

func (usageBuilder) trailer() string {
	return `## Flags

`
}
