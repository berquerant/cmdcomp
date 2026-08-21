package cli

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"time"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/goccy/go-yaml"
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

	Success    bool   `name:"success" usage:"exit successfully even if there are diffs;\nin other words, succeed even if the diff command returns exit status 1" yaml:"success,omitempty"`
	ConfigPath string `name:"config" usage:"config file path; default: UserConfigDir/cmdcomp/config.yml or $HOME/.cmdcomp.yml or .cmdcomp.yml; see https://pkg.go.dev/os#UserConfigDir" yaml:"-"`
	PresetName string `name:"preset" usage:"name of preset to be used" yaml:"-"`
	Version    bool   `name:"version" usage:"display version" yaml:"-"`
}

func (c *Config) AsConfig() *config.Config {
	return &config.Config{
		ShowCmdLog:      c.ShowCmdLog,
		Debug:           c.Debug,
		DryRun:          c.DryRun,
		Startup:         c.Startup,
		Interceptor:     c.Interceptor,
		Preprocess:      c.Preprocess,
		LeftPreprocess:  c.LeftPreprocess,
		RightPreprocess: c.RightPreprocess,
		Diff:            c.Diff,
		WorkDir:         c.WorkDir,
		Shell:           c.Shell,
		Delimiter:       c.Delimiter,
		UseLabel:        c.UseLabel,
		Env:             c.Env,
		LeftEnv:         c.LeftEnv,
		RightEnv:        c.RightEnv,
		Cleanup:         c.Cleanup,
		CommonArgs:      c.CommonArgs,
		LeftArgs:        c.LeftArgs,
		RightArgs:       c.RightArgs,
		Writer:          c.Writer,
		TempDir:         c.TempDir,
		Timeout:         c.Timeout,
		ProcessTimeout:  c.ProcessTimeout,
	}
}

func (c *Config) Init(args []string) error {
	cfg := c.AsConfig()
	if err := cfg.Init(args); err != nil {
		return err
	}
	c.CommonArgs = cfg.CommonArgs
	c.LeftArgs = cfg.LeftArgs
	c.RightArgs = cfg.RightArgs
	c.TempDir = cfg.TempDir
	return nil
}

func (c *Config) SetupLogger(w io.Writer) {
	c.AsConfig().SetupLogger(w)
}

func (c *Config) Close() error {
	return c.AsConfig().Close()
}

func newDefaultConfig() *Config {
	return &Config{
		Shell:     "bash",
		Delimiter: "--",
		Diff:      "diff",
	}
}

type Preset struct {
	Config  config.Config `yaml:"config"`
	Success bool          `yaml:"success,omitempty"`
}

func (p *Preset) ToConfig() *Config {
	c := &Config{
		Success: p.Success,
	}
	c.FromConfig(&p.Config)
	return c
}

func (c *Config) FromConfig(cfg *config.Config) {
	c.ShowCmdLog = cfg.ShowCmdLog
	c.Debug = cfg.Debug
	c.DryRun = cfg.DryRun
	c.Startup = cfg.Startup
	c.Interceptor = cfg.Interceptor
	c.Preprocess = cfg.Preprocess
	c.LeftPreprocess = cfg.LeftPreprocess
	c.RightPreprocess = cfg.RightPreprocess
	c.Diff = cfg.Diff
	c.WorkDir = cfg.WorkDir
	c.Shell = cfg.Shell
	c.Delimiter = cfg.Delimiter
	c.UseLabel = cfg.UseLabel
	c.Env = cfg.Env
	c.LeftEnv = cfg.LeftEnv
	c.RightEnv = cfg.RightEnv
	c.Cleanup = cfg.Cleanup
	c.CommonArgs = cfg.CommonArgs
	c.LeftArgs = cfg.LeftArgs
	c.RightArgs = cfg.RightArgs
	c.Writer = cfg.Writer
	c.TempDir = cfg.TempDir
	c.Timeout = cfg.Timeout
	c.ProcessTimeout = cfg.ProcessTimeout
}

type ConfigSet struct {
	Presets map[string]*Preset `yaml:"presets"`
}

func (c *ConfigSet) Find(name string) (*Config, bool) {
	x, ok := c.Presets[name]
	if !ok {
		return nil, false
	}
	return x.ToConfig(), true
}

func (c *ConfigSet) merge(x *ConfigSet) *ConfigSet {
	if c.Presets == nil {
		c.Presets = map[string]*Preset{}
	}
	maps.Copy(c.Presets, x.Presets)
	return c
}

func LoadConfigSet(path string) (*ConfigSet, error) {
	c, err := loadConfigSetOrDefault(path)
	if err != nil {
		return nil, err
	}
	return c.merge(builtinConfigSet()), nil
}

func loadConfigSetOrDefault(path string) (*ConfigSet, error) {
	if path == "" {
		return loadDefaultConfigSet(), nil
	}
	c, err := loadConfigSet(path)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to load config file %s", err, path)
	}
	return c, nil
}

func loadDefaultConfigSet() *ConfigSet {
	for _, x := range defaultConfigPaths() {
		if _, err := os.Stat(x); err != nil {
			continue
		}
		if c, err := loadConfigSet(x); err == nil {
			return c
		}
	}
	var c ConfigSet
	return &c
}

func loadConfigSet(path string) (*ConfigSet, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var c ConfigSet
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	applyDefaultValuesToConfigSet(&c)
	return &c, nil
}

func applyDefaultValuesToConfigSet(cs *ConfigSet) {
	dc := newDefaultConfig()
	for _, p := range cs.Presets {
		if p.Config.Diff == "" {
			p.Config.Diff = dc.Diff
		}
		if p.Config.Delimiter == "" {
			p.Config.Delimiter = dc.Delimiter
		}
		if p.Config.Shell == "" {
			p.Config.Shell = dc.Shell
		}
	}
}

func newConfigExample() *ConfigSet {
	return &ConfigSet{
		Presets: map[string]*Preset{
			"example": &Preset{
				Success: true,
				Config: config.Config{
					ShowCmdLog: true,
					Debug:      true,
					Startup: []string{
						"echo startup",
					},
					Interceptor: []string{
						"echo interceptor",
					},
					Preprocess: []string{
						"grep common",
					},
					LeftPreprocess: []string{
						"grep left",
					},
					RightPreprocess: []string{
						"grep right",
					},
					Diff:      "diff",
					WorkDir:   "workdir",
					Shell:     "bash",
					Delimiter: "--",
					UseLabel:  true,
					Env: []string{
						"X=1",
					},
					LeftEnv: []string{
						"Y=2",
					},
					RightEnv: []string{
						"Y=3",
					},
					Cleanup: []string{
						"echo cleanup",
					},
					CommonArgs: []string{
						"echo",
					},
					LeftArgs: []string{
						"common left",
					},
					RightArgs: []string{
						"common right",
					},
				},
			},
		},
	}
}

func defaultConfigPaths() []string {
	xs := []string{}
	if x, err := os.UserConfigDir(); err == nil {
		xs = append(xs, filepath.Join(x, "cmdcomp", "config.yml"))
	}
	if x, err := os.UserHomeDir(); err == nil {
		xs = append(xs, filepath.Join(x, ".cmdcomp.yml"))
	}
	return append(xs, ".cmdcomp.yml")
}
