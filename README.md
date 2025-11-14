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
// sed 's|a|c|' leftfile > leftfile2
// sed 's|a|c|' rightfile > rightfile2
// diff leftfile2 rightfile2
cmdcomp -p 'sed "s|a|c|"' -- echo -- a -- b

// helm template datadog/datadog --version 3.68.0 > leftfile1
// helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug > rightfile1
// yq 'select(.kind=="Secret")' leftfile1 > leftfile2
// yq 'select(.kind=="Secret")' rightfile1 > rightfile2
// objdiff -c leftfile2 rightfile2
cmdcomp -p "yq 'select(.kind==\"Secret\")'" -x 'objdiff -c' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

// helm template datadog/datadog --version 3.68.0 > leftfile1
// helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug > rightfile1
// yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json leftfile1 > leftfile2
// yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json rightfile1 > rightfile2
// npx jsondiffpatch --format=jsonpatch leftfile2 rightfile2
cmdcomp -p "yq 'select(.kind==\"Deployment\" and .metadata.name==\"release-name-datadog-cluster-agent\")' -o json" -x 'npx jsondiffpatch --format=jsonpatch' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

// helm template datadog/datadog --version 3.68.0 > leftfile1
// helm template datadog/datadog --version 3.69.3 --set datadog.logLevel=debug > rightfile1
// yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json leftfile1 > leftfile2
// yq 'select(.kind=="Deployment" and .metadata.name=="release-name-datadog-cluster-agent")' -o json rightfile1 > rightfile2
// gron leftfile2 > leftfile3
// gron rightfile2 > rightfile3
// diff -u --color leftfile3 rightfile3
cmdcomp -p "yq 'select(.kind==\"Deployment\" and .metadata.name==\"release-name-datadog-cluster-agent\")' -o json" -p 'gron' -x 'diff -u --color' -- helm template datadog/datadog -- --version 3.68.0 -- --version 3.69.3 --set datadog.logLevel=debug

// helm template ./charts/datadog > leftfile1
// git checkout datadog-3.69.3
// helm template ./charts/datadog > rightfile1
// objdiff -c leftfile1 rightfile1
cmdcomp -i 'git checkout datadog-3.69.3' -x 'objdiff -c' -- helm template ./charts/datadog

# Flags

      --debug                     enable debug logs
  -x, --diff string               diff command; invoked like 'diff LEFT_FILE RIGHT_FILE' (default "diff")
  -i, --interceptor stringArray   process after left command and before right command; invoked like 'interceptor'
  -p, --preprocess stringArray    process before diff; invoked like 'preprocess FILE'; should output result to stdout
  -s, --shell string              shell command to be executed (default "bash")
  -w, --work_dir string           working directory; keep temporary files
```

## Install

``` shell
go install github.com/berquerant/cmdcomp/cmd/cmdcomp@latest
```
