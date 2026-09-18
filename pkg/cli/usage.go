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
	UsageCode               string
	MCPConfigCode           string
	MCPToolCallExamplesCode string
	LifecycleCode           string
	ExamplesCode            string
	ConfigFormatCode        string
	BuiltinConfigCode       string
	ConfigUsageCode         string
	ConfigExamplesCode      string
	SentryShellCode         string
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
	return u.renderExamples()
}

func (u UsageBuilder) configFormatCode() string {
	return u.code("yaml", u.marshalYaml(newConfigExample()))
}

func (u UsageBuilder) builtinConfigCode() string {
	return u.code("yaml", u.marshalYaml(builtinConfigSet()))
}

func (u UsageBuilder) configUsageCode() string {
	return u.code("shell", `# automatically loads config from default search paths (~/.cmdcomp.yml, .cmdcomp.yml, etc.)
# uses 'default' section if present in the config file
cmdcomp -- ...

# use specific config file
cmdcomp --config CONFIG_PATH -- ...

# ignore 'default' section in config file
cmdcomp --no-default -- ...

# use builtin "json" preset
cmdcomp --preset json -- ...

# compose multiple presets with config file and CLI flags
cmdcomp --config CONFIG_PATH --preset example,u -x 'diff -u' -- ...`)
}

func (u UsageBuilder) configExamplesCode() string {
	return u.code("yaml", `default:
  diff: diff -u --color
  shell: bash

presets:
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

func (u UsageBuilder) mcpConfigCode() string {
	return u.code("json", `{
  "mcpServers": {
    "cmdcomp": {
      "command": "cmdcomp",
      "args": ["--mcp"]
    }
  }
}`)
}

func (u UsageBuilder) mcpToolCallExamplesCode() string {
	return u.code("json", `// 1. Basic command diff
{
  "name": "cmdcomp_diff",
  "arguments": {
    "common_args": ["echo"],
    "left_args": ["hello left"],
    "right_args": ["hello right"]
  }
}

// 2. Diff with preset and custom diff tool
{
  "name": "cmdcomp_diff",
  "arguments": {
    "presets": ["json"],
    "diff": "diff -u",
    "common_args": ["cat"],
    "left_args": ["left.json"],
    "right_args": ["right.json"]
  }
}

// 3. Diff using literal stdin content
{
  "name": "cmdcomp_diff",
  "arguments": {
    "common_args": ["cat"],
    "left_preprocess": ["sed 's/foo/bar/'"],
    "right_preprocess": ["sed 's/foo/baz/'"],
    "stdin_content": "foo 123\n"
  }
}

// 4. Generate dry-run script
{
  "name": "cmdcomp_dryrun",
  "arguments": {
    "common_args": ["echo"],
    "left_args": ["a"],
    "right_args": ["b"]
  }
}`)
}

var rawUsageTemplate = `cmdcomp -- compare the output of two commands with optional preprocessing and customizable diff

## Usage

{{.UsageCode}}

## MCP Server

cmdcomp can run as a Model Context Protocol (MCP) server over stdio using the ` + "`--mcp`" + ` flag. This allows LLMs and AI agents to invoke cmdcomp as a tool.

### MCP Server Registration Example

Add the following to your MCP client configuration:

{{.MCPConfigCode}}

### Tool Call Arguments Example (JSON)

When an AI agent invokes tools on the cmdcomp MCP server, use JSON arguments as follows:

{{.MCPToolCallExamplesCode}}

### Available Tools

- ` + "`cmdcomp_diff`" + `: Compare stdout of two commands or snapshots with optional preprocessing filters and custom diff tools.
- ` + "`cmdcomp_dryrun`" + `: Generate an executable bash script capturing the full execution pipeline.
- ` + "`cmdcomp_list_presets`" + `: List available presets defined in configuration files or built-in presets.

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

### Search Paths
cmdcomp automatically searches for configuration files in the following order:
1. ` + "`$XDG_CONFIG_HOME/cmdcomp/config.yml`" + ` (or ` + "`os.UserConfigDir()/cmdcomp/config.yml`" + `)
2. ` + "`$HOME/.cmdcomp.yml`" + `
3. ` + "`.cmdcomp.yml`" + ` (in current working directory)
Explicit ` + "`--config / -C`" + ` flag or ` + "`CMDCOMP_CONFIG`" + ` environment variable overrides the search order.

### Format
A configuration file is written in YAML and consists of two top-level sections:
- ` + "`default`" + `: Optional base configuration applied automatically to all executions (unless ` + "`--no-default`" + ` is specified).
- ` + "`presets`" + `: A dictionary of named configurations that can be selected or composed via ` + "`--preset / -P`" + ` (e.g. ` + "`--preset preset1,preset2`" + `).

Each preset (and ` + "`default`" + `) accepts identical fields directly corresponding to CLI flags and options:

| YAML Key | Type | Corresponding Flag | Description |
|:---|:---|:---|:---|
| ` + "`diff`" + ` | string | ` + "`-x, --diff`" + ` | Diff command to compare outputs (e.g. ` + "`diff -u`" + `, ` + "`colordiff`" + `, ` + "`dyff`" + `) |
| ` + "`shell`" + ` | string | ` + "`--shell`" + ` | Shell executable used to run subcommands (default: ` + "`bash`" + `) |
| ` + "`delimiter`" + ` | string | ` + "`--delimiter`" + ` | Delimiter separating common, left, and right args (default: ` + "`--`" + `) |
| ` + "`label`" + ` | boolean | ` + "`-l, --label`" + ` | Pass ` + "`--label`" + ` arguments to diff command |
| ` + "`success`" + ` | boolean | ` + "`--success`" + ` | Exit 0 even when diffs are detected (diff status 1) |
| ` + "`dry-run`" + ` | boolean | ` + "`-n, --dry-run`" + ` | Generate shell script capturing execution pipeline |
| ` + "`debug`" + ` | boolean | ` + "`--debug`" + ` | Enable debug log output |
| ` + "`show-cmd-log`" + ` | boolean | ` + "`--show-cmd-log`" + ` | Print subcommands stdout and stderr to logs |
| ` + "`work-dir`" + ` | string | ` + "`--work-dir`" + ` | Directory for temporary files (preserves files if set) |
| ` + "`timeout`" + ` | duration | ` + "`--timeout`" + ` | Maximum timeout for entire execution (e.g. ` + "`30s`" + `, ` + "`2m`" + `) |
| ` + "`process-timeout`" + ` | duration | ` + "`--process-timeout`" + ` | Timeout for each subcommand (e.g. ` + "`10s`" + `, ` + "`1m`" + `) |
| ` + "`startup`" + ` | string list | ` + "`-s, --startup`" + ` | Setup commands run sequentially before execution |
| ` + "`interceptor`" + ` | string list | ` + "`-i, --interceptor`" + ` | Commands run between left and right commands |
| ` + "`preprocess`" + ` | string list | ` + "`-p, --preprocess`" + ` | Pipe filter commands applied to both outputs (e.g. ` + "`jq`" + `, ` + "`sed`" + `) |
| ` + "`left-preprocess`" + ` | string list | ` + "`-L, --left-preprocess`" + ` | Filter commands applied only to left output |
| ` + "`right-preprocess`" + ` | string list | ` + "`-R, --right-preprocess`" + ` | Filter commands applied only to right output |
| ` + "`cleanup`" + ` | string list | ` + "`-c, --cleanup`" + ` | Teardown commands guaranteed to execute on exit |
| ` + "`env`" + ` | string list | ` + "`-e, --env`" + ` | Environment variables for all commands (` + "`KEY=VAL`" + `) |
| ` + "`left-env`" + ` | string list | ` + "`-E, --left-env`" + ` | Environment variables for left command only |
| ` + "`right-env`" + ` | string list | ` + "`-F, --right-env`" + ` | Environment variables for right command only |
| ` + "`stdin`" + ` | string | ` + "`-I, --stdin`" + ` | Input piped to stdin (` + "`-`" + ` for stdin, ` + "`@file`" + ` for file) |
| ` + "`left-stdin`" + ` | string | ` + "`-J, --left-stdin`" + ` | Input piped to left command stdin only |
| ` + "`right-stdin`" + ` | string | ` + "`-K, --right-stdin`" + ` | Input piped to right command stdin only |
| ` + "`snapshot`" + ` | string | ` + "`-S, --snapshot`" + ` | Static snapshot replacing both command outputs |
| ` + "`left-snapshot`" + ` | string | ` + "`-T, --left-snapshot`" + ` | Static snapshot replacing left command output |
| ` + "`right-snapshot`" + ` | string | ` + "`-U, --right-snapshot`" + ` | Static snapshot replacing right command output |
| ` + "`common-args`" + ` | string list | (CLI trailing args) | Preset common args prepended to both commands |
| ` + "`left-args`" + ` | string list | (CLI trailing args) | Preset left-specific command args |
| ` + "`right-args`" + ` | string list | (CLI trailing args) | Preset right-specific command args |

Full example structure:

{{.ConfigFormatCode}}

### Builtin presets

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
		UsageCode:               u.usageCode(),
		MCPConfigCode:           u.mcpConfigCode(),
		MCPToolCallExamplesCode: u.mcpToolCallExamplesCode(),
		LifecycleCode:           u.lifecycleCode(),
		ExamplesCode:            u.examplesCode(),
		ConfigFormatCode:        u.configFormatCode(),
		BuiltinConfigCode:       u.builtinConfigCode(),
		ConfigUsageCode:         u.configUsageCode(),
		ConfigExamplesCode:      u.configExamplesCode(),
		SentryShellCode:         u.sentryShellCode(),
	}

	var buf bytes.Buffer
	if err := parsedUsageTemplate.Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}
