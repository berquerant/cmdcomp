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
	ShowCmdLog      bool          `name:"show-cmd-log" usage:"print stdout and stderr of executed subcommands to log output" yaml:"show-cmd-log,omitempty"`
	Debug           bool          `name:"debug" usage:"enable debug log output" yaml:"debug,omitempty"`
	DryRun          bool          `name:"dry-run" usage:"print generated bash script capturing the full execution pipeline without executing commands" yaml:"dry-run,omitempty"`
	// Command list hooks: separated by newline (\n) in env vars to allow quotes/delimiters in scripts safely.
	Startup         []string      `name:"startup" short:"s" split:"true" sep:"\n" usage:"command(s) executed sequentially before running commands (e.g. repo updates). Can be specified multiple times. In env vars, separate commands with newlines" yaml:"startup,omitempty"`
	Interceptor     []string      `name:"interceptor" short:"i" split:"true" sep:"\n" usage:"command(s) executed sequentially after left command and before right command (e.g. git checkout). Can be specified multiple times. In env vars, separate commands with newlines" yaml:"interceptor,omitempty"`
	Preprocess      []string      `name:"preprocess" short:"p" split:"true" sep:"\n" usage:"filter pipeline command(s) applied to both left and right outputs before diffing. Reads stdin, writes stdout (e.g. jq, yq, sed). Multiple flags form a piped chain. In env vars, separate commands with newlines" yaml:"preprocess,omitempty"`
	LeftPreprocess  []string      `name:"left-preprocess" split:"true" sep:"\n" usage:"additional filter pipeline command(s) applied only to left output after common preprocess. Multiple flags form a piped chain. In env vars, separate commands with newlines" yaml:"left-preprocess,omitempty"`
	RightPreprocess []string      `name:"right-preprocess" split:"true" sep:"\n" usage:"additional filter pipeline command(s) applied only to right output after common preprocess. Multiple flags form a piped chain. In env vars, separate commands with newlines" yaml:"right-preprocess,omitempty"`
	Diff            string        `name:"diff" short:"x" default:"diff" usage:"diff command invoked as '<diff> LEFT_FILE RIGHT_FILE' (e.g. 'diff -u', 'colordiff', 'dyff', 'objdiff -c')" yaml:"diff,omitempty"`
	WorkDir         string        `name:"work-dir" short:"w" usage:"working directory for temporary output files. When specified, temporary files are preserved after execution" yaml:"work-dir,omitempty"`
	Shell           string        `name:"shell" short:"S" default:"bash" usage:"shell executable used to run subcommands" yaml:"shell,omitempty"`
	Delimiter       string        `name:"delimiter" short:"d" default:"--" usage:"delimiter token separating [COMMON_ARGS], [LEFT_ARGS], and [RIGHT_ARGS] (e.g. '---')" yaml:"delimiter,omitempty"`
	UseLabel        bool          `name:"label" short:"l" usage:"pass '--label LEFT_ARG' and '--label RIGHT_ARG' to the diff command (useful for diff/colordiff)" yaml:"label,omitempty"`
	// Environment variable pairs (KEY=VALUE): separated by comma (,) in env vars.
	Env             []string      `name:"env" short:"e" split:"true" sep:"," usage:"environment variables passed to all subcommands along with system environment (KEY=VALUE). Can be specified multiple times or comma-separated" yaml:"env,omitempty"`
	LeftEnv         []string      `name:"left-env" split:"true" sep:"," usage:"environment variables passed only to left command and left preprocess (KEY=VALUE). Can be specified multiple times or comma-separated" yaml:"left-env,omitempty"`
	RightEnv        []string      `name:"right-env" split:"true" sep:"," usage:"environment variables passed only to right command and right preprocess (KEY=VALUE). Can be specified multiple times or comma-separated" yaml:"right-env,omitempty"`
	Cleanup         []string      `name:"cleanup" short:"c" split:"true" sep:"\n" usage:"command(s) guaranteed to execute before cmdcomp exits, even on failure. Can be specified multiple times. In env vars, separate commands with newlines" yaml:"cleanup,omitempty"`
	Stdin           string        `name:"stdin" usage:"pass input to stdin of both left and right commands ('-' for stdin, '@filename' for file)" yaml:"stdin,omitempty"`
	LeftStdin       string        `name:"left-stdin" usage:"pass input to stdin of left command only ('-' for stdin, '@filename' for file)" yaml:"left-stdin,omitempty"`
	RightStdin      string        `name:"right-stdin" usage:"pass input to stdin of right command only ('-' for stdin, '@filename' for file)" yaml:"right-stdin,omitempty"`

	CommonArgs []string `name:"-" yaml:"common-args,omitempty"`
	LeftArgs   []string `name:"-" yaml:"left-args,omitempty"`
	RightArgs  []string `name:"-" yaml:"right-args,omitempty"`

	Reader         io.Reader     `name:"-" json:"-" yaml:"-"`
	Writer         io.Writer     `name:"-" json:"-" yaml:"-"`
	TempDir        string        `name:"-" json:"-" yaml:"-"`
	Timeout        time.Duration `name:"timeout" usage:"maximum timeout for entire cmdcomp execution (e.g. '30s', '2m')" yaml:"timeout,omitempty"`
	ProcessTimeout time.Duration `name:"process-timeout" usage:"maximum timeout for each individual subcommand execution (e.g. '10s', '1m')" yaml:"process-timeout,omitempty"`

	Success    bool   `name:"success" usage:"exit 0 when diffs are detected (exit status 1 from diff command). Failures (exit code 2) still return 2" yaml:"success,omitempty"`
	ConfigPath string `name:"config" usage:"configuration file path (default search order: UserConfigDir/cmdcomp/config.yml, $HOME/.cmdcomp.yml, .cmdcomp.yml)" yaml:"-"`
	PresetName string `name:"preset" usage:"name of preset configuration to load from config file or built-in presets (e.g. 'json', 'yml', 'helm', 'k8s', 'dyff')" yaml:"-"`
	Version    bool   `name:"version" usage:"display version and exit" yaml:"-"`
}

func (c *Config) Init(args []string) error {
	if err := c.validateStdin(); err != nil {
		return err
	}
	if err := c.setTempDir(); err != nil {
		return err
	}
	if err := c.setArgs(args); err != nil {
		return err
	}
	return nil
}

func (c *Config) validateStdin() error {
	for name, val := range map[string]string{
		"stdin":       c.Stdin,
		"left-stdin":  c.LeftStdin,
		"right-stdin": c.RightStdin,
	} {
		if val == "" || val == "-" || strings.HasPrefix(val, "@") {
			continue
		}
		return fmt.Errorf("%w: invalid %s '%s': must be '-' or '@filename'", ErrConfig, name, val)
	}
	return nil
}

func (c Config) GetLeftStdin() string {
	if c.LeftStdin != "" {
		return c.LeftStdin
	}
	return c.Stdin
}

func (c Config) GetRightStdin() string {
	if c.RightStdin != "" {
		return c.RightStdin
	}
	return c.Stdin
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
