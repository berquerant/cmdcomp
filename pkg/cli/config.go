package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/goccy/go-yaml"
)

type Config struct {
	config.Config
	Success bool `yaml:"success"`
}

func newDefaultConfig() *Config {
	return &Config{
		Config: config.Config{
			Shell:     "bash",
			Delimiter: "--",
			Diff:      "diff",
		},
	}
}

type ConfigSet struct {
	Presets map[string]*Config `yaml:"presets"`
}

func (c *ConfigSet) Find(name string) (*Config, bool) {
	x, ok := c.Presets[name]
	return x, ok
}

func LoadConfigSet(path string) (*ConfigSet, error) {
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
	for _, c := range cs.Presets {
		if c.Diff == "" {
			c.Diff = dc.Diff
		}
		if c.Delimiter == "" {
			c.Delimiter = dc.Delimiter
		}
		if c.Shell == "" {
			c.Shell = dc.Shell
		}
	}
}

func newConfigExample() *ConfigSet {
	return &ConfigSet{
		Presets: map[string]*Config{
			"example": &Config{
				Success: true,
				Config: config.Config{
					ShowCmdLog: true,
					Debug:      true,
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
