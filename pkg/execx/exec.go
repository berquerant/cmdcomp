package execx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
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

// Run executes the command and returns the filepath where the results were written.
func (c *Cmd) Run(ctx context.Context) (string, error) {
	if len(c.args) == 0 {
		return "", fmt.Errorf("%w: no args", ErrRun)
	}

	d, err := os.MkdirTemp(c.tmpDir, "cmdcomp")
	if err != nil {
		return "", err
	}

	path := filepath.Join(d, "out")
	stdout, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer stdout.Close()

	cmd := exec.CommandContext(ctx, c.args[0], c.args[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	slog.Debug("exec", slog.Any("args", cmd.Args))
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return path, nil
}
