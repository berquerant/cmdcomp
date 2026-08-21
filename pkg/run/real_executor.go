package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/berquerant/cmdcomp/pkg/execx"
)

// pathFileRef wraps a real filesystem path for use as a FileRef.
type pathFileRef struct{ path string }

func (r pathFileRef) ShellExpr() string { return r.path }

// cmdLog records the execution details of a single command run.
type cmdLog struct {
	args    []string
	in      string
	out     string
	start   time.Time
	end     time.Time
	elapsed int64
	err     string
}

func newCmdLog(args []string) *cmdLog {
	return &cmdLog{args: args, start: time.Now()}
}

func (c *cmdLog) close(out string, err error) {
	c.end = time.Now()
	c.elapsed = c.end.Sub(c.start).Milliseconds()
	c.out = out
	if err != nil {
		c.err = err.Error()
	}
}

func (c cmdLog) intoSlogAttrs() []any {
	xs := []any{slog.String("args", strings.Join(c.args, " "))}
	if x := c.in; x != "" {
		xs = append(xs, slog.String("in", x))
	}
	if x := c.out; x != "" {
		xs = append(xs, slog.String("out", x))
	}
	xs = append(xs, slog.Time("start", c.start), slog.Time("end", c.end),
		slog.Int64("elapsed_ms", c.elapsed))
	if x := c.err; x != "" {
		xs = append(xs, slog.String("err", x))
	}
	return xs
}

// RealExecutor implements Executor by actually running commands.
type RealExecutor struct {
	tmpDir         string
	shell          string
	showCmdLog     bool
	writer         io.Writer
	processTimeout time.Duration
}

func newRealExecutor(tmpDir, shell string, showCmdLog bool, writer io.Writer, processTimeout time.Duration) *RealExecutor {
	return &RealExecutor{
		tmpDir:         tmpDir,
		shell:          shell,
		showCmdLog:     showCmdLog,
		writer:         writer,
		processTimeout: processTimeout,
	}
}

func (e *RealExecutor) withProcessTimeout(ctx context.Context, ignore bool) (context.Context, context.CancelFunc) {
	if ignore || e.processTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, e.processTimeout)
}

func (e *RealExecutor) logCmd(cl *cmdLog) {
	if e.showCmdLog {
		slog.Info("command log", cl.intoSlogAttrs()...)
	}
}

func (e *RealExecutor) SetupStdin(ctx context.Context, stdin string, r io.Reader) (FileRef, error) {
	if stdin == "" {
		return nil, nil
	}
	if after, ok := strings.CutPrefix(stdin, "@"); ok {
		path := after
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("%w: stat stdin file %s", err, path)
		}
		return pathFileRef{path: path}, nil
	}
	if stdin == "-" {
		tmpfile := execx.NewTmpFile(e.tmpDir)
		f, err := tmpfile.Open()
		if err != nil {
			return nil, fmt.Errorf("%w: create tempfile for stdin", err)
		}
		defer f.Close()
		if r == nil {
			r = os.Stdin
		}
		if _, err := io.Copy(f, r); err != nil {
			return nil, fmt.Errorf("%w: copy stdin to tempfile", err)
		}
		return pathFileRef{path: tmpfile.Path()}, nil
	}
	return nil, fmt.Errorf("invalid stdin '%s'", stdin)
}

func (e *RealExecutor) RunHook(ctx context.Context, req HookRequest) error {
	slog.Debug(fmt.Sprintf("start hook %s[%d]", req.Phase, req.Index))
	runCtx, cancel := e.withProcessTimeout(ctx, req.Phase == "cleanup")
	defer cancel()

	c := exec.CommandContext(runCtx, e.shell, "-c", req.Cmd)
	// Hook stdout must not mix with diff output on stdout.
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), req.ExtraEnv...)
	cl := newCmdLog(c.Args)
	err := c.Run()
	cl.close("", err)
	e.logCmd(cl)
	if err != nil {
		return errors.Join(ErrHook, fmt.Errorf("%w: run %s[%d]", err, req.Phase, req.Index))
	}
	slog.Debug(fmt.Sprintf("end hook %s[%d]", req.Phase, req.Index))
	return nil
}

func (e *RealExecutor) RunGenCmd(ctx context.Context, req GenCmdRequest) (FileRef, error) {
	slog.Debug(fmt.Sprintf("start gen %s", req.Name), slog.Any("args", req.Args))
	runCtx, cancel := e.withProcessTimeout(ctx, false)
	defer cancel()

	c := execx.NewCmd(e.tmpDir, req.Args...)
	c.Env = append(os.Environ(), req.ExtraEnv...)
	if req.Stdin != nil {
		f, err := os.Open(req.Stdin.ShellExpr())
		if err != nil {
			return nil, errors.Join(ErrGenCmd, fmt.Errorf("%w: open stdin for %s", err, req.Name))
		}
		defer f.Close()
		c.Stdin = f
	}
	cl := newCmdLog(req.Args)
	out, err := c.Run(runCtx)
	cl.close(out, err)
	e.logCmd(cl)
	if err != nil {
		return nil, errors.Join(ErrGenCmd, fmt.Errorf("%w: run %s", err, req.Name))
	}
	slog.Debug(fmt.Sprintf("end gen %s", req.Name), slog.String("out", out))
	return pathFileRef{path: out}, nil
}

func (e *RealExecutor) RunPipeline(ctx context.Context, req PipelineRequest) (FileRef, error) {
	if len(req.Cmds) == 0 {
		return req.Input, nil
	}
	slog.Debug(fmt.Sprintf("start pipeline %s", req.Name), slog.String("in", req.Input.ShellExpr()))
	runCtx, cancel := e.withProcessTimeout(ctx, false)
	defer cancel()

	xs := make([]*execx.Cmd, len(req.Cmds))
	for i, p := range req.Cmds {
		xs[i] = execx.NewCmd(e.tmpDir, e.shell, "-c", p)
		xs[i].Env = append(os.Environ(), req.ExtraEnv...)
	}
	stdin, err := os.Open(req.Input.ShellExpr())
	if err != nil {
		return nil, fmt.Errorf("%w: open input for %s pipeline", err, req.Name)
	}
	defer stdin.Close()
	p := execx.NewPipedCmd(runCtx, e.tmpDir, stdin, xs...)
	logs := make([]*cmdLog, len(xs))
	for i, x := range xs {
		v, _ := x.IntoExecCmd(runCtx)
		logs[i] = newCmdLog(v.Args)
	}
	logs[0].in = req.Input.ShellExpr()
	runErr := p.Run(runCtx)
	for _, cl := range logs {
		cl.close(p.Path(), runErr)
		e.logCmd(cl)
	}
	if runErr != nil {
		return nil, errors.Join(ErrPipeline, fmt.Errorf("%w: run %s pipeline", runErr, req.Name))
	}
	slog.Debug(fmt.Sprintf("end pipeline %s", req.Name), slog.String("out", p.Path()))
	return pathFileRef{path: p.Path()}, nil
}

func (e *RealExecutor) RunDiff(ctx context.Context, req DiffRequest) error {
	args := []string{req.Cmd, shellQuote(req.Left.ShellExpr()), shellQuote(req.Right.ShellExpr())}
	for _, l := range req.Labels {
		args = append(args, "--label", shellQuote(l))
	}
	slog.Debug("start run diff", slog.Any("args", args))
	runCtx, cancel := e.withProcessTimeout(ctx, false)
	defer cancel()

	c := exec.CommandContext(runCtx, e.shell, "-c", strings.Join(args, " "))
	c.Stdout = e.writer
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), req.ExtraEnv...)
	cl := newCmdLog(c.Args)
	err := c.Run()
	cl.close("", err)
	e.logCmd(cl)
	if err != nil {
		err = errors.Join(ErrDiff, fmt.Errorf("%w: run diff", err))
	}
	slog.Debug("end run diff", slog.Any("err", err))
	return err
}
