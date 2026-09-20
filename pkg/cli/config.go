package cli

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"time"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/berquerant/structconfig"
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

// ConfigMerger applies defaults, presets, env configs, and CLI flags in precedence order.
type ConfigMerger struct {
	merger *structconfig.Merger[Config]
	base   Config
}

func NewConfigMerger() *ConfigMerger {
	return &ConfigMerger{
		merger: structconfig.NewMerger[Config](),
		base:   *newDefaultConfig(),
	}
}

func (m *ConfigMerger) ApplyDefault(cs *ConfigSet, noDefault bool) error {
	if noDefault || cs.Default == nil {
		return nil
	}
	merged, err := m.merger.Merge(m.base, *cs.Default)
	if err != nil {
		return err
	}
	m.base = merged
	return nil
}

func (m *ConfigMerger) ApplyPresets(cs *ConfigSet, presetNames []string) error {
	for _, p := range presetNames {
		x, ok := cs.Find(p)
		if !ok {
			return fmt.Errorf("preset not found %s", p)
		}
		merged, err := m.merger.Merge(m.base, *x)
		if err != nil {
			return err
		}
		m.base = merged
	}
	return nil
}

func (m *ConfigMerger) MergeLayers(envConfig, cliConfig Config) (*Config, error) {
	baseAndEnv, err := m.merger.Merge(m.base, envConfig)
	if err != nil {
		return nil, err
	}
	merged, err := m.merger.Merge(baseAndEnv, cliConfig)
	if err != nil {
		return nil, err
	}
	return &merged, nil
}

func (m *ConfigMerger) Base() Config {
	return m.base
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
