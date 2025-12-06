package execx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	ex "github.com/berquerant/execx"
)

type Cmd struct {
	tmpDir string
	args   []string
}

func NewCmd(tmpDir string, arg ...string) *Cmd {
	return &Cmd{
		tmpDir: tmpDir,
		args:   arg,
	}
}

var ErrRun = errors.New("Run")

func (c *Cmd) intoExecCmd(ctx context.Context) (*exec.Cmd, error) {
	if len(c.args) == 0 {
		return nil, fmt.Errorf("%w: no args", ErrRun)
	}

	cmd := exec.CommandContext(ctx, c.args[0], c.args[1:]...)
	cmd.Env = os.Environ()
	return cmd, nil
}

// Run executes the command and returns the filepath where the results were written.
func (c *Cmd) Run(ctx context.Context) (string, error) {
	cmd, err := c.intoExecCmd(ctx)
	if err != nil {
		return "", err
	}

	tmpfile := NewTmpFile(c.tmpDir)
	stdout, err := tmpfile.Open()
	if err != nil {
		return "", err
	}
	defer stdout.Close()

	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	slog.Debug("exec", slog.Any("args", cmd.Args))
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return tmpfile.Path(), nil
}

type TmpFile struct {
	dir  string
	path string
}

func NewTmpFile(dir string) *TmpFile {
	return &TmpFile{
		dir: dir,
	}
}

func (f TmpFile) Path() string {
	return f.path
}

func (f *TmpFile) Open() (*os.File, error) {
	d, err := os.MkdirTemp(f.dir, "cmdcomp")
	if err != nil {
		return nil, err
	}
	f.path = filepath.Join(d, "out")
	return os.Create(f.path)
}

type Pipeline struct {
	cmds  []*Cmd
	stdin io.Reader
	dir   string
	path  string
}

func NewPipedCmd(ctx context.Context, dir string, stdin io.Reader, cmd ...*Cmd) *Pipeline {
	return &Pipeline{
		cmds:  cmd,
		dir:   dir,
		stdin: stdin,
	}
}

func (p Pipeline) Path() string {
	return p.path
}

func (p *Pipeline) Run(ctx context.Context) error {
	xs := make([]*exec.Cmd, len(p.cmds))
	for i, c := range p.cmds {
		x, err := c.intoExecCmd(ctx)
		if err != nil {
			return fmt.Errorf("%w: failed to convert cmds[%d] to exec.Cmd", err, i)
		}
		xs[i] = x
	}
	cmd, err := ex.NewPipedCmd(xs...)
	if err != nil {
		return err
	}

	stdoutFile := NewTmpFile(p.dir)
	stdout, err := stdoutFile.Open()
	if err != nil {
		return err
	}
	defer stdout.Close()

	p.path = stdoutFile.Path()
	cmd.Stdin = p.stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(ctx); err != nil {
		return err
	}
	return cmd.Wait()
}
