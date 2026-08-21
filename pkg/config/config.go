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
	ShowCmdLog      bool     `yaml:"showCmdLog,omitempty"`
	Debug           bool     `yaml:"debug,omitempty"`
	DryRun          bool     `yaml:"dryrun,omitempty"`
	Startup         []string `yaml:"startup,omitempty"`
	Interceptor     []string `yaml:"interceptor,omitempty"`
	Preprocess      []string `yaml:"preprocess,omitempty"`
	LeftPreprocess  []string `yaml:"leftPreprocess,omitempty"`
	RightPreprocess []string `yaml:"rightPreprocess,omitempty"`
	Diff            string   `yaml:"diff,omitempty"`
	WorkDir         string   `yaml:"workDir,omitempty"`
	Shell           string   `yaml:"shell,omitempty"`
	Delimiter       string   `yaml:"delimiter,omitempty"`
	UseLabel        bool     `yaml:"label,omitempty"`
	Env             []string `yaml:"env,omitempty"`
	LeftEnv         []string `yaml:"leftEnv,omitempty"`
	RightEnv        []string `yaml:"rightEnv,omitempty"`
	Cleanup         []string `yaml:"cleanup,omitempty"`

	CommonArgs []string `yaml:"commonArgs,omitempty"`
	LeftArgs   []string `yaml:"leftArgs,omitempty"`
	RightArgs  []string `yaml:"rightArgs,omitempty"`

	Writer         io.Writer     `json:"-" yaml:"-"`
	TempDir        string        `json:"-" yaml:"-"`
	Timeout        time.Duration `yaml:"timeout,omitempty"`
	ProcessTimeout time.Duration `yaml:"processTimeout,omitempty"`
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
