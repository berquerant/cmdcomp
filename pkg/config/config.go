package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/berquerant/cmdcomp/pkg/slicex"
)

var (
	ErrConfig = errors.New("Config")
)

func NewConfig(
	w io.Writer,
	preprocess []string,
	diff, shell string,
) *Config {
	return &Config{
		Preprocess: preprocess,
		Diff:       diff,
		Shell:      shell,
		Writer:     w,
	}
}

type Config struct {
	Debug      bool
	Preprocess []string
	Diff       string
	WorkDir    string
	Shell      string

	CommonArgs []string
	LeftArgs   []string
	RightArgs  []string

	Writer  io.Writer `json:"-"`
	TempDir string
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
	if c.WorkDir == "" {
		return os.RemoveAll(c.TempDir)
	}
	return nil
}

func (c *Config) setTempDir() error {
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

func (c Config) GetLeftArgs() []string {
	return append(c.CommonArgs, c.LeftArgs...)
}

func (c Config) GetRightArgs() []string {
	return append(c.CommonArgs, c.RightArgs...)
}

func (c *Config) setArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: no args", ErrConfig)
	}
	before, after := slicex.Split(args, "--")
	c.CommonArgs = before
	c.LeftArgs, c.RightArgs = slicex.Split(after, "--")

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
