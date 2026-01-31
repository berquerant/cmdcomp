package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

	logC := make(chan *cmdLog, 100)
	runner := &runner{
		Config: c,
		logC:   logC,
	}

	doneC := make(chan error)
	go func() {
		err := runner.run(ctx)
		close(logC)
		doneC <- err
	}()

	for x := range logC {
		if c.ShowCmdLog {
			slog.Info("command log", x.intoSlogAttrs()...)
		}
	}

	return <-doneC
}

// An error from diff command.
var ErrDiff = errors.New("Diff")

type runner struct {
	*config.Config
	logC chan *cmdLog
}

type cmdLog struct {
	args    []string
	in      string
	out     string
	start   time.Time
	end     time.Time
	elapsed int64
	err     string
}

func (c cmdLog) intoSlogAttrs() []any {
	xs := []any{}
	xs = append(xs, slog.String("args", strings.Join(c.args, " ")))
	if x := c.in; x != "" {
		xs = append(xs, slog.String("in", x))
	}
	if x := c.out; x != "" {
		xs = append(xs, slog.String("out", x))
	}
	xs = append(xs, slog.Time("start", c.start))
	xs = append(xs, slog.Time("end", c.end))
	xs = append(xs, slog.Int64("elapsed_ms", c.elapsed))
	if x := c.err; x != "" {
		xs = append(xs, slog.String("err", x))
	}
	return xs
}

func newCmdLog(args []string) *cmdLog {
	return &cmdLog{
		args:  args,
		start: time.Now(),
	}
}

func (c *cmdLog) close(out string, err error) {
	c.end = time.Now()
	c.elapsed = c.end.Sub(c.start).Milliseconds()
	c.out = out
	if err != nil {
		c.err = err.Error()
	}
}

func (r *runner) runCmd(ctx context.Context, arg ...string) (string, error) {
	c := execx.NewCmd(r.TempDir, arg...)
	x := newCmdLog(arg)
	out, err := c.Run(ctx)
	x.close(out, err)
	r.logC <- x
	return out, err
}

func (r *runner) runGenCmd(ctx context.Context, target string, arg ...string) (string, error) {
	slog.Debug(fmt.Sprintf("start run %s", target), slog.Any("args", arg))
	out, err := r.runCmd(ctx, arg...)
	if err != nil {
		return "", fmt.Errorf("%w: run %s", err, target)
	}
	slog.Debug(fmt.Sprintf("end run %s", target), slog.String("out", out))
	return out, nil
}

func (r *runner) newShellCmd(arg ...string) *execx.Cmd {
	return execx.NewCmd(r.TempDir, append([]string{r.Shell, "-c"}, arg...)...)
}

func (r *runner) runInterceptors(ctx context.Context) error {
	for i, p := range r.Interceptor {
		logger := slog.With(slog.Int("count", i), slog.String("interceptor", p))
		logger.Debug("start run interceptor")
		cmd := exec.CommandContext(ctx, r.Shell, "-c", p)
		cmd.Stdout = os.Stderr // interceptor stdout cannot be mixed with diff stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		x := newCmdLog(cmd.Args)
		err := cmd.Run()
		x.close("", err)
		r.logC <- x
		if err != nil {
			return fmt.Errorf("%w: run interceptor[%d]", err, i)
		}
		slog.Debug("end run interceptor")
	}
	return nil
}

func (r *runner) runLeftGenCmd(ctx context.Context) (string, error) {
	return r.runGenCmd(ctx, "left", r.GetLeftArgs()...)
}

func (r *runner) runRightGenCmd(ctx context.Context) (string, error) {
	return r.runGenCmd(ctx, "right", r.GetRightArgs()...)
}

type cmdResult struct {
	leftOut, rightOut string
}

func (r *runner) runGenCmdsConcurrently(ctx context.Context) (*cmdResult, error) {
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

func (r *runner) runGenCmdsWithInterceptor(ctx context.Context) (*cmdResult, error) {
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

func (r *runner) runGenCmds(ctx context.Context) (*cmdResult, error) {
	if len(r.Interceptor) > 0 {
		return r.runGenCmdsWithInterceptor(ctx)
	}
	return r.runGenCmdsConcurrently(ctx)
}

func (r *runner) newPreprocessCmds() []*execx.Cmd {
	xs := make([]*execx.Cmd, len(r.Preprocess))
	for i, p := range r.Preprocess {
		logger := slog.With(slog.Int("count", i), slog.String("preprocess", p))
		logger.Debug("preprocess")
		xs[i] = r.newShellCmd(p)
	}
	return xs
}

func (r *runner) runPreprocess(ctx context.Context, target, input string) (string, error) {
	slog.Debug(fmt.Sprintf("start %s preprocess", target), slog.String("in", input))
	stdin, err := os.Open(input)
	if err != nil {
		return "", fmt.Errorf("%w: run %s preprocess", err, target)
	}
	defer stdin.Close()
	cmds := r.newPreprocessCmds()
	p := execx.NewPipedCmd(ctx, r.TempDir, stdin, cmds...)
	logs := make([]*cmdLog, len(cmds))
	for i, x := range cmds {
		v, _ := x.IntoExecCmd(ctx)
		logs[i] = newCmdLog(v.Args)
	}
	logs[0].in = input
	err = p.Run(ctx)
	for _, x := range logs {
		x.close(p.Path(), err)
	}
	for _, x := range logs {
		r.logC <- x
	}
	if err != nil {
		return "", fmt.Errorf("%w: run %s preprocess", err, target)
	}
	slog.Debug(fmt.Sprintf("end %s preprocess", target), slog.String("out", p.Path()))
	return p.Path(), nil
}

func (r *runner) runPreprocesses(ctx context.Context, left, right string) (*cmdResult, error) {
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

func (r *runner) newRunDiffArgument(left, right string) []string {
	xs := []string{
		r.Diff,
		left,
		right,
	}
	if r.UseLabel {
		// use '___' to join the arguments.
		// since they are passed as bash -c, using ' ' delimiters makes correct escaping complicated
		xs = append(xs, "--label", strings.Join(r.GetLeftArgs(), "___"))
		xs = append(xs, "--label", strings.Join(r.GetRightArgs(), "___"))
	}
	return xs
}

func (r *runner) runDiff(ctx context.Context, left, right string) error {
	cmd := exec.CommandContext(ctx, r.Shell, "-c", strings.Join(r.newRunDiffArgument(left, right), " "))
	slog.Debug("start run diff", slog.Any("cmd", cmd.Args))
	cmd.Stdout = r.Writer
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	x := newCmdLog(cmd.Args)
	err := cmd.Run()
	x.close("", err)
	r.logC <- x
	if err != nil {
		err = errors.Join(ErrDiff, err)
	}
	slog.Debug("end run diff", slog.Any("err", err))
	return err
}

func (r *runner) run(ctx context.Context) error {
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
