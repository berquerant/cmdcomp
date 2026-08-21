package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
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

	stdinRef, err := r.exec.SetupStdin(ctx, r.Config.Stdin, r.Config.Reader)
	if err != nil {
		return err
	}

	if err := r.runHooks(ctx, "startup", r.Config.Startup); err != nil {
		return err
	}

	result, err := r.runGenCmds(ctx, stdinRef)
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

func (r *runner) runGenCmds(ctx context.Context, stdinRef FileRef) (*genResult, error) {
	if len(r.Config.Interceptor) > 0 {
		return r.runGenCmdsWithInterceptor(ctx, stdinRef)
	}
	return r.runGenCmdsConcurrently(ctx, stdinRef)
}

// runGenCmdsConcurrently runs left and right commands in parallel when no interceptor is set.
func (r *runner) runGenCmdsConcurrently(ctx context.Context, stdinRef FileRef) (*genResult, error) {
	var (
		leftRef, rightRef FileRef
		eg, _             = errgroup.WithContext(ctx)
	)
	eg.Go(func() error {
		ref, err := r.exec.RunGenCmd(ctx, GenCmdRequest{
			Name:     "left",
			ExtraEnv: r.Config.GetLeftEnv(),
			Args:     r.Config.GetLeftArgs(),
			Stdin:    stdinRef,
		})
		if err != nil {
			return err
		}
		leftRef = ref
		return nil
	})
	eg.Go(func() error {
		ref, err := r.exec.RunGenCmd(ctx, GenCmdRequest{
			Name:     "right",
			ExtraEnv: r.Config.GetRightEnv(),
			Args:     r.Config.GetRightArgs(),
			Stdin:    stdinRef,
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

// runGenCmdsWithInterceptor runs left, interceptors, then right sequentially.
func (r *runner) runGenCmdsWithInterceptor(ctx context.Context, stdinRef FileRef) (*genResult, error) {
	leftRef, err := r.exec.RunGenCmd(ctx, GenCmdRequest{
		Name:     "left",
		ExtraEnv: r.Config.GetLeftEnv(),
		Args:     r.Config.GetLeftArgs(),
		Stdin:    stdinRef,
	})
	if err != nil {
		return nil, err
	}
	if err := r.runHooks(ctx, "interceptor", r.Config.Interceptor); err != nil {
		return nil, err
	}
	rightRef, err := r.exec.RunGenCmd(ctx, GenCmdRequest{
		Name:     "right",
		ExtraEnv: r.Config.GetRightEnv(),
		Args:     r.Config.GetRightArgs(),
		Stdin:    stdinRef,
	})
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
	})
}
