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
	tmpDir     string
	shell      string
	showCmdLog bool
	writer     io.Writer
}

func newRealExecutor(tmpDir, shell string, showCmdLog bool, writer io.Writer) *RealExecutor {
	return &RealExecutor{
		tmpDir:     tmpDir,
		shell:      shell,
		showCmdLog: showCmdLog,
		writer:     writer,
	}
}

func (e *RealExecutor) logCmd(cl *cmdLog) {
	if e.showCmdLog {
		slog.Info("command log", cl.intoSlogAttrs()...)
	}
}

func (e *RealExecutor) RunHook(ctx context.Context, req HookRequest) error {
	slog.Debug(fmt.Sprintf("start hook %s[%d]", req.Phase, req.Index))
	c := exec.CommandContext(ctx, e.shell, "-c", req.Cmd)
	// Hook stdout must not mix with diff output on stdout.
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), req.ExtraEnv...)
	cl := newCmdLog(c.Args)
	err := c.Run()
	cl.close("", err)
	e.logCmd(cl)
	if err != nil {
		return fmt.Errorf("%w: run %s[%d]", err, req.Phase, req.Index)
	}
	slog.Debug(fmt.Sprintf("end hook %s[%d]", req.Phase, req.Index))
	return nil
}

func (e *RealExecutor) RunGenCmd(ctx context.Context, req GenCmdRequest) (FileRef, error) {
	slog.Debug(fmt.Sprintf("start gen %s", req.Name), slog.Any("args", req.Args))
	c := execx.NewCmd(e.tmpDir, req.Args...)
	c.Env = append(os.Environ(), req.ExtraEnv...)
	cl := newCmdLog(req.Args)
	out, err := c.Run(ctx)
	cl.close(out, err)
	e.logCmd(cl)
	if err != nil {
		return nil, fmt.Errorf("%w: run %s", err, req.Name)
	}
	slog.Debug(fmt.Sprintf("end gen %s", req.Name), slog.String("out", out))
	return pathFileRef{path: out}, nil
}

func (e *RealExecutor) RunPipeline(ctx context.Context, req PipelineRequest) (FileRef, error) {
	if len(req.Cmds) == 0 {
		return req.Input, nil
	}
	slog.Debug(fmt.Sprintf("start pipeline %s", req.Name), slog.String("in", req.Input.ShellExpr()))
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
	p := execx.NewPipedCmd(ctx, e.tmpDir, stdin, xs...)
	logs := make([]*cmdLog, len(xs))
	for i, x := range xs {
		v, _ := x.IntoExecCmd(ctx)
		logs[i] = newCmdLog(v.Args)
	}
	logs[0].in = req.Input.ShellExpr()
	runErr := p.Run(ctx)
	for _, cl := range logs {
		cl.close(p.Path(), runErr)
		e.logCmd(cl)
	}
	if runErr != nil {
		return nil, fmt.Errorf("%w: run %s pipeline", runErr, req.Name)
	}
	slog.Debug(fmt.Sprintf("end pipeline %s", req.Name), slog.String("out", p.Path()))
	return pathFileRef{path: p.Path()}, nil
}

func (e *RealExecutor) RunDiff(ctx context.Context, req DiffRequest) error {
	args := []string{req.Cmd, req.Left.ShellExpr(), req.Right.ShellExpr()}
	for _, l := range req.Labels {
		args = append(args, "--label", l)
	}
	slog.Debug("start run diff", slog.Any("args", args))
	c := exec.CommandContext(ctx, e.shell, "-c", strings.Join(args, " "))
	c.Stdout = e.writer
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), req.ExtraEnv...)
	cl := newCmdLog(c.Args)
	err := c.Run()
	cl.close("", err)
	e.logCmd(cl)
	if err != nil {
		err = errors.Join(ErrDiff, err)
	}
	slog.Debug("end run diff", slog.Any("err", err))
	return err
}
