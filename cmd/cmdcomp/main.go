package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/berquerant/cmdcomp/pkg/cli"
	"github.com/berquerant/cmdcomp/pkg/mcp"
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

	if c.MCP {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		s := mcp.NewServer(c)
		if err := s.Serve(ctx); err != nil && !errors.Is(err, context.Canceled) {
			fail(err)
		}
		return
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
