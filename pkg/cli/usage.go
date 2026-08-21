package cli

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"
)

type UsageBuilder struct{}

type usageTemplateData struct {
	UsageCode          string
	LifecycleCode      string
	ExamplesCode       string
	ConfigFormatCode   string
	BuiltinConfigCode  string
	ConfigUsageCode    string
	ConfigExamplesCode string
	SentryShellCode    string
}

func (UsageBuilder) code(lang, s string) string {
	return fmt.Sprintf("```%s\n%s```", lang, strings.TrimRight(s, "\n")+"\n")
}

func (UsageBuilder) marshalYaml(v any) string {
	b, _ := yaml.MarshalWithOptions(v, yaml.Indent(2), yaml.IndentSequence(true))
	return string(b)
}

func (u UsageBuilder) usageCode() string {
	return u.code("shell", `cmdcomp [flags] -- COMMON_ARGS [-- LEFT_ARGS [-- RIGHT_ARGS]]`)
}

func (u UsageBuilder) lifecycleCode() string {
	return u.code("text", `[stdin input] ('-' or '@filename')
      │ (replicated to both commands if --stdin is specified)
      ├───────────────────────────────┐
      ▼                               ▼
[left command]                 [right command]
      │                               ▲
      │ ──► [interceptor hooks] ──────┘ (if interceptor specified: run sequentially)
      │                               │
      ▼ (stdout)                      ▼ (stdout)
[preprocess:left pipeline]     [preprocess:right pipeline]
(common + left-preprocess)     (common + right-preprocess)
      │                               │
      ▼                               ▼
 [left tempfile]               [right tempfile]
      │                               │
      └───────────────┬───────────────┘
                      ▼
               [diff command] (e.g. diff left_file right_file)
                      │
                      ▼
               [cleanup hooks] (guaranteed to run via defer)`)
}

func (u UsageBuilder) examplesCode() string {
	return u.code("shell", `# echo a > leftfile
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

# helm repo update
# helm template datadog/datadog --version 3.68.0 | yq 'select(.kind=="Secret")' > leftfile
# helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug | yq 'select(.kind=="Secret")' > rightfile
# objdiff -c leftfile rightfile
cmdcomp --startup 'helm repo update' -p "yq 'select(.kind==\"Secret\")'" -x 'objdiff -c' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

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
cmdcomp -x 'diff -u --color' -p 'yq -o json' -p 'gron' -- helm show values datadog/datadog --version -- 3.69.3 -- 3.164.1`)
}

func (u UsageBuilder) configFormatCode() string {
	return u.code("yaml", u.marshalYaml(newConfigExample()))
}

func (u UsageBuilder) builtinConfigCode() string {
	return u.code("yaml", u.marshalYaml(builtinConfigSet()))
}

func (u UsageBuilder) configUsageCode() string {
	return u.code("shell", `# use "example" preset
# overriding "diff" option
cmdcomp --config CONFIG_PATH --preset example -x 'diff -u'

# use builtin "json" preset
cmdcomp --preset json -- ...`)
}

func (u UsageBuilder) configExamplesCode() string {
	return u.code("yaml", `presets:
  sentry:
    diff: objdiff -cv
    common-args:  ["helm", "template", "sentry/sentry", "--version", "$VERSION"]`)
}

func (u UsageBuilder) sentryShellCode() string {
	return u.code("shell", `# helm template sentry/sentry --version 28.0.3 > leftfile
# helm template sentry/sentry --version 29.5.1 > rightfile
# objdiff -cv leftfile rightfile
cmdcomp --config CONFIG --preset sentry --left-env 'VERSION=28.0.3' --right-env 'VERSION=29.5.1'`)
}

var rawUsageTemplate = `cmdcomp -- compare the output of two commands with optional preprocessing and customizable diff

## Usage

{{.UsageCode}}

## Lifecycle & Data Flow

cmdcomp executes subcommands and pipelines in the following order:

{{.LifecycleCode}}

1. **startup**: Setup commands run sequentially before executing left/right commands (e.g. helm repo update).
2. **stdin setup**: If '--stdin', '--left-stdin', or '--right-stdin' is specified, input from stdin ('-') or files ('@filename') is prepared for left and right commands (individual '--left-stdin' / '--right-stdin' takes precedence over '--stdin').
3. **left command & right command**:
   - Without interceptor: Left and right commands run concurrently.
   - With interceptor: Left command runs first -> interceptor hooks run sequentially (e.g. git checkout <branch>) -> Right command runs.
4. **preprocess pipeline**: Standard output of left and right commands are piped through preprocess filters:
   - Left output: piped through preprocess -> left-preprocess
   - Right output: piped through preprocess -> right-preprocess
5. **diff**: Output files from the preprocess pipelines are passed to the diff tool ('<diff> LEFT_FILE RIGHT_FILE').
6. **cleanup**: Teardown hooks are guaranteed to run when cmdcomp exits, even on failure or error.

## Examples

{{.ExamplesCode}}

## Config file

### Format

{{.ConfigFormatCode}}

### Builtin config

{{.BuiltinConfigCode}}

### Usage

{{.ConfigUsageCode}}

### Examples

{{.ConfigExamplesCode}}

then

{{.SentryShellCode}}

## Exit Codes

- 0: No diff detected, dryrun, version/help displayed, or diff detected with --success.
- 1: Diff detected (exit code of diff command).
- 2: Process failure (command error, hook error, pipeline error, timeout, config/flag error). Always returns 2 even if --success is specified.

## Environment Variables

All flags can be specified via environment variables using the 'CMDCOMP_' prefix (e.g. CMDCOMP_DIFF, CMDCOMP_PRESET, CMDCOMP_SHOW_CMD_LOG).
Precedence: Default/Preset < Environment Variables < Command-line Flags

- Command lists (CMDCOMP_STARTUP, CMDCOMP_PREPROCESS, etc.): Separate multiple commands with newline.
- Environment variable pairs (CMDCOMP_ENV, CMDCOMP_LEFT_ENV, CMDCOMP_RIGHT_ENV): Separate entries with comma (,).

## Flags

`

var parsedUsageTemplate = template.Must(template.New("usage").Parse(rawUsageTemplate))

func (u UsageBuilder) Build() string {
	data := usageTemplateData{
		UsageCode:          u.usageCode(),
		LifecycleCode:      u.lifecycleCode(),
		ExamplesCode:       u.examplesCode(),
		ConfigFormatCode:   u.configFormatCode(),
		BuiltinConfigCode:  u.builtinConfigCode(),
		ConfigUsageCode:    u.configUsageCode(),
		ConfigExamplesCode: u.configExamplesCode(),
		SentryShellCode:    u.sentryShellCode(),
	}

	var buf bytes.Buffer
	if err := parsedUsageTemplate.Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}
