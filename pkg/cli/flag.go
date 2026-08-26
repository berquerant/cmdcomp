package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/berquerant/cmdcomp/pkg/slicex"
	"github.com/berquerant/cmdcomp/version"
	"github.com/berquerant/structconfig"
	"github.com/spf13/pflag"
)

var (
	ErrExit = errors.New("Exit")
)

func ParseConfig(args []string, stdout, stderr io.Writer) (*Config, error) {
	fs := pflag.NewFlagSet("main", pflag.ContinueOnError)
	fs.SetOutput(stdout)
	fs.Usage = func() {
		var b UsageBuilder
		fmt.Fprint(stdout, b.Build())
		fmt.Fprintln(stdout, "```")
		fs.PrintDefaults()
		fmt.Fprintln(stdout, "```")
	}

	sc := structconfig.New[Config]()
	if err := sc.SetFlags(fs); err != nil {
		return nil, err
	}

	before, after := slicex.Split(args, "--")
	if len(before) > 0 {
		err := fs.Parse(before)
		if errors.Is(err, pflag.ErrHelp) {
			return nil, ErrExit
		}
		if err != nil {
			return nil, err
		}
	}

	var cliConfig Config
	if err := sc.FromFlags(&cliConfig, fs); err != nil {
		return nil, err
	}

	if cliConfig.Version {
		version.Write(stdout)
		return nil, ErrExit
	}

	var envConfig Config
	if err := sc.FromEnv(&envConfig, structconfig.WithEnvPrefix("CMDCOMP_")); err != nil {
		return nil, err
	}

	configPath := envConfig.ConfigPath
	if cliConfig.ConfigPath != "" {
		configPath = cliConfig.ConfigPath
	}

	presetNames := envConfig.PresetNames
	if len(cliConfig.PresetNames) > 0 {
		presetNames = cliConfig.PresetNames
	}

	cs, err := LoadConfigSet(configPath)
	if err != nil {
		return nil, err
	}

	merger := structconfig.NewMerger[Config]()
	baseConfig := newDefaultConfig()
	for _, p := range presetNames {
		x, ok := cs.Find(p)
		if !ok {
			return nil, fmt.Errorf("preset not found %s", p)
		}
		mergedPreset, err := merger.Merge(*baseConfig, *x)
		if err != nil {
			return nil, err
		}
		baseConfig = &mergedPreset
		slog.Debug("use preset", slog.String("preset", p))
	}

	// Layered configuration merge order:
	// Precedence: baseConfig (default or combined presets) < envConfig (CMDCOMP_*) < cliConfig (CLI flags)
	baseAndEnv, err := merger.Merge(*baseConfig, envConfig)
	if err != nil {
		return nil, err
	}
	merged, err := merger.Merge(baseAndEnv, cliConfig)
	if err != nil {
		return nil, err
	}
	c := &merged

	if c.Reader == nil {
		c.Reader = os.Stdin
	}
	c.Writer = stdout
	c.SetupLogger(stderr)
	slog.Debug("parse args", slog.Any("args", before))
	slog.Debug("init args", slog.Any("args", after))
	if err := c.Init(after); err != nil {
		return nil, err
	}

	cj, _ := json.Marshal(c)
	slog.Debug("config", slog.String("json", string(cj)))
	return c, nil
}
