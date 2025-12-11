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
	runner := &runner{
		Config: c,
	}
	return runner.run(ctx)
}

// An error from diff command.
var ErrDiff = errors.New("Diff")

type runner struct {
	*config.Config
}

func (r runner) runCmd(ctx context.Context, arg ...string) (string, error) {
	return execx.NewCmd(r.TempDir, arg...).Run(ctx)
}

func (r runner) runGenCmd(ctx context.Context, target string, arg ...string) (string, error) {
	slog.Debug(fmt.Sprintf("start run %s", target), slog.Any("args", arg))
	out, err := r.runCmd(ctx, arg...)
	if err != nil {
		return "", fmt.Errorf("%w: run %s", err, target)
	}
	slog.Debug(fmt.Sprintf("end run %s", target), slog.String("out", out))
	return out, nil
}

func (r runner) newShellCmd(arg ...string) *execx.Cmd {
	return execx.NewCmd(r.TempDir, append([]string{r.Shell, "-c"}, arg...)...)
}

func (r runner) runInterceptors(ctx context.Context) error {
	for i, p := range r.Interceptor {
		logger := slog.With(slog.Int("count", i), slog.String("interceptor", p))
		logger.Debug("start run interceptor")
		cmd := exec.CommandContext(ctx, r.Shell, "-c", p)
		cmd.Stdout = os.Stderr // interceptor stdout cannot be mixed with diff stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%w: run interceptor[%d]", err, i)
		}
		slog.Debug("end run interceptor")
	}
	return nil
}

func (r runner) runLeftGenCmd(ctx context.Context) (string, error) {
	return r.runGenCmd(ctx, "left", r.GetLeftArgs()...)
}

func (r runner) runRightGenCmd(ctx context.Context) (string, error) {
	return r.runGenCmd(ctx, "right", r.GetRightArgs()...)
}

type cmdResult struct {
	leftOut, rightOut string
}

func (r runner) runGenCmdsConcurrently(ctx context.Context) (*cmdResult, error) {
	var (
		leftOut, rightOut string
		eg, _             = errgroup.WithContext(ctx)
	)
	eg.Go(func() error {
		out, err := r.runLeftGenCmd(ctx)
		if err != nil {
			return err
		}
		leftOut = out
		return nil
	})
	eg.Go(func() error {
		out, err := r.runRightGenCmd(ctx)
		if err != nil {
			return err
		}
		rightOut = out
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &cmdResult{
		leftOut:  leftOut,
		rightOut: rightOut,
	}, nil
}

func (r runner) runGenCmdsWithInterceptor(ctx context.Context) (*cmdResult, error) {
	var (
		leftOut, rightOut string
		err               error
	)
	if leftOut, err = r.runLeftGenCmd(ctx); err != nil {
		return nil, err
	}
	if err := r.runInterceptors(ctx); err != nil {
		return nil, err
	}
	if rightOut, err = r.runRightGenCmd(ctx); err != nil {
		return nil, err
	}
	return &cmdResult{
		leftOut:  leftOut,
		rightOut: rightOut,
	}, nil
}

func (r runner) runGenCmds(ctx context.Context) (*cmdResult, error) {
	if len(r.Interceptor) > 0 {
		return r.runGenCmdsWithInterceptor(ctx)
	}
	return r.runGenCmdsConcurrently(ctx)
}

func (r runner) newPreprocessCmds() []*execx.Cmd {
	xs := make([]*execx.Cmd, len(r.Preprocess))
	for i, p := range r.Preprocess {
		logger := slog.With(slog.Int("count", i), slog.String("preprocess", p))
		logger.Debug("preprocess")
		xs[i] = r.newShellCmd(p)
	}
	return xs
}

func (r runner) runPreprocess(ctx context.Context, target, input string) (string, error) {
	slog.Debug(fmt.Sprintf("start %s preprocess", target), slog.String("in", input))
	stdin, err := os.Open(input)
	if err != nil {
		return "", fmt.Errorf("%w: run %s preprocess", err, target)
	}
	defer stdin.Close()
	p := execx.NewPipedCmd(ctx, r.TempDir, stdin, r.newPreprocessCmds()...)
	if err := p.Run(ctx); err != nil {
		return "", fmt.Errorf("%w: run %s preprocess", err, target)
	}
	slog.Debug(fmt.Sprintf("end %s preprocess", target), slog.String("out", p.Path()))
	return p.Path(), nil
}

func (r runner) runPreprocesses(ctx context.Context, left, right string) (*cmdResult, error) {
	if len(r.Preprocess) == 0 {
		return &cmdResult{
			leftOut:  left,
			rightOut: right,
		}, nil
	}

	var (
		leftOut, rightOut string
		eg, _             = errgroup.WithContext(ctx)
	)
	eg.Go(func() error {
		out, err := r.runPreprocess(ctx, "left", left)
		if err != nil {
			return err
		}
		leftOut = out
		return nil
	})
	eg.Go(func() error {
		out, err := r.runPreprocess(ctx, "right", right)
		if err != nil {
			return err
		}
		rightOut = out
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &cmdResult{
		leftOut:  leftOut,
		rightOut: rightOut,
	}, nil
}

func (r runner) runDiff(ctx context.Context, left, right string) error {
	slog.Debug("start run diff", slog.String("diff", r.Diff))
	cmd := exec.CommandContext(ctx, r.Shell, "-c", r.Diff+" "+left+" "+right)
	cmd.Stdout = r.Writer
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err != nil {
		err = errors.Join(ErrDiff, err)
	}
	slog.Debug("end run diff", slog.Any("err", err))
	return err
}

func (r runner) run(ctx context.Context) error {
	defer r.Close()

	result, err := r.runGenCmds(ctx)
	if err != nil {
		return err
	}

	result, err = r.runPreprocesses(ctx, result.leftOut, result.rightOut)
	if err != nil {
		return err
	}

	return r.runDiff(ctx, result.leftOut, result.rightOut)
}
