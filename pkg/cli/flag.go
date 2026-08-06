package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/berquerant/cmdcomp/pkg/slicex"
	"github.com/berquerant/cmdcomp/version"
	"github.com/spf13/pflag"
)

var (
	ErrExit = errors.New("Exit")
)

func ParseConfig(args []string, stdout, stderr io.Writer) (*Config, error) {
	fs := pflag.NewFlagSet("main", pflag.ContinueOnError)
	fs.Usage = func() {
		var b usageBuilder
		fmt.Fprint(stderr, b.build())
		fs.PrintDefaults()
	}

	var (
		displayVersion = fs.Bool("version", false, "display version")
		debug          = fs.Bool("debug", false, "enable debug logs")
		showCmdLog     = fs.Bool("showCmdLog", false, "show command logs")
		workDir        = fs.StringP("workDir", "w", "", "working directory; keep temporary files")
		shell          = fs.StringP("shell", "S", "bash", "shell command to be executed")
		delimiter      = fs.StringP("delimiter", "d", "--", `arguments delimiter;
change the '--' separating COMMON_ARGS, LEFT_ARGS, and RIGHT_ARGS in this`)
		success = fs.Bool("success", false, `exit successfully even if there are diffs;
in other words, succeed even if the diff command returns exit status 1`)
		useLabel   = fs.BoolP("label", "l", false, "use '--label' option of diff command")
		configPath = fs.String("config", "",
			"config file path; default: UserConfigDir/cmdcomp/config.yml or $HOME/.cmdcomp.yml or .cmdcomp.yml; see https://pkg.go.dev/os#UserConfigDir")
		presetName                                  = fs.String("preset", "", "name of preset to be used")
		startup, interceptor, cleanup               []string
		preprocess, leftPreprocess, rightPreprocess []string
		env, leftEnv, rightEnv                      []string
		diff                                        string
	)
	// workaround: https://github.com/spf13/pflag/issues/370
	fs.StringArrayVarP(&startup, "startup", "s", nil,
		"process before running commands; invoked like 'startup'")
	fs.StringArrayVarP(&interceptor, "interceptor", "i", nil,
		"process after left command and before right command; invoked like 'interceptor'",
	)
	fs.StringArrayVarP(&cleanup, "cleanup", "c", nil,
		"process before exiting cmdcomp process; invoked like 'cleanup'")
	fs.StringArrayVarP(&preprocess, "preprocess", "p", nil,
		"process before diff; invoked like 'preprocess'; should read input from stdin; should output result to stdout",
	)
	fs.StringArrayVar(&leftPreprocess, "leftPreprocess", nil,
		"additional left process before diff; invoked like 'leftPreprocess'; should read input from stdin; should output result to stdout",
	)
	fs.StringArrayVar(&rightPreprocess, "rightPreprocess", nil,
		"additional right process before diff; invoked like 'rightPreprocess'; should read input from stdin; should output result to stdout",
	)
	fs.StringArrayVarP(&env, "env", "e", nil,
		`process environment variables;
Passed to all processes along with os.Environ.
--leftEnv is also passed to left output and left preprocess.
--rightEnv is also passed to right output and right preprocess.`,
	)
	fs.StringArrayVar(&leftEnv, "leftEnv", nil, "left process environment variables")
	fs.StringArrayVar(&rightEnv, "rightEnv", nil, "right process environment variables")
	fs.StringVarP(&diff, "diff", "x", "diff",
		"diff command; invoked like 'diff LEFT_FILE RIGHT_FILE'",
	)

	before, after := slicex.Split(args, "--")
	err := fs.Parse(before)
	if errors.Is(err, pflag.ErrHelp) {
		return nil, ErrExit
	}
	if err != nil {
		return nil, err
	}
	if *displayVersion {
		version.Write(stdout)
		return nil, ErrExit
	}

	cs, err := LoadConfigSet(*configPath)
	if err != nil {
		return nil, err
	}

	var c *Config
	if p := *presetName; p != "" {
		x, ok := cs.Find(p)
		if !ok {
			return nil, fmt.Errorf("preset not found %s", p)
		}
		c = x
		slog.Debug("use preset", slog.String("preset", p))
	} else {
		c = newDefaultConfig()
	}

	c.Writer = stdout
	fs.Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "debug":
			c.Debug = *debug
		case "showCmdLog":
			c.ShowCmdLog = *showCmdLog
		case "workDir":
			c.WorkDir = *workDir
		case "shell":
			c.Shell = *shell
		case "delimiter":
			c.Delimiter = *delimiter
		case "success":
			c.Success = *success
		case "label":
			c.UseLabel = *useLabel
		case "startup":
			c.Startup = startup
		case "interceptor":
			c.Interceptor = interceptor
		case "cleanup":
			c.Cleanup = cleanup
		case "preprocess":
			c.Preprocess = preprocess
		case "leftPreprocess":
			c.LeftPreprocess = leftPreprocess
		case "rightPreprocess":
			c.RightPreprocess = rightPreprocess
		case "env":
			c.Env = env
		case "leftEnv":
			c.LeftEnv = leftEnv
		case "rightEnv":
			c.RightEnv = rightEnv
		case "diff":
			c.Diff = diff
		}
	})

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
