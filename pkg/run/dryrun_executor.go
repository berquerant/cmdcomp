package run

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// varFileRef represents a shell variable that holds a tmpdir-relative file path.
type varFileRef struct{ varName string }

func (r varFileRef) ShellExpr() string { return fmt.Sprintf(`"$%s"`, r.varName) }

// DryRunExecutor implements Executor by writing a commented, executable shell
// script to an io.Writer instead of running any commands.
//
// Because it implements Executor, any step routed through the interface
// is automatically captured in the script — no per-feature dry-run code is needed.
type DryRunExecutor struct {
	mu             sync.Mutex
	w              io.Writer
	shell          string
	timeout        time.Duration
	processTimeout time.Duration
}

// NewDryRunExecutor creates a DryRunExecutor and writes the script preamble immediately.
func NewDryRunExecutor(shell string, w io.Writer, timeout, processTimeout time.Duration) *DryRunExecutor {
	e := &DryRunExecutor{
		shell:          shell,
		w:              w,
		timeout:        timeout,
		processTimeout: processTimeout,
	}
	e.preamble()
	return e
}

func (e *DryRunExecutor) preamble() {
	fmt.Fprintf(e.w, "#!/usr/bin/env %s\n", e.shell)
	if e.timeout > 0 {
		fmt.Fprintf(e.w, "# timeout: %s\n", e.timeout)
	}
	if e.processTimeout > 0 {
		fmt.Fprintf(e.w, "# processTimeout: %s\n", e.processTimeout)
	}
	fmt.Fprintln(e.w, "set -euo pipefail")
	fmt.Fprintln(e.w, `_CMDCOMP_TMPDIR=$(mktemp -d)`)
	fmt.Fprintln(e.w, `trap 'rm -rf "$_CMDCOMP_TMPDIR"' EXIT`)
}

// stepVar converts a step name to an upper-case shell variable name.
// e.g. "preprocess:left" → "_CMDCOMP_PREPROCESS_LEFT"
func stepVar(name string) string {
	r := strings.NewReplacer(":", "_", "-", "_", " ", "_", ".", "_")
	return "_CMDCOMP_" + strings.ToUpper(r.Replace(name))
}

// tmpPath returns a shell expression for a tmpdir-relative file named after the step.
// e.g. "preprocess:left" → `"$_CMDCOMP_TMPDIR/preprocess_left"`
func tmpPath(name string) string {
	r := strings.NewReplacer(":", "_", "-", "_", " ", "_", ".", "_")
	return fmt.Sprintf(`"$_CMDCOMP_TMPDIR/%s"`, strings.ToLower(r.Replace(name)))
}

// shellQuote single-quotes a string for safe use in a generated shell script.
func shellQuote(s string) string {
	if !strings.ContainsAny(s, " \t\n\"'\\$`|&;()<>{}!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func joinArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}

// envPrefix returns "KEY=VAL ... " for inline env injection, or "" if empty.
func envPrefix(extraEnv []string) string {
	if len(extraEnv) == 0 {
		return ""
	}
	return strings.Join(extraEnv, " ") + " "
}

// literalFileRef represents a raw shell-quoted literal file path.
type literalFileRef struct{ path string }

func (r literalFileRef) ShellExpr() string { return shellQuote(r.path) }

func (e *DryRunExecutor) SetupStdin(_ context.Context, stdin string, _ io.Reader) (FileRef, error) {
	if stdin == "" {
		return nil, nil
	}
	if after, ok := strings.CutPrefix(stdin, "@"); ok {
		path := after
		return literalFileRef{path: path}, nil
	}
	if stdin == "-" {
		e.mu.Lock()
		defer e.mu.Unlock()
		fmt.Fprintf(e.w, "\n# stdin\n")
		fmt.Fprintln(e.w, `_CMDCOMP_STDIN=$(mktemp "$_CMDCOMP_TMPDIR/stdin.XXXXXX")`)
		fmt.Fprintln(e.w, `cat > "$_CMDCOMP_STDIN"`)
		return varFileRef{varName: "_CMDCOMP_STDIN"}, nil
	}
	return nil, fmt.Errorf("invalid stdin '%s'", stdin)
}

func (e *DryRunExecutor) RunHook(_ context.Context, req HookRequest) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	fmt.Fprintf(e.w, "\n# %s[%d]\n", req.Phase, req.Index)
	if len(req.ExtraEnv) > 0 {
		// Export in a subshell scope to avoid polluting subsequent steps.
		fmt.Fprintf(e.w, "export %s\n", strings.Join(req.ExtraEnv, " "))
	}
	fmt.Fprintln(e.w, req.Cmd)
	return nil
}

func (e *DryRunExecutor) RunGenCmd(_ context.Context, req GenCmdRequest) (FileRef, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	varN := stepVar(req.Name)
	fmt.Fprintf(e.w, "\n# %s\n", req.Name)
	fmt.Fprintf(e.w, "%s=%s\n", varN, tmpPath(req.Name))
	stdinRedirect := ""
	if req.Stdin != nil {
		stdinRedirect = fmt.Sprintf(" < %s", req.Stdin.ShellExpr())
	}
	fmt.Fprintf(e.w, "%s%s%s > \"$%s\"\n", envPrefix(req.ExtraEnv), joinArgs(req.Args), stdinRedirect, varN)
	return varFileRef{varName: varN}, nil
}

func (e *DryRunExecutor) RunPipeline(_ context.Context, req PipelineRequest) (FileRef, error) {
	if len(req.Cmds) == 0 {
		return req.Input, nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	varN := stepVar(req.Name)
	fmt.Fprintf(e.w, "\n# %s\n", req.Name)
	fmt.Fprintf(e.w, "%s=%s\n", varN, tmpPath(req.Name))
	// Redirect the input file into the first command; chain the rest with pipes.
	parts := make([]string, len(req.Cmds))
	copy(parts, req.Cmds)
	parts[0] = parts[0] + " < " + req.Input.ShellExpr()
	fmt.Fprintf(e.w, "%s%s > \"$%s\"\n", envPrefix(req.ExtraEnv), strings.Join(parts, " | "), varN)
	return varFileRef{varName: varN}, nil
}

func (e *DryRunExecutor) RunDiff(_ context.Context, req DiffRequest) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	fmt.Fprintf(e.w, "\n# diff\n")
	parts := []string{envPrefix(req.ExtraEnv) + req.Cmd, req.Left.ShellExpr(), req.Right.ShellExpr()}
	for _, l := range req.Labels {
		parts = append(parts, "--label", shellQuote(l))
	}
	fmt.Fprintln(e.w, strings.Join(parts, " "))
	return nil
}
