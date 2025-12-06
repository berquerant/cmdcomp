package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/berquerant/cmdcomp/pkg/execx"
	"golang.org/x/sync/errgroup"
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

// An error from diff command.
var ErrDiff = errors.New("Diff")

func run(ctx context.Context, c *config.Config) error {
	defer c.Close()

	var (
		runCmd = func(arg ...string) (string, error) {
			return execx.NewCmd(c.TempDir, arg...).Run(ctx)
		}
		runGenCmd = func(target string, arg ...string) (string, error) {
			slog.Debug(fmt.Sprintf("start run %s", target), slog.Any("args", arg))
			out, err := runCmd(arg...)
			if err != nil {
				return "", fmt.Errorf("%w: run %s", err, target)
			}
			slog.Debug(fmt.Sprintf("end run %s", target), slog.String("out", out))
			return out, nil
		}

		leftOut, rightOut string
	)

	if len(c.Interceptor) == 0 {
		eg, _ := errgroup.WithContext(ctx)
		eg.Go(func() error {
			out, err := runGenCmd("left", c.GetLeftArgs()...)
			if err != nil {
				return err
			}
			leftOut = out
			return nil
		})
		eg.Go(func() error {
			out, err := runGenCmd("right", c.GetRightArgs()...)
			if err != nil {
				return err
			}
			rightOut = out
			return nil
		})
		if err := eg.Wait(); err != nil {
			return err
		}
	} else {
		var err error
		if leftOut, err = runGenCmd("left", c.GetLeftArgs()...); err != nil {
			return err
		}
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
		if rightOut, err = runGenCmd("right", c.GetRightArgs()...); err != nil {
			return err
		}
	}

	if len(c.Preprocess) > 0 {
		var (
			newShellCmd = func(arg ...string) *execx.Cmd {
				return execx.NewCmd(c.TempDir, append([]string{c.Shell, "-c"}, arg...)...)
			}
			runPreprocess = func(target, inFile string, cmd ...*execx.Cmd) (string, error) {
				slog.Debug(fmt.Sprintf("start %s preprocess", target), slog.String("in", inFile))
				stdin, err := os.Open(inFile)
				if err != nil {
					return "", fmt.Errorf("%w: run %s preprocess", err, target)
				}
				defer stdin.Close()
				p := execx.NewPipedCmd(ctx, c.TempDir, stdin, cmd...)
				if err := p.Run(ctx); err != nil {
					return "", fmt.Errorf("%w: run %s preprocess", err, target)
				}
				slog.Debug(fmt.Sprintf("end %s preprocess", target), slog.String("out", p.Path()))
				return p.Path(), nil
			}
			leftPreprocess  = make([]*execx.Cmd, len(c.Preprocess))
			rightPreprocess = make([]*execx.Cmd, len(c.Preprocess))
		)
		for i, p := range c.Preprocess {
			logger := slog.With(slog.Int("count", i), slog.String("preprocess", p))
			logger.Debug("preprocess")
			leftPreprocess[i] = newShellCmd(p)
			rightPreprocess[i] = newShellCmd(p)
		}

		eg, _ := errgroup.WithContext(ctx)
		eg.Go(func() error {
			out, err := runPreprocess("left", leftOut, leftPreprocess...)
			if err != nil {
				return err
			}
			leftOut = out
			return nil
		})
		eg.Go(func() error {
			out, err := runPreprocess("right", rightOut, rightPreprocess...)
			if err != nil {
				return err
			}
			rightOut = out
			return nil
		})
		if err := eg.Wait(); err != nil {
			return err
		}
	}

	slog.Debug("start run diff", slog.String("diff", c.Diff))
	cmd := exec.CommandContext(ctx, c.Shell, "-c", c.Diff+" "+leftOut+" "+rightOut)
	cmd.Stdout = c.Writer
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err != nil {
		err = errors.Join(ErrDiff, err)
	}
	slog.Debug("end run diff", slog.Any("err", err))
	return err
}
