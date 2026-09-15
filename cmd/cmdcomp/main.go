package main

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"

	"github.com/berquerant/cmdcomp/pkg/cli"
	"github.com/berquerant/cmdcomp/pkg/run"
)

func main() {
	c, err := cli.ParseConfig(os.Args, os.Stdout, os.Stderr)
	if errors.Is(err, cli.ErrExit) {
		return
	}
	if err != nil {
		fail(err)
	}

	if err := run.Main(c); err != nil {
		if errors.Is(err, run.ErrDiff) {
			if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
				if c.Success && exitErr.ExitCode() == 1 {
					return
				}
				os.Exit(exitErr.ExitCode())
			}
		}
		fail(err)
	}
}

func fail(err error) {
	if err != nil {
		slog.Error("exit", slog.Any("err", err))
		os.Exit(2)
	}
}
