package run

import "context"

// FileRef is a handle to command output.
// RealExecutor wraps real filesystem paths; DryRunExecutor wraps shell variable names.
// By threading FileRef through pipeline steps, neither the runner nor callers
// need to know which implementation is active.
type FileRef interface {
	// ShellExpr returns a shell-safe expression for this file:
	// a quoted path in real mode, or a variable reference (e.g. "$_CMDCOMP_LEFT") in dry-run.
	ShellExpr() string
}

// HookRequest holds the parameters for a single hook invocation.
type HookRequest struct {
	// Phase is the hook phase name (startup, interceptor, cleanup).
	Phase string
	// Index is the 0-based position within the phase.
	Index int
	// ExtraEnv holds KEY=VALUE pairs appended to os.Environ().
	ExtraEnv []string
	// Cmd is the shell command string to execute.
	Cmd string
}

// GenCmdRequest holds the parameters for a command that produces output.
type GenCmdRequest struct {
	// Name is used for labelling and variable/path naming.
	Name     string
	ExtraEnv []string
	Args     []string
}

// PipelineRequest holds the parameters for a pipeline of shell commands.
type PipelineRequest struct {
	Name     string
	ExtraEnv []string
	// Input is the FileRef whose content is piped into the first command.
	Input FileRef
	// Cmds is the list of shell command strings to chain with pipes.
	Cmds []string
}

// DiffRequest holds the parameters for the diff step.
type DiffRequest struct {
	// Cmd is the diff command string (e.g. "diff -u").
	Cmd      string
	ExtraEnv []string
	// Labels, if non-empty, are appended as successive --label arguments.
	Labels []string
	Left   FileRef
	Right  FileRef
}

// Executor abstracts how each execution step in cmdcomp is carried out.
//
// Architecture contract: every command invocation in cmdcomp MUST be expressed
// through a method on this interface. This guarantees that RealExecutor (actual
// execution) and DryRunExecutor (shell-script generation) stay in sync
// automatically. When a new pipeline step is added, routing it through the
// existing Executor methods grants dry-run support with no additional effort.
// Adding fields to request structs extends behaviour without breaking callers.
type Executor interface {
	RunHook(ctx context.Context, req HookRequest) error
	RunGenCmd(ctx context.Context, req GenCmdRequest) (FileRef, error)
	RunPipeline(ctx context.Context, req PipelineRequest) (FileRef, error)
	RunDiff(ctx context.Context, req DiffRequest) error
}
