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

// echo a > leftfile
// echo b > rightfile
// sed 's|a|c|' leftfile > leftfile2
// sed 's|a|c|' rightfile > rightfile2
// diff leftfile2 rightfile2
cmdcomp -p 'sed s|a|c|' -- echo -- a -- b

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

# Flags

`

func main() {
	fs := pflag.NewFlagSet("main", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fs.PrintDefaults()
	}

	var (
		debug      = fs.Bool("debug", false, "enable debug logs")
		workDir    = fs.StringP("work_dir", "w", "", "working directory; keep temporary files")
		shell      = fs.StringP("shell", "s", "bash", "shell command to be executed")
		preprocess []string
		diff       string
	)
	// workaround: https://github.com/spf13/pflag/issues/370
	fs.StringArrayVarP(&preprocess, "preprocess", "p", nil,
		"process before diff; invoked like 'preprocess FILE'; should output result to stdout",
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

	c := config.NewConfig(os.Stdout, preprocess, diff, *shell)
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
