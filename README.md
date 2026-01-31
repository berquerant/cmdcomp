# cmdcomp

```
❯ cmdcomp --help
cmdcomp -- compare the output of two commands with optional preprocessing and customizable diff

# Usage

cmdcomp [flags] -- COMMON_ARGS [-- LEFT_ARGS [-- RIGHT_ARGS]]

# Examples

// echo a > leftfile
// echo b > rightfile
// diff leftfile rightfile
cmdcomp -- echo -- a -- b

// echo a > leftfile
// echo b > rightfile
// diff -u leftfile rightfile
cmdcomp -x 'diff -u' -- echo -- a -- b

// echo a > leftfile
// echo b > rightfile
// diff -u leftfile rightfile --label echo___a --label echo___b
cmdcomp -x 'diff -u' -l -- echo -- a -- b

// echo a | sed 's|a|c|' > leftfile
// echo b | sed 's|a|c|' > rightfile
// diff leftfile rightfile
cmdcomp -p 'sed "s|a|c|"' -- echo -- a -- b

// helm template datadog/datadog --version 3.68.0 | yq 'select(.kind=="Secret")' > leftfile
// helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug | yq 'select(.kind=="Secret")' > rightfile
// objdiff -c leftfile rightfile
cmdcomp -p "yq 'select(.kind==\"Secret\")'" -x 'objdiff -c' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

// helm template datadog/datadog --version 3.68.0 | yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json > leftfile
// helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug | yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json > rightfile
// npx jsondiffpatch --format=jsonpatch leftfile rightfile
cmdcomp -p "yq 'select(.kind==\"Deployment\" and .metadata.name==\"release-name-datadog-cluster-agent\")' -o json" -x 'npx jsondiffpatch --format=jsonpatch' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

// helm template datadog/datadog --version 3.68.0 | yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json | gron > leftfile
// helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug | yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json | gron > rightfile
// diff -u --color leftfile rightfile
cmdcomp -p "yq 'select(.kind==\"Deployment\" and .metadata.name==\"release-name-datadog-cluster-agent\")' -o json" -p 'gron' -x 'diff -u --color' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

// helm template ./charts/datadog > leftfile
// git checkout datadog-3.69.3
// helm template ./charts/datadog > rightfile
// objdiff -c leftfile rightfile
cmdcomp -i 'git checkout datadog-3.69.3' -x 'objdiff -c' -- helm template ./charts/datadog

// echo echo -- a > leftfile
// echo echo -- b > rightfile
// diff leftfile rightfile
cmdcomp -d '---' -- echo --- echo -- a --- echo -- b

// cmdcomp --success -- echo -- a -- b > leftfile
// cmdcomp --success -- echo -- a -- c > rightfile
// diff leftfile rightfile
cmdcomp -d '---' -- cmdcomp --success -- echo -- a -- --- b --- c

// helm show values datadog/datadog --version 3.69.3 | yq -o json | gron > leftfile
// helm show values datadog/datadog --version 3.164.1 | yq -o json | gron > rightfile
// diff -u --color leftfile rightfile
cmdcomp -x 'diff -u --color' -p 'yq -o json' -p 'gron' -- helm show values datadog/datadog --version -- 3.69.3 -- 3.164.1

# Flags

      --debug                     enable debug logs
  -d, --delimiter string          arguments delimiter;
                                  change the '--' separating COMMON_ARGS, LEFT_ARGS, and RIGHT_ARGS in this (default "--")
  -x, --diff string               diff command; invoked like 'diff LEFT_FILE RIGHT_FILE' (default "diff")
  -i, --interceptor stringArray   process after left command and before right command; invoked like 'interceptor'
  -l, --label                     use '--label' option of diff command
  -p, --preprocess stringArray    process before diff; invoked like 'preprocess'; should read input from stdin; should output result to stdout
  -s, --shell string              shell command to be executed (default "bash")
      --showCmdLog                show command logs
      --success                   exit successfully even if there are diffs;
                                  in other words, succeed even if the diff command returns exit status 1
      --version                   display version
  -w, --workDir string            working directory; keep temporary files
```

## Install

``` shell
go install github.com/berquerant/cmdcomp/cmd/cmdcomp@latest
```
