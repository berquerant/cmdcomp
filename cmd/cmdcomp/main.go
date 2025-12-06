package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/berquerant/cmdcomp/pkg/run"
	"github.com/berquerant/cmdcomp/pkg/slicex"
	"github.com/berquerant/cmdcomp/version"
	"github.com/spf13/pflag"
)

const usage = `cmdcomp -- compare the output of two commands with optional preprocessing and customizable diff

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

# Flags

`

func main() {
	fs := pflag.NewFlagSet("main", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fs.PrintDefaults()
	}

	var (
		displayVersion = fs.Bool("version", false, "display version")
		debug          = fs.Bool("debug", false, "enable debug logs")
		workDir        = fs.StringP("work_dir", "w", "", "working directory; keep temporary files")
		shell          = fs.StringP("shell", "s", "bash", "shell command to be executed")
		delimiter      = fs.StringP("delimiter", "d", "--", `arguments delimiter;
change the '--' separating COMMON_ARGS, LEFT_ARGS, and RIGHT_ARGS in this`)
		success = fs.Bool("success", false, `exit successfully even if there are diffs;
in other words, succeed even if the diff command returns exit status 1`)
		interceptor []string
		preprocess  []string
		diff        string
	)
	// workaround: https://github.com/spf13/pflag/issues/370
	fs.StringArrayVarP(&interceptor, "interceptor", "i", nil,
		"process after left command and before right command; invoked like 'interceptor'",
	)
	fs.StringArrayVarP(&preprocess, "preprocess", "p", nil,
		"process before diff; invoked like 'preprocess'; should read input from stdin; should output result to stdout",
	)
	fs.StringVarP(&diff, "diff", "x", "diff",
		"diff command; invoked like 'diff LEFT_FILE RIGHT_FILE'",
	)

	before, after := slicex.Split(os.Args, "--")
	err := fs.Parse(before)
	if errors.Is(err, pflag.ErrHelp) {
		return
	}
	fail(err)
	if *displayVersion {
		version.Write(os.Stdout)
		return
	}

	c := config.NewConfig(os.Stdout, interceptor, preprocess, diff, *shell, *delimiter)
	c.Debug = *debug
	c.WorkDir = *workDir
	c.SetupLogger(os.Stderr)
	slog.Debug("parse args", slog.Any("args", before))
	slog.Debug("init args", slog.Any("args", after))
	fail(c.Init(after))

	cj, _ := json.Marshal(c)
	slog.Debug("config", slog.String("json", string(cj)))
	if err := run.Main(c); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if *success && errors.Is(err, run.ErrDiff) && exitErr.ExitCode() == 1 {
				return
			}
			os.Exit(exitErr.ExitCode())
		}
		fail(err)
	}
}

func fail(err error) {
	if err != nil {
		slog.Error("exit", slog.Any("err", err))
		os.Exit(1)
	}
}
