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

	"github.com/berquerant/cmdcomp/pkg/config"
	"golang.org/x/sync/errgroup"
)

func Main(c *config.Config) error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGPIPE,
	)
	defer stop()

	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	var exec Executor
	if c.DryRun {
		exec = NewDryRunExecutor(c.Shell, c.Writer, c.Timeout, c.ProcessTimeout)
	} else {
		exec = newRealExecutor(c.TempDir, c.Shell, c.ShowCmdLog, c.Writer, c.ProcessTimeout)
	}

	r := &runner{Config: c, exec: exec}
	return r.run(ctx)
}

// Errors returned by execution phases.
var (
	ErrDiff     = errors.New("Diff")
	ErrHook     = errors.New("Hook")
	ErrGenCmd   = errors.New("GenCmd")
	ErrPipeline = errors.New("Pipeline")
)

// ExitCode determines the process exit code for a run result and configuration.
// It returns 0 when err is nil or when diff was detected (exit code 1) and success is true.
// It returns 1 when diff was detected and success is false.
// It returns 2 for execution failures.
func ExitCode(err error, success bool) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrDiff) {
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		code := 1
		if ok {
			code = exitErr.ExitCode()
		}
		if success && code == 1 {
			return 0
		}
		return code
	}
	return 2
}

type runner struct {
	*config.Config
	exec Executor
}

type genResult struct {
	leftRef, rightRef FileRef
}

func (r *runner) run(ctx context.Context) (resultErr error) {
	defer func() {
		cleanupErr := r.runHooks(ctx, "cleanup", r.Config.Cleanup)
		resultErr = errors.Join(resultErr, cleanupErr)
		_ = r.Config.Close()
	}()

	stdinRes, err := r.exec.SetupStdin(ctx, SetupInputRequest{
		Left:   r.Config.GetLeftStdin(),
		Right:  r.Config.GetRightStdin(),
		Reader: r.Config.Reader,
	})
	if err != nil {
		return err
	}

	snapRes, err := r.exec.SetupSnapshot(ctx, SetupInputRequest{
		Left:   r.Config.GetLeftSnapshot(),
		Right:  r.Config.GetRightSnapshot(),
		Reader: r.Config.Reader,
	})
	if err != nil {
		return err
	}

	if err := r.runHooks(ctx, "startup", r.Config.Startup); err != nil {
		return err
	}

	result, err := r.runGenCmds(ctx, stdinRes, snapRes)
	if err != nil {
		return err
	}

	result, err = r.runPreprocesses(ctx, result)
	if err != nil {
		return err
	}

	return r.runDiff(ctx, result.leftRef, result.rightRef)
}

// runHooks executes each command in cmds sequentially under the named phase.
func (r *runner) runHooks(ctx context.Context, phase string, cmds []string) error {
	for i, cmd := range cmds {
		slog.Debug(fmt.Sprintf("hook %s[%d]", phase, i), slog.String("cmd", cmd))
		if err := r.exec.RunHook(ctx, HookRequest{
			Phase:    phase,
			Index:    i,
			ExtraEnv: r.Config.Env,
			Cmd:      cmd,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) runGenCmds(ctx context.Context, stdinRes *SetupInputResult, snapRes *SetupInputResult) (*genResult, error) {
	if len(r.Config.Interceptor) > 0 {
		return r.runGenCmdsWithInterceptor(ctx, stdinRes, snapRes)
	}
	return r.runGenCmdsConcurrently(ctx, stdinRes, snapRes)
}

func (r *runner) runSideGenCmd(ctx context.Context, name string, snapRef FileRef, extraEnv, args []string, stdinRef FileRef) (FileRef, error) {
	if snapRef != nil {
		return snapRef, nil
	}
	return r.exec.RunGenCmd(ctx, GenCmdRequest{
		Name:     name,
		ExtraEnv: extraEnv,
		Args:     args,
		Stdin:    stdinRef,
	})
}

// runGenCmdsConcurrently runs left and right commands in parallel when no interceptor is set.
// If a snapshot is configured for a side, command execution is skipped for that side.
func (r *runner) runGenCmdsConcurrently(ctx context.Context, stdinRes *SetupInputResult, snapRes *SetupInputResult) (*genResult, error) {
	var (
		leftRef, rightRef FileRef
		eg, _             = errgroup.WithContext(ctx)
	)
	eg.Go(func() error {
		ref, err := r.runSideGenCmd(ctx, "left", snapRes.LeftRef, r.Config.GetLeftEnv(), r.Config.GetLeftArgs(), stdinRes.LeftRef)
		if err != nil {
			return err
		}
		leftRef = ref
		return nil
	})
	eg.Go(func() error {
		ref, err := r.runSideGenCmd(ctx, "right", snapRes.RightRef, r.Config.GetRightEnv(), r.Config.GetRightArgs(), stdinRes.RightRef)
		if err != nil {
			return err
		}
		rightRef = ref
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &genResult{leftRef: leftRef, rightRef: rightRef}, nil
}

// runGenCmdsWithInterceptor runs left, interceptors, then right sequentially.
// If a snapshot is configured for a side, command execution is skipped for that side.
func (r *runner) runGenCmdsWithInterceptor(ctx context.Context, stdinRes *SetupInputResult, snapRes *SetupInputResult) (*genResult, error) {
	leftRef, err := r.runSideGenCmd(ctx, "left", snapRes.LeftRef, r.Config.GetLeftEnv(), r.Config.GetLeftArgs(), stdinRes.LeftRef)
	if err != nil {
		return nil, err
	}
	if err := r.runHooks(ctx, "interceptor", r.Config.Interceptor); err != nil {
		return nil, err
	}
	rightRef, err := r.runSideGenCmd(ctx, "right", snapRes.RightRef, r.Config.GetRightEnv(), r.Config.GetRightArgs(), stdinRes.RightRef)
	if err != nil {
		return nil, err
	}
	return &genResult{leftRef: leftRef, rightRef: rightRef}, nil
}

// runPreprocesses applies preprocess pipelines to left and right outputs in parallel.
func (r *runner) runPreprocesses(ctx context.Context, result *genResult) (*genResult, error) {
	var (
		leftRef, rightRef FileRef
		eg, _             = errgroup.WithContext(ctx)
	)
	eg.Go(func() error {
		ref, err := r.exec.RunPipeline(ctx, PipelineRequest{
			Name:     "preprocess:left",
			ExtraEnv: r.Config.GetLeftEnv(),
			Input:    result.leftRef,
			Cmds:     r.Config.GetLeftPreprocess(),
		})
		if err != nil {
			return err
		}
		leftRef = ref
		return nil
	})
	eg.Go(func() error {
		ref, err := r.exec.RunPipeline(ctx, PipelineRequest{
			Name:     "preprocess:right",
			ExtraEnv: r.Config.GetRightEnv(),
			Input:    result.rightRef,
			Cmds:     r.Config.GetRightPreprocess(),
		})
		if err != nil {
			return err
		}
		rightRef = ref
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &genResult{leftRef: leftRef, rightRef: rightRef}, nil
}

func (r *runner) runDiff(ctx context.Context, left, right FileRef) error {
	var labels []string
	if r.Config.UseLabel {
		// Use '___' as arg separator: spaces make shell escaping complicated.
		labels = []string{
			strings.Join(r.Config.GetLeftArgs(), "___"),
			strings.Join(r.Config.GetRightArgs(), "___"),
		}
	}
	return r.exec.RunDiff(ctx, DiffRequest{
		Cmd:      r.Config.Diff,
		ExtraEnv: r.Config.Env,
		Labels:   labels,
		Left:     left,
		Right:    right,
		Success:  r.Config.Success,
	})
}
