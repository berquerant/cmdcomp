package cli

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"time"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/goccy/go-yaml"
)

type Config = config.Config

func newDefaultConfig() *Config {
	return &Config{
		Shell:     "bash",
		Delimiter: "--",
		Diff:      "diff",
	}
}

type ConfigSet struct {
	Default *Config            `yaml:"default,omitempty"`
	Presets map[string]*Config `yaml:"presets"`
}

func (c *ConfigSet) Find(name string) (*Config, bool) {
	x, ok := c.Presets[name]
	if !ok {
		return nil, false
	}
	return x, true
}

func (c *ConfigSet) merge(x *ConfigSet) *ConfigSet {
	if c.Presets == nil {
		c.Presets = map[string]*Config{}
	}
	maps.Copy(c.Presets, x.Presets)
	if c.Default == nil {
		c.Default = x.Default
	}
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
		if p.Diff == "" {
			p.Diff = dc.Diff
		}
		if p.Delimiter == "" {
			p.Delimiter = dc.Delimiter
		}
		if p.Shell == "" {
			p.Shell = dc.Shell
		}
	}
}

func newConfigExample() *ConfigSet {
	return &ConfigSet{
		Default: &Config{
			Diff:  "diff -u",
			Shell: "bash",
		},
		Presets: map[string]*Config{
			"example": &Config{
				ShowCmdLog: true,
				Debug:      true,
				DryRun:     false,
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
				Stdin:          "-",
				LeftStdin:      "-",
				RightStdin:     "@data.txt",
				Snapshot:       "-",
				LeftSnapshot:   "@left.txt",
				RightSnapshot:  "@right.txt",
				CommonArgs: []string{
					"echo",
				},
				LeftArgs: []string{
					"common left",
				},
				RightArgs: []string{
					"common right",
				},
				Timeout:        time.Minute,
				ProcessTimeout: 10 * time.Second,
				Success:        true,
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
