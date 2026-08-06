# cmdcomp

````
cmdcomp -- compare the output of two commands with optional preprocessing and customizable diff

## Usage

```shell
cmdcomp [flags] -- COMMON_ARGS [-- LEFT_ARGS [-- RIGHT_ARGS]]
```

## Examples

```shell
# echo a > leftfile
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
cmdcomp -x 'diff -u --color' -p 'yq -o json' -p 'gron' -- helm show values datadog/datadog --version -- 3.69.3 -- 3.164.1
```

## Config file

### Format

```yaml
presets:
  example:
    config:
      showCmdLog: true
      debug: true
      startup:
        - echo startup
      interceptor:
        - echo interceptor
      preprocess:
        - grep common
      leftPreprocess:
        - grep left
      rightPreprocess:
        - grep right
      diff: diff
      workDir: workdir
      shell: bash
      delimiter: --
      label: true
      env:
        - X=1
      leftEnv:
        - Y=2
      rightEnv:
        - Y=3
      cleanup:
        - echo cleanup
      commonArgs:
        - echo
      leftArgs:
        - common left
      rightArgs:
        - common right
    success: true
```

### Builtin config

```yaml
presets:
  branch:
    config:
      interceptor:
        - git switch ${RIGHT}
      diff: diff -u --color
      shell: bash
      delimiter: --
      cleanup:
        - git switch ${ORIG}
  helm:
    config:
      startup:
        - helm repo update
      diff: objdiff -cv
      shell: bash
      delimiter: --
  json:
    config:
      preprocess:
        - jq -S .
      diff: diff -u --color
      shell: bash
      delimiter: --
  k8s:
    config:
      diff: objdiff -cv
      shell: bash
      delimiter: --
  k8s-clean:
    config:
      preprocess:
        - yq 'del(.metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp, .metadata.generation, .metadata.managedFields, .status)'
      diff: objdiff -cv
      shell: bash
      delimiter: --
  yml:
    config:
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
    config:
      diff: objdiff -cv
      commonArgs:  ["helm", "template", "sentry/sentry", "--version", "$VERSION"]
```

then

```shell
# helm template sentry/sentry --version 28.0.3 > leftfile
# helm template sentry/sentry --version 29.5.1 > rightfile
# objdiff -cv leftfile rightfile
cmdcomp --config CONFIG --preset sentry --leftEnv 'VERSION=28.0.3' --rightEnv 'VERSION=29.5.1'
```

## Flags

  -c, --cleanup stringArray           process before exiting cmdcomp process; invoked like 'cleanup'
      --config string                 config file path; default: UserConfigDir/cmdcomp/config.yml or $HOME/.cmdcomp.yml or .cmdcomp.yml; see https://pkg.go.dev/os#UserConfigDir
      --debug                         enable debug logs
  -d, --delimiter string              arguments delimiter;
                                      change the '--' separating COMMON_ARGS, LEFT_ARGS, and RIGHT_ARGS in this (default "--")
  -x, --diff string                   diff command; invoked like 'diff LEFT_FILE RIGHT_FILE' (default "diff")
      --env stringArray               process environment variables;
                                      Passed to all processes along with os.Environ.
                                      --leftEnv is also passed to left output and left preprocess.
                                      --rightEnv is also passed to right output and right preprocess.
  -i, --interceptor stringArray       process after left command and before right command; invoked like 'interceptor'
  -l, --label                         use '--label' option of diff command
      --leftEnv stringArray           left process environment variables
      --leftPreprocess stringArray    additional left process before diff; invoked like 'leftPreprocess'; should read input from stdin; should output result to stdout
  -p, --preprocess stringArray        process before diff; invoked like 'preprocess'; should read input from stdin; should output result to stdout
      --preset string                 name of preset to be used
      --rightEnv stringArray          right process environment variables
      --rightPreprocess stringArray   additional right process before diff; invoked like 'rightPreprocess'; should read input from stdin; should output result to stdout
  -S, --shell string                  shell command to be executed (default "bash")
      --showCmdLog                    show command logs
  -s, --startup stringArray           process before running commands; invoked like 'startup'
      --success                       exit successfully even if there are diffs;
                                      in other words, succeed even if the diff command returns exit status 1
      --version                       display version
  -w, --workDir string                working directory; keep temporary files
````

## Install

``` shell
go install github.com/berquerant/cmdcomp/cmd/cmdcomp@latest
```
