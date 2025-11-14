package run

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/berquerant/cmdcomp/pkg/execx"
)

func Main(c *config.Config) error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGPIPE,
	)
	defer stop()
	return run(ctx, c)
}

func run(ctx context.Context, c *config.Config) error {
	defer c.Close()

	runCmd := func(arg ...string) (string, error) {
		return execx.NewCmd(c.TempDir, arg...).Run(ctx)
	}
	runShellCmd := func(arg ...string) (string, error) {
		return runCmd(append([]string{c.Shell, "-c"}, arg...)...)
	}

	slog.Debug("start run left", slog.Any("args", c.GetLeftArgs()))
	leftOut, err := runCmd(c.GetLeftArgs()...)
	if err != nil {
		return fmt.Errorf("%w: run left", err)
	}
	slog.Debug("end run left", slog.String("out", leftOut))

	for i, p := range c.Interceptor {
		logger := slog.With(slog.Int("count", i), slog.String("interceptor", p))
		logger.Debug("start run interceptor")
		cmd := exec.CommandContext(ctx, c.Shell, "-c", p)
		cmd.Stdout = os.Stderr // interceptor stdout cannot be mixed with diff stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%w: run interceptor[%d]", err, i)
		}
		slog.Debug("end run interceptor")
	}

	slog.Debug("start run right", slog.Any("args", c.GetRightArgs()))
	rightOut, err := runCmd(c.GetRightArgs()...)
	if err != nil {
		return fmt.Errorf("%w: run right", err)
	}
	slog.Debug("end run right", slog.String("out", rightOut))

	for i, p := range c.Preprocess {
		logger := slog.With(slog.Int("count", i), slog.String("preprocess", p))
		logger.Debug("start run preprocess for left")
		leftOut, err = runShellCmd(p + " " + leftOut)
		if err != nil {
			return fmt.Errorf("%w: run preprocess[%d] for left", err, i)
		}
		logger.Debug("end run preprocess for left", slog.String("out", leftOut))

		logger.Debug("start run preprocess for right")
		rightOut, err = runShellCmd(p + " " + rightOut)
		if err != nil {
			return fmt.Errorf("%w: run preprocess[%d] for right", err, i)
		}
		logger.Debug("end run preprocess for right", slog.String("out", rightOut))
	}

	slog.Debug("start run diff", slog.String("diff", c.Diff))
	cmd := exec.CommandContext(ctx, c.Shell, "-c", c.Diff+" "+leftOut+" "+rightOut)
	cmd.Stdout = c.Writer
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	err = cmd.Run()
	slog.Debug("end run diff", slog.Any("err", err))
	return err
}
