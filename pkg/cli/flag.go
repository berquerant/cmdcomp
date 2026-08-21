package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

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
	fs.Usage = func() {
		var b UsageBuilder
		fmt.Fprint(stderr, b.Build())
		fs.PrintDefaults()
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

	cs, err := LoadConfigSet(cliConfig.ConfigPath)
	if err != nil {
		return nil, err
	}

	var baseConfig *Config
	if p := cliConfig.PresetName; p != "" {
		x, ok := cs.Find(p)
		if !ok {
			return nil, fmt.Errorf("preset not found %s", p)
		}
		baseConfig = x
		slog.Debug("use preset", slog.String("preset", p))
	} else {
		baseConfig = newDefaultConfig()
	}

	var envConfig Config
	if err := sc.FromEnv(&envConfig, structconfig.WithEnvPrefix("CMDCOMP_")); err != nil {
		return nil, err
	}
	parseEnvStringSlices(&envConfig, "CMDCOMP_")

	merger := structconfig.NewMerger[Config]()
	baseAndEnv, err := merger.Merge(*baseConfig, envConfig)
	if err != nil {
		return nil, err
	}
	merged, err := merger.Merge(baseAndEnv, cliConfig)
	if err != nil {
		return nil, err
	}
	c := &merged
	c.ConfigPath = ""
	c.PresetName = ""
	c.Version = false

	c.Writer = stdout
	c.SetupLogger(stderr)
	slog.Debug("parse args", slog.Any("args", before))
	if len(after) > 0 {
		slog.Debug("init args", slog.Any("args", after))
		if err := c.Init(after); err != nil {
			return nil, err
		}
	}

	cj, _ := json.Marshal(c)
	slog.Debug("config", slog.String("json", string(cj)))
	return c, nil
}

func parseEnvStringSlices(c *Config, prefix string) {
	fields := []struct {
		envName string
		target  *[]string
	}{
		{"STARTUP", &c.Startup},
		{"INTERCEPTOR", &c.Interceptor},
		{"PREPROCESS", &c.Preprocess},
		{"LEFT_PREPROCESS", &c.LeftPreprocess},
		{"RIGHT_PREPROCESS", &c.RightPreprocess},
		{"ENV", &c.Env},
		{"LEFT_ENV", &c.LeftEnv},
		{"RIGHT_ENV", &c.RightEnv},
		{"CLEANUP", &c.Cleanup},
	}

	for _, f := range fields {
		v, ok := os.LookupEnv(prefix + f.envName)
		if !ok || v == "" {
			continue
		}
		*f.target = splitEnvSlice(v)
	}
}

func splitEnvSlice(s string) []string {
	// If newline is present, split by newline
	if strings.Contains(s, "\n") {
		var res []string
		for line := range strings.SplitSeq(s, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				res = append(res, line)
			}
		}
		return res
	}
	// Otherwise split by comma
	var res []string
	for item := range strings.SplitSeq(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			res = append(res, item)
		}
	}
	return res
}
