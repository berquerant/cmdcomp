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
	return `### Basic Comparison
Compare the standard output of two commands:

` + u.code("shell", `# Equivalent shell workflow:
# echo a > leftfile
# echo b > rightfile
# diff leftfile rightfile
cmdcomp -- echo -- a -- b`) + `

### Custom Diff Tool & Label
Use a customized diff tool (e.g. 'diff -u', 'colordiff') and pass argument labels with '-l' / '--label':

` + u.code("shell", `# Unified diff with labels:
cmdcomp -x 'diff -u' -l -- echo -- a -- b`) + `

### Common Preprocess Pipeline
Apply filter pipelines (e.g. jq, yq, sed) to both outputs before diffing:

` + u.code("shell", `# Filter both command outputs through sed:
cmdcomp -p 'sed "s|a|c|"' -- echo -- a -- b`) + `

### Asymmetric Preprocessing (Left & Right)
Apply specific preprocess filters only to the left or right command output in addition to common filters:

` + u.code("shell", `# Replace strings differently on left vs right side:
cmdcomp --left-preprocess 'sed "s|a|c|"' --right-preprocess 'sed "s|a|d|"' -- echo -- a -- a`) + `

### Startup Hooks & Advanced Preprocessing
Run setup commands (e.g. 'helm repo update') sequentially before running the compare commands:

` + u.code("shell", `# Update repo before comparing secret manifests:
cmdcomp --startup 'helm repo update' -p "yq 'select(.kind==\"Secret\")'" -x 'objdiff -c' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug`) + `

### Sequential Execution with Interceptor
Execute left command first, run interceptor hooks (e.g. git checkout, database migration), then run right command:

` + u.code("shell", `# Compare local helm chart across git revisions:
cmdcomp -i 'git checkout datadog-3.69.3' -x 'objdiff -c' -- helm template ./charts/datadog`) + `

### Cleanup Hooks & Working Directory
Ensure teardown hooks run on exit and optionally preserve temporary files in a specified directory:

` + u.code("shell", `# Preserve temp files and clean up resources:
cmdcomp -w ./tmp-workdir -c 'echo "cleanup done"' -- echo -- a -- b`) + `

### Environment Variables (Common, Left, Right)
Pass environment variables to all commands or exclusively to the left or right side:

` + u.code("shell", `# Inject environment variables per side:
cmdcomp -e 'COMMON=1' --left-env 'TARGET=left' --right-env 'TARGET=right' -- bash -c 'echo "$COMMON:$TARGET"'`) + `

### Replicating Standard Input (--stdin)
Replicate input from standard input ('-') or a file ('@filename') into the stdin of subcommands:

` + u.code("shell", `# Pass shared stdin to grep commands:
echo -e "alpha\nbeta" | cmdcomp --stdin - -- grep -- alpha -- beta

# Pass input from file to left command only:
cmdcomp --left-stdin '@data.txt' --right-stdin '-' -- grep -- pattern`) + `

### Snapshot Comparison (--snapshot)
Bypass command execution on one or both sides and compare directly against static files or stdin:

` + u.code("shell", `# Compare a pre-generated baseline snapshot file against live command output:
cmdcomp --left-snapshot '@baseline.yaml' -p 'yq ...' -- helm template ./charts/app

# Compare two static snapshot files through common preprocess filters without commands:
cmdcomp --left-snapshot '@file1.json' --right-snapshot '@file2.json' -p 'jq .key'`) + `

### Dry Run Mode (--dry-run)
Generate an executable bash script capturing the exact execution pipeline without running any commands:

` + u.code("shell", `# Output shell script for inspection or reproduction:
cmdcomp --dry-run -x 'diff -u' -p 'jq .' -- curl -s https://api/v1 -- curl -s https://api/v2`) + `

### Exit Code & Success Override (--success)
Exit with 0 even when diffs are detected (useful for CI summary steps without failing build):

` + u.code("shell", `# Return exit code 0 on diff:
cmdcomp --success -- echo -- a -- b`) + `

### Custom Delimiter
Change the argument delimiter from '--' to another token:

` + u.code("shell", `# Use '---' as delimiter when subcommands themselves take '--':
cmdcomp -d '---' -- echo --- echo -- a --- echo -- b`)
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
3. **snapshot setup**: If '--snapshot', '--left-snapshot', or '--right-snapshot' is specified, command execution is skipped for that side and the given input is used directly as the command output. Individual '--left-snapshot' / '--right-snapshot' takes precedence over '--snapshot'. Both '--stdin' and '--snapshot' cannot be used together for the same side.
4. **left command & right command**:
   - Without interceptor: Left and right commands run concurrently.
   - With interceptor: Left command runs first -> interceptor hooks run sequentially (e.g. git checkout <branch>) -> Right command runs.
   - Sides with a snapshot configured are skipped entirely.
5. **preprocess pipeline**: Standard output of left and right commands (or snapshot inputs) are piped through preprocess filters:
   - Left output: piped through preprocess -> left-preprocess
   - Right output: piped through preprocess -> right-preprocess
6. **diff**: Output files from the preprocess pipelines are passed to the diff tool ('<diff> LEFT_FILE RIGHT_FILE').
7. **cleanup**: Teardown hooks are guaranteed to run when cmdcomp exits, even on failure or error.

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
