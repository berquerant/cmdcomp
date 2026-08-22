# cmdcomp

cmdcomp -- compare the output of two commands with optional preprocessing and customizable diff

## Usage

```shell
cmdcomp [flags] -- COMMON_ARGS [-- LEFT_ARGS [-- RIGHT_ARGS]]
```

## Lifecycle & Data Flow

cmdcomp executes subcommands and pipelines in the following order:

```text
[stdin input] ('-' or '@filename')
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
               [cleanup hooks] (guaranteed to run via defer)
```

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

### Basic Comparison
Compare the standard output of two commands:

```shell
# Equivalent shell workflow:
# echo a > leftfile
# echo b > rightfile
# diff leftfile rightfile
cmdcomp -- echo -- a -- b
```

### Custom Diff Tool & Label
Use a customized diff tool (e.g. 'diff -u', 'colordiff') and pass argument labels with '-l' / '--label':

```shell
# Unified diff with labels:
cmdcomp -x 'diff -u' -l -- echo -- a -- b
```

### Common Preprocess Pipeline
Apply filter pipelines (e.g. jq, yq, sed) to both outputs before diffing:

```shell
# Filter both command outputs through sed:
cmdcomp -p 'sed "s|a|c|"' -- echo -- a -- b
```

### Asymmetric Preprocessing (Left & Right)
Apply specific preprocess filters only to the left or right command output in addition to common filters:

```shell
# Replace strings differently on left vs right side:
cmdcomp --left-preprocess 'sed "s|a|c|"' --right-preprocess 'sed "s|a|d|"' -- echo -- a -- a
```

### Startup Hooks & Advanced Preprocessing
Run setup commands sequentially before running the compare commands:

```shell
# Run startup hook before comparing filtered outputs:
cmdcomp -s 'echo "starting setup"' -p 'sed "s|foo|bar|"' -x 'diff -u' -- echo -- foo1 -- foo2
```

### Sequential Execution with Interceptor
Execute left command first, run interceptor hooks (e.g. git checkout, state update), then run right command:

```shell
# Run interceptor hook between left and right commands:
cmdcomp -i 'echo "state changed"' -x 'diff -u' -- echo -- left -- right
```

### Cleanup Hooks & Working Directory
Ensure teardown hooks run on exit and optionally preserve temporary files in a specified directory:

```shell
# Preserve temp files and clean up resources:
cmdcomp --work-dir ./tmp-workdir -c 'echo "cleanup done"' -- echo -- a -- b
```

### Environment Variables (Common, Left, Right)
Pass environment variables to all commands or exclusively to the left or right side:

```shell
# Inject environment variables per side:
cmdcomp -e 'COMMON=1' --left-env 'TARGET=left' --right-env 'TARGET=right' -- bash -c 'echo "$COMMON:$TARGET"'
```

### Replicating Standard Input (--stdin)
Replicate input from standard input ('-') or a file ('@filename') into the stdin of subcommands:

```shell
# Pass shared stdin to subcommands:
# echo -e "alpha\nbeta" | cmdcomp -I - -- grep -- alpha -- beta

# Pass input from file to left command and stdin to right command:
cmdcomp --left-stdin '@data.txt' --right-stdin '-' -- cat
```

### Snapshot Comparison (--snapshot)
Bypass command execution on one or both sides and compare directly against static files or stdin:

```shell
# Compare a pre-generated baseline snapshot file against live command output:
# cmdcomp --left-snapshot '@baseline.txt' -p 'sed "s|x|y|"' -- echo -- live

# Compare two static snapshot files through common preprocess filters without commands:
cmdcomp --left-snapshot '@file1.json' --right-snapshot '@file2.json' -p 'sed "s|10|20|"'
```

### Dry Run Mode (--dry-run)
Generate an executable bash script capturing the exact execution pipeline without running any commands:

```shell
# Output shell script for inspection or reproduction:
# cmdcomp -n -x 'diff -u' -p 'sed "s|v1|common|"' -- echo -- api-v1 -- echo -- api-v2
cmdcomp -x 'diff -u' -p 'sed "s|v1|common|"' -- echo -- api-v1 -- echo -- api-v2
```

### Exit Code & Success Override (--success)
Exit with 0 even when diffs are detected (useful for CI summary steps without failing build):

```shell
# Return exit code 0 on diff:
cmdcomp --success -- echo -- a -- b
```

### Custom Delimiter
Change the argument delimiter from '--' to another token:

```shell
# Use '===' as delimiter when subcommands themselves take '--':
cmdcomp --delimiter '===' -- echo === echo -- a === echo -- b
```

### Nested Comparison (Comparing Diffs)
Compare the diff outputs of two inner cmdcomp executions (e.g. comparing the effect of branch B vs branch C against baseline A):

```shell
# Compare two diff outputs using --delimiter to avoid nested delimiter collision:
cmdcomp --delimiter '===' -- cmdcomp --success -- echo -- base -- === branch-b === branch-c
```

## Config file

### Format

```yaml
presets:
  example:
    show-cmd-log: true
    debug: true
    startup:
      - echo startup
    interceptor:
      - echo interceptor
    preprocess:
      - grep common
    left-preprocess:
      - grep left
    right-preprocess:
      - grep right
    diff: diff
    work-dir: workdir
    shell: bash
    delimiter: --
    label: true
    env:
      - X=1
    left-env:
      - Y=2
    right-env:
      - Y=3
    cleanup:
      - echo cleanup
    common-args:
      - echo
    left-args:
      - common left
    right-args:
      - common right
    success: true
```

### Builtin config

```yaml
presets:
  dyff:
    diff: dyff between --omit-header --set-exit-code
    shell: bash
    delimiter: --
  helm:
    startup:
      - helm repo update
    diff: objdiff -cv
    shell: bash
    delimiter: --
  helm-chart:
    startup:
      - helm repo update
    preprocess:
      - yq -P 'sort_keys(..)'
    diff: diff -u --color
    shell: bash
    delimiter: --
    common-args:
      - helm
      - show
      - chart
      - $CHART
    left-args:
      - --version
      - $LEFT
    right-args:
      - --version
      - $RIGHT
  helm-values:
    startup:
      - helm repo update
    preprocess:
      - yq -P 'sort_keys(..)'
    diff: diff -u --color
    shell: bash
    delimiter: --
    common-args:
      - helm
      - show
      - values
      - $CHART
    left-args:
      - --version
      - $LEFT
    right-args:
      - --version
      - $RIGHT
  json:
    preprocess:
      - jq -S .
    diff: diff -u --color
    shell: bash
    delimiter: --
  k8s:
    diff: objdiff -cv
    shell: bash
    delimiter: --
  k8s-clean:
    preprocess:
      - yq 'del(.metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp, .metadata.generation, .metadata.managedFields, .status)'
    diff: objdiff -cv
    shell: bash
    delimiter: --
  u:
    diff: diff -u
    shell: bash
    delimiter: --
  uc:
    diff: diff -u --color
    shell: bash
    delimiter: --
  yml:
    preprocess:
      - yq -P 'sort_keys(..)'
    diff: diff -u --color
    shell: bash
    delimiter: --
```

### Usage

```shell
# use "example" preset
# overriding "diff" option
cmdcomp --config CONFIG_PATH --preset example -x 'diff -u'

# use builtin "json" preset
cmdcomp --preset json -- ...
```

### Examples

```yaml
presets:
  sentry:
    diff: objdiff -cv
    common-args:  ["helm", "template", "sentry/sentry", "--version", "$VERSION"]
```

then

```shell
# helm template sentry/sentry --version 28.0.3 > leftfile
# helm template sentry/sentry --version 29.5.1 > rightfile
# objdiff -cv leftfile rightfile
cmdcomp --config CONFIG --preset sentry --left-env 'VERSION=28.0.3' --right-env 'VERSION=29.5.1'
```

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

```
  -c, --cleanup stringArray            command(s) guaranteed to execute before cmdcomp exits, even on failure. Can be specified multiple times. In env vars, separate commands with newlines
  -C, --config string                  configuration file path (default search order: UserConfigDir/cmdcomp/config.yml, $HOME/.cmdcomp.yml, .cmdcomp.yml)
      --debug                          enable debug log output
      --delimiter string               delimiter token separating [COMMON_ARGS], [LEFT_ARGS], and [RIGHT_ARGS] (e.g. '===') (default "--")
  -x, --diff string                    diff command invoked as '<diff> LEFT_FILE RIGHT_FILE' (e.g. 'diff -u', 'colordiff', 'dyff', 'objdiff -c') (default "diff")
  -n, --dry-run                        print generated bash script capturing the full execution pipeline without executing commands
  -e, --env stringArray                environment variables passed to all subcommands along with system environment (KEY=VALUE). Can be specified multiple times or comma-separated
  -i, --interceptor stringArray        command(s) executed sequentially after left command and before right command (e.g. git checkout). Can be specified multiple times. In env vars, separate commands with newlines
  -l, --label                          pass '--label LEFT_ARG' and '--label RIGHT_ARG' to the diff command (useful for diff/colordiff)
      --left-env stringArray           environment variables passed only to left command and left preprocess (KEY=VALUE). Can be specified multiple times or comma-separated
      --left-preprocess stringArray    additional filter pipeline command(s) applied only to left output after common preprocess. Multiple flags form a piped chain. In env vars, separate commands with newlines
      --left-snapshot string           use input as left command output without executing the left command ('-' for stdin, '@filename' for file)
      --left-stdin string              pass input to stdin of left command only ('-' for stdin, '@filename' for file)
  -p, --preprocess stringArray         filter pipeline command(s) applied to both left and right outputs before diffing. Reads stdin, writes stdout (e.g. jq, yq, sed). Multiple flags form a piped chain. In env vars, separate commands with newlines
  -P, --preset string                  name of preset configuration to load from config file or built-in presets (e.g. 'json', 'yml', 'helm', 'k8s', 'dyff', 'u', 'uc')
      --process-timeout duration       maximum timeout for each individual subcommand execution (e.g. '10s', '1m')
      --right-env stringArray          environment variables passed only to right command and right preprocess (KEY=VALUE). Can be specified multiple times or comma-separated
      --right-preprocess stringArray   additional filter pipeline command(s) applied only to right output after common preprocess. Multiple flags form a piped chain. In env vars, separate commands with newlines
      --right-snapshot string          use input as right command output without executing the right command ('-' for stdin, '@filename' for file)
      --right-stdin string             pass input to stdin of right command only ('-' for stdin, '@filename' for file)
      --shell string                   shell executable used to run subcommands (default "bash")
      --show-cmd-log                   print stdout and stderr of executed subcommands to log output
  -S, --snapshot string                use input as both left and right command outputs without executing commands ('-' for stdin, '@filename' for file)
  -s, --startup stringArray            command(s) executed sequentially before running commands (e.g. repo updates). Can be specified multiple times. In env vars, separate commands with newlines
  -I, --stdin string                   pass input to stdin of both left and right commands ('-' for stdin, '@filename' for file)
      --success                        exit 0 when diffs are detected (exit status 1 from diff command). Failures (exit code 2) still return 2
      --timeout duration               maximum timeout for entire cmdcomp execution (e.g. '30s', '2m')
      --version                        display version and exit
      --work-dir string                working directory for temporary output files. When specified, temporary files are preserved after execution
```

## Install

``` shell
go install github.com/berquerant/cmdcomp/cmd/cmdcomp@latest
```
