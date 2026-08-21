package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/berquerant/cmdcomp/pkg/slicex"
)

var (
	ErrConfig = errors.New("Config")
)

type Config struct {
	ShowCmdLog      bool          `name:"show-cmd-log" usage:"show command logs" yaml:"showCmdLog,omitempty"`
	Debug           bool          `name:"debug" usage:"enable debug logs" yaml:"debug,omitempty"`
	DryRun          bool          `name:"dry-run" usage:"print the shell commands that would be executed, then exit without running them" yaml:"dryrun,omitempty"`
	Startup         []string      `name:"startup" short:"s" usage:"process before running commands; invoked like 'startup'" yaml:"startup,omitempty"`
	Interceptor     []string      `name:"interceptor" short:"i" usage:"process after left command and before right command; invoked like 'interceptor'" yaml:"interceptor,omitempty"`
	Preprocess      []string      `name:"preprocess" short:"p" usage:"process before diff; invoked like 'preprocess'; should read input from stdin; should output result to stdout" yaml:"preprocess,omitempty"`
	LeftPreprocess  []string      `name:"left-preprocess" usage:"additional left process before diff; invoked like 'left-preprocess'; should read input from stdin; should output result to stdout" yaml:"leftPreprocess,omitempty"`
	RightPreprocess []string      `name:"right-preprocess" usage:"additional right process before diff; invoked like 'right-preprocess'; should read input from stdin; should output result to stdout" yaml:"rightPreprocess,omitempty"`
	Diff            string        `name:"diff" short:"x" default:"diff" usage:"diff command; invoked like 'diff LEFT_FILE RIGHT_FILE'" yaml:"diff,omitempty"`
	WorkDir         string        `name:"work-dir" short:"w" usage:"working directory; keep temporary files" yaml:"workDir,omitempty"`
	Shell           string        `name:"shell" short:"S" default:"bash" usage:"shell command to be executed" yaml:"shell,omitempty"`
	Delimiter       string        `name:"delimiter" short:"d" default:"--" usage:"arguments delimiter;\nchange the '--' separating COMMON_ARGS, LEFT_ARGS, and RIGHT_ARGS in this" yaml:"delimiter,omitempty"`
	UseLabel        bool          `name:"label" short:"l" usage:"use '--label' option of diff command" yaml:"label,omitempty"`
	Env             []string      `name:"env" short:"e" usage:"process environment variables;\nPassed to all processes along with os.Environ.\n--left-env is also passed to left output and left preprocess.\n--right-env is also passed to right output and right preprocess." yaml:"env,omitempty"`
	LeftEnv         []string      `name:"left-env" usage:"left process environment variables" yaml:"leftEnv,omitempty"`
	RightEnv        []string      `name:"right-env" usage:"right process environment variables" yaml:"rightEnv,omitempty"`
	Cleanup         []string      `name:"cleanup" short:"c" usage:"process before exiting cmdcomp process; invoked like 'cleanup'" yaml:"cleanup,omitempty"`

	CommonArgs []string `name:"-" yaml:"commonArgs,omitempty"`
	LeftArgs   []string `name:"-" yaml:"leftArgs,omitempty"`
	RightArgs  []string `name:"-" yaml:"rightArgs,omitempty"`

	Writer         io.Writer     `name:"-" json:"-" yaml:"-"`
	TempDir        string        `name:"-" json:"-" yaml:"-"`
	Timeout        time.Duration `name:"timeout" usage:"timeout for entire command execution" yaml:"timeout,omitempty"`
	ProcessTimeout time.Duration `name:"process-timeout" usage:"timeout for each individual process execution" yaml:"processTimeout,omitempty"`
}

func (c *Config) Init(args []string) error {
	if err := c.setTempDir(); err != nil {
		return err
	}
	if err := c.setArgs(args); err != nil {
		return err
	}
	return nil
}

func (c *Config) Close() error {
	if c.DryRun || c.WorkDir != "" {
		return nil
	}
	return os.RemoveAll(c.TempDir)
}

func (c *Config) setTempDir() error {
	if c.DryRun {
		return nil // no temp directory needed for dry-run
	}
	if d := c.WorkDir; d != "" {
		c.TempDir = d
		return nil
	}
	d, err := os.MkdirTemp(os.TempDir(), "cmdcomp")
	if err != nil {
		return err
	}
	c.TempDir = d
	return nil
}

func (c Config) GetLeftEnv() []string {
	return append(c.Env, c.LeftEnv...)
}

func (c Config) GetRightEnv() []string {
	return append(c.Env, c.RightEnv...)
}

func (Config) applyEnv(env, v []string) []string {
	d := map[string]string{}
	for _, x := range env {
		xs := strings.SplitN(x, "=", 2)
		switch len(xs) {
		case 2:
			d[xs[0]] = xs[1]
		case 1:
			d[xs[0]] = ""
		}
	}

	s := make([]string, len(v))
	for i, x := range v {
		s[i] = os.Expand(x, func(k string) string {
			if v, ok := d[k]; ok {
				return v
			}
			return fmt.Sprintf("$%s", k)
		})
	}
	slog.Debug("apply env", slog.Any("before", v), slog.Any("after", s))
	return s
}

func (c Config) applyLeftEnv(v []string) []string {
	return c.applyEnv(c.GetLeftEnv(), v)
}

func (c Config) applyRightEnv(v []string) []string {
	return c.applyEnv(c.GetRightEnv(), v)
}

func (c Config) GetLeftPreprocess() []string {
	return c.applyLeftEnv(append(c.Preprocess, c.LeftPreprocess...))
}

func (c Config) GetRightPreprocess() []string {
	return c.applyRightEnv(append(c.Preprocess, c.RightPreprocess...))
}

func (c Config) GetLeftArgs() []string {
	return c.applyLeftEnv(append(c.CommonArgs, c.LeftArgs...))
}

func (c Config) GetRightArgs() []string {
	return c.applyRightEnv(append(c.CommonArgs, c.RightArgs...))
}

func (c *Config) setArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: no args", ErrConfig)
	}
	before, after := slicex.Split(args, c.Delimiter)
	c.CommonArgs = before
	c.LeftArgs, c.RightArgs = slicex.Split(after, c.Delimiter)

	if len(c.GetLeftArgs()) == 0 {
		return fmt.Errorf("%w: no left args", ErrConfig)
	}
	if len(c.GetRightArgs()) == 0 {
		return fmt.Errorf("%w: no right args", ErrConfig)
	}
	return nil
}

func (c Config) SetupLogger(w io.Writer) {
	level := slog.LevelInfo
	if c.Debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
}
