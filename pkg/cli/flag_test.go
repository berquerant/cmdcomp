package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/berquerant/cmdcomp/pkg/cli"
	"github.com/stretchr/testify/assert"
)

func TestParseConfig(t *testing.T) {
	const base = `presets:
  base:
    preprocess:
      - grep base`

	configPath := filepath.Join(t.TempDir(), "base.yml")
	if !assert.Nil(t, os.WriteFile(configPath, []byte(base), 0644)) {
		return
	}

	for _, tc := range []struct {
		name   string
		args   []string
		want   *cli.Config
		errMsg string
	}{
		{
			name: "default",
			args: []string{
				"--", "echo", "x",
			},
			want: &cli.Config{
				Diff:      "diff",
				Delimiter: "--",
				Shell:     "bash",
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "use base",
			args: []string{
				"--config", configPath,
				"--preset", "base",
				"--", "echo", "x",
			},
			want: &cli.Config{
				ConfigPath:  configPath,
				PresetNames: []string{"base"},
				Diff:        "diff",
				Delimiter:   "--",
				Shell:       "bash",
				Preprocess: []string{
					"grep base",
				},
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "use u preset",
			args: []string{
				"--preset", "u",
				"--", "echo", "x",
			},
			want: &cli.Config{
				PresetNames: []string{"u"},
				Diff:        "diff -u",
				Delimiter:   "--",
				Shell:       "bash",
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "use uc preset",
			args: []string{
				"--preset", "uc",
				"--", "echo", "x",
			},
			want: &cli.Config{
				PresetNames: []string{"uc"},
				Diff:        "diff -u --color",
				Delimiter:   "--",
				Shell:       "bash",
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "use preset short flag -P",
			args: []string{
				"-P", "u",
				"--", "echo", "x",
			},
			want: &cli.Config{
				PresetNames: []string{"u"},
				Diff:        "diff -u",
				Delimiter:   "--",
				Shell:       "bash",
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "use multiple presets composed in order",
			args: []string{
				"--config", configPath,
				"-P", "base",
				"-P", "u",
				"--", "echo", "x",
			},
			want: &cli.Config{
				ConfigPath:  configPath,
				PresetNames: []string{"base", "u"},
				Diff:        "diff -u",
				Delimiter:   "--",
				Shell:       "bash",
				Preprocess: []string{
					"grep base",
				},
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "use comma-separated presets",
			args: []string{
				"--config", configPath,
				"--preset", "base,uc",
				"--", "echo", "x",
			},
			want: &cli.Config{
				ConfigPath:  configPath,
				PresetNames: []string{"base", "uc"},
				Diff:        "diff -u --color",
				Delimiter:   "--",
				Shell:       "bash",
				Preprocess: []string{
					"grep base",
				},
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "startup",
			args: []string{
				"-s", "echo s1",
				"--startup", "echo s2",
				"--", "echo", "x",
			},
			want: &cli.Config{
				Diff:      "diff",
				Delimiter: "--",
				Shell:     "bash",
				Startup: []string{
					"echo s1",
					"echo s2",
				},
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "cleanup",
			args: []string{
				"-c", "echo c1",
				"--cleanup", "echo c2",
				"--", "echo", "x",
			},
			want: &cli.Config{
				Diff:      "diff",
				Delimiter: "--",
				Shell:     "bash",
				Cleanup: []string{
					"echo c1",
					"echo c2",
				},
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "dry-run short flag",
			args: []string{
				"-n",
				"--", "echo", "x",
			},
			want: &cli.Config{
				DryRun:    true,
				Diff:      "diff",
				Delimiter: "--",
				Shell:     "bash",
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "stdin short flag",
			args: []string{
				"-I", "-",
				"--", "echo", "x",
			},
			want: &cli.Config{
				Stdin:     "-",
				Diff:      "diff",
				Delimiter: "--",
				Shell:     "bash",
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "snapshot short flag",
			args: []string{
				"-S", "@snap.txt",
				"--", "echo", "x",
			},
			want: &cli.Config{
				Snapshot:  "@snap.txt",
				Diff:      "diff",
				Delimiter: "--",
				Shell:     "bash",
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "left and right short flags -L, -R, -E, -F, -J, -K, -T, -U",
			args: []string{
				"-L", "grep left1",
				"-L", "grep left2",
				"-R", "grep right1",
				"-E", "LK1=LV1",
				"-F", "RK1=RV1",
				"-J", "-",
				"-K", "@rstdin.txt",
				"--", "echo", "x",
			},
			want: &cli.Config{
				Diff:            "diff",
				Delimiter:       "--",
				Shell:           "bash",
				LeftPreprocess:  []string{"grep left1", "grep left2"},
				RightPreprocess: []string{"grep right1"},
				LeftEnv:         []string{"LK1=LV1"},
				RightEnv:        []string{"RK1=RV1"},
				LeftStdin:       "-",
				RightStdin:      "@rstdin.txt",
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "left and right snapshot short flags -T, -U",
			args: []string{
				"-T", "@left.txt",
				"-U", "@right.txt",
			},
			want: &cli.Config{
				Diff:          "diff",
				Delimiter:     "--",
				Shell:         "bash",
				LeftSnapshot:  "@left.txt",
				RightSnapshot: "@right.txt",
			},
		},
		{
			name: "config short flag",
			args: []string{
				"-C", configPath,
				"--preset", "base",
				"--", "echo", "x",
			},
			want: &cli.Config{
				ConfigPath:  configPath,
				PresetNames: []string{"base"},
				Diff:        "diff",
				Delimiter:   "--",
				Shell:       "bash",
				Preprocess: []string{
					"grep base",
				},
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "timeout and processTimeout",
			args: []string{
				"--timeout", "1m",
				"--process-timeout", "5s",
				"--", "echo", "x",
			},
			want: &cli.Config{
				Diff:           "diff",
				Delimiter:      "--",
				Shell:          "bash",
				Timeout:        time.Minute,
				ProcessTimeout: 5 * time.Second,
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "no args and no snapshots returns error",
			args: []string{
				"--diff", "diff -u",
			},
			errMsg: "no args",
		},
		{
			name: "invalid stdin without -- returns error",
			args: []string{
				"--stdin", "invalid-stdin",
			},
			errMsg: "invalid stdin 'invalid-stdin'",
		},
		{
			name: "both snapshots without -- succeeds and sets tempdir",
			args: []string{
				"--left-snapshot", "@left.txt",
				"--right-snapshot", "@right.txt",
			},
			want: &cli.Config{
				Diff:          "diff",
				Delimiter:     "--",
				Shell:         "bash",
				LeftSnapshot:  "@left.txt",
				RightSnapshot: "@right.txt",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cli.ParseConfig(tc.args, os.Stdout, os.Stderr)
			if m := tc.errMsg; m != "" {
				assert.ErrorContains(t, err, m)
				return
			}
			assert.Nil(t, err)
			tc.want.Writer = nil
			got.Writer = nil
			tc.want.Reader = nil
			got.Reader = nil
			tc.want.TempDir = ""
			got.TempDir = ""
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseConfig_Env(t *testing.T) {
	t.Setenv("CMDCOMP_DIFF", "custom-diff")
	t.Setenv("CMDCOMP_SHELL", "zsh")
	t.Setenv("CMDCOMP_STARTUP", "echo s1\necho s2")
	t.Setenv("CMDCOMP_ENV", "K1=V1, K2=V2")
	t.Setenv("CMDCOMP_TIMEOUT", "10s")
	t.Setenv("CMDCOMP_PROCESS_TIMEOUT", "2s")

	got, err := cli.ParseConfig([]string{"--", "echo", "x"}, os.Stdout, os.Stderr)
	assert.Nil(t, err)
	want := &cli.Config{
		Diff:           "custom-diff",
		Shell:          "zsh",
		Delimiter:      "--",
		Startup:        []string{"echo s1", "echo s2"},
		Env:            []string{"K1=V1", "K2=V2"},
		Timeout:        10 * time.Second,
		ProcessTimeout: 2 * time.Second,
		CommonArgs:     []string{"echo", "x"},
	}
	got.Writer = nil
	got.Reader = nil
	got.TempDir = ""
	assert.Equal(t, want, got)

	// Test CLI flag overrides environment variable
	for _, tc := range []struct {
		title      string
		args       []string
		wantDiff   string
		wantShell  string
		wantStdin  string
		wantLeft   string
		wantRight  string
		wantErrMsg string
	}{
		{
			title:     "flag overrides env",
			args:      []string{"--diff", "flag-diff", "--shell", "sh", "--", "echo", "x"},
			wantDiff:  "flag-diff",
			wantShell: "sh",
		},
		{
			title:     "stdin '-' is valid",
			args:      []string{"--stdin", "-", "--", "echo", "x"},
			wantStdin: "-",
			wantLeft:  "-",
			wantRight: "-",
		},
		{
			title:     "stdin '@file' is valid",
			args:      []string{"--stdin", "@data.txt", "--", "echo", "x"},
			wantStdin: "@data.txt",
			wantLeft:  "@data.txt",
			wantRight: "@data.txt",
		},
		{
			title:     "left-stdin and right-stdin",
			args:      []string{"--left-stdin", "-", "--right-stdin", "@data.txt", "--", "echo", "x"},
			wantLeft:  "-",
			wantRight: "@data.txt",
		},
		{
			title:      "raw stdin without '@' is invalid",
			args:       []string{"--stdin", "data.txt", "--", "echo", "x"},
			wantErrMsg: "invalid stdin 'data.txt': must be '-' or '@filename'",
		},
		{
			title:      "raw left-stdin without '@' is invalid",
			args:       []string{"--left-stdin", "data.txt", "--", "echo", "x"},
			wantErrMsg: "invalid left-stdin 'data.txt': must be '-' or '@filename'",
		},
		{
			title:      "raw right-stdin without '@' is invalid",
			args:       []string{"--right-stdin", "data.txt", "--", "echo", "x"},
			wantErrMsg: "invalid right-stdin 'data.txt': must be '-' or '@filename'",
		},
		{
			title:     "snapshot '-' is valid",
			args:      []string{"--snapshot", "-", "--", "echo", "x"},
			wantStdin: "",
		},
		{
			title:     "snapshot '@file' is valid",
			args:      []string{"--snapshot", "@data.txt", "--", "echo", "x"},
			wantStdin: "",
		},
		{
			title:     "left-snapshot and right-snapshot",
			args:      []string{"--left-snapshot", "-", "--right-snapshot", "@data.txt", "--", "echo", "x"},
			wantStdin: "",
		},
		{
			title:      "raw snapshot without '@' is invalid",
			args:       []string{"--snapshot", "data.txt", "--", "echo", "x"},
			wantErrMsg: "invalid snapshot 'data.txt': must be '-' or '@filename'",
		},
		{
			title:      "raw left-snapshot without '@' is invalid",
			args:       []string{"--left-snapshot", "data.txt", "--", "echo", "x"},
			wantErrMsg: "invalid left-snapshot 'data.txt': must be '-' or '@filename'",
		},
		{
			title:      "raw right-snapshot without '@' is invalid",
			args:       []string{"--right-snapshot", "data.txt", "--", "echo", "x"},
			wantErrMsg: "invalid right-snapshot 'data.txt': must be '-' or '@filename'",
		},
		{
			title:      "stdin and snapshot exclusivity error for left side",
			args:       []string{"--left-stdin", "-", "--left-snapshot", "@data.txt", "--", "echo", "x"},
			wantErrMsg: "stdin and snapshot cannot be used together for left command",
		},
		{
			title:      "stdin and snapshot exclusivity error for right side",
			args:       []string{"--stdin", "-", "--right-snapshot", "@data.txt", "--", "echo", "x"},
			wantErrMsg: "stdin and snapshot cannot be used together for right command",
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			got, err := cli.ParseConfig(tc.args, os.Stdout, os.Stderr)
			if tc.wantErrMsg != "" {
				assert.ErrorContains(t, err, tc.wantErrMsg)
				return
			}
			assert.Nil(t, err)
			if tc.wantDiff != "" {
				assert.Equal(t, tc.wantDiff, got.Diff)
			}
			if tc.wantShell != "" {
				assert.Equal(t, tc.wantShell, got.Shell)
			}
			if tc.wantStdin != "" {
				assert.Equal(t, tc.wantStdin, got.Stdin)
			}
			if tc.wantLeft != "" {
				assert.Equal(t, tc.wantLeft, got.GetLeftStdin())
			}
			if tc.wantRight != "" {
				assert.Equal(t, tc.wantRight, got.GetRightStdin())
			}
		})
	}
}

func TestParseConfig_EnvPresetAndConfig(t *testing.T) {
	const customYml = `presets:
  custom:
    diff: custom-yml-diff
    shell: zsh
    preprocess:
      - grep custom
    env:
      - K1=V1
  other:
    diff: other-yml-diff
    preprocess:
      - grep other
    env:
      - K2=V2
`
	configPath := filepath.Join(t.TempDir(), "custom.yml")
	if !assert.Nil(t, os.WriteFile(configPath, []byte(customYml), 0644)) {
		return
	}

	for _, tc := range []struct {
		name           string
		env            map[string]string
		args           []string
		wantDiff       string
		wantShell      string
		wantConfig     string
		wantPresets    []string
		wantPreprocess []string
		wantEnv        []string
		wantErrMsg     string
	}{
		{
			name: "env preset builtin",
			env: map[string]string{
				"CMDCOMP_PRESET": "uc",
			},
			args:        []string{"--", "echo", "x"},
			wantDiff:    "diff -u --color",
			wantPresets: []string{"uc"},
		},
		{
			name: "env multiple presets comma separated",
			env: map[string]string{
				"CMDCOMP_CONFIG": configPath,
				"CMDCOMP_PRESET": "custom,other",
			},
			args:           []string{"--", "echo", "x"},
			wantDiff:       "other-yml-diff",
			wantShell:      "zsh",
			wantConfig:     configPath,
			wantPresets:    []string{"custom", "other"},
			wantPreprocess: []string{"grep other"},
			wantEnv:        []string{"K2=V2"},
		},
		{
			name: "env config and preset",
			env: map[string]string{
				"CMDCOMP_CONFIG": configPath,
				"CMDCOMP_PRESET": "custom",
			},
			args:           []string{"--", "echo", "x"},
			wantDiff:       "custom-yml-diff",
			wantShell:      "zsh",
			wantConfig:     configPath,
			wantPresets:    []string{"custom"},
			wantPreprocess: []string{"grep custom"},
			wantEnv:        []string{"K1=V1"},
		},
		{
			name: "flag overrides env preset completely",
			env: map[string]string{
				"CMDCOMP_CONFIG": configPath,
				"CMDCOMP_PRESET": "custom,uc",
			},
			args:           []string{"--preset", "other", "--", "echo", "x"},
			wantDiff:       "other-yml-diff",
			wantPresets:    []string{"other"},
			wantPreprocess: []string{"grep other"},
			wantEnv:        []string{"K2=V2"},
		},
		{
			name: "flag overrides env config",
			env: map[string]string{
				"CMDCOMP_CONFIG": "non-existent-config.yml",
				"CMDCOMP_PRESET": "custom",
			},
			args:           []string{"--config", configPath, "--", "echo", "x"},
			wantDiff:       "custom-yml-diff",
			wantShell:      "zsh",
			wantConfig:     configPath,
			wantPresets:    []string{"custom"},
			wantPreprocess: []string{"grep custom"},
			wantEnv:        []string{"K1=V1"},
		},
		{
			name: "env preset not found",
			env: map[string]string{
				"CMDCOMP_PRESET": "non-existent-preset",
			},
			args:       []string{"--", "echo", "x"},
			wantErrMsg: "preset not found non-existent-preset",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got, err := cli.ParseConfig(tc.args, os.Stdout, os.Stderr)
			if tc.wantErrMsg != "" {
				assert.ErrorContains(t, err, tc.wantErrMsg)
				return
			}
			assert.Nil(t, err)
			if tc.wantDiff != "" {
				assert.Equal(t, tc.wantDiff, got.Diff)
			}
			if tc.wantShell != "" {
				assert.Equal(t, tc.wantShell, got.Shell)
			}
			if tc.wantConfig != "" {
				assert.Equal(t, tc.wantConfig, got.ConfigPath)
			}
			if len(tc.wantPresets) > 0 {
				assert.Equal(t, tc.wantPresets, got.PresetNames)
			}
			if len(tc.wantPreprocess) > 0 {
				assert.Equal(t, tc.wantPreprocess, got.Preprocess)
			}
			if len(tc.wantEnv) > 0 {
				assert.Equal(t, tc.wantEnv, got.Env)
			}
		})
	}
}

func TestParseConfig_DefaultSection(t *testing.T) {
	const configWithDefault = `default:
  diff: default-diff
  shell: zsh
  preprocess:
    - grep default-preprocess
  env:
    - DEFAULT_KEY=DEFAULT_VAL
presets:
  custom:
    diff: custom-diff
    preprocess:
      - grep custom-preprocess
`
	configPath := filepath.Join(t.TempDir(), "default_config.yml")
	if !assert.Nil(t, os.WriteFile(configPath, []byte(configWithDefault), 0644)) {
		return
	}

	for _, tc := range []struct {
		name           string
		env            map[string]string
		args           []string
		wantDiff       string
		wantShell      string
		wantPreprocess []string
		wantEnv        []string
		wantPresets    []string
	}{
		{
			name: "load default section when no preset is given",
			args: []string{
				"--config", configPath,
				"--", "echo", "x",
			},
			wantDiff:       "default-diff",
			wantShell:      "zsh",
			wantPreprocess: []string{"grep default-preprocess"},
			wantEnv:        []string{"DEFAULT_KEY=DEFAULT_VAL"},
		},
		{
			name: "preset overrides default section",
			args: []string{
				"--config", configPath,
				"--preset", "custom",
				"--", "echo", "x",
			},
			wantDiff:       "custom-diff",
			wantShell:      "zsh", // inherited from default section
			wantPreprocess: []string{"grep custom-preprocess"}, // overridden by custom preset
			wantEnv:        []string{"DEFAULT_KEY=DEFAULT_VAL"}, // inherited from default section
			wantPresets:    []string{"custom"},
		},
		{
			name: "builtin preset overrides default section",
			args: []string{
				"--config", configPath,
				"-P", "uc",
				"--", "echo", "x",
			},
			wantDiff:       "diff -u --color", // overridden by uc preset
			wantShell:      "zsh",            // inherited from default section
			wantPreprocess: []string{"grep default-preprocess"},
			wantEnv:        []string{"DEFAULT_KEY=DEFAULT_VAL"},
			wantPresets:    []string{"uc"},
		},
		{
			name: "no-default flag ignores default section",
			args: []string{
				"--config", configPath,
				"--no-default",
				"--", "echo", "x",
			},
			wantDiff:  "diff", // builtin default
			wantShell: "bash", // builtin default
		},
		{
			name: "no-default flag with preset only applies preset",
			args: []string{
				"--config", configPath,
				"--no-default",
				"--preset", "custom",
				"--", "echo", "x",
			},
			wantDiff:       "custom-diff",
			wantShell:      "bash", // builtin default because default section was ignored
			wantPreprocess: []string{"grep custom-preprocess"},
			wantPresets:    []string{"custom"},
		},
		{
			name: "CMDCOMP_NO_DEFAULT env ignores default section",
			env: map[string]string{
				"CMDCOMP_NO_DEFAULT": "true",
			},
			args: []string{
				"--config", configPath,
				"--", "echo", "x",
			},
			wantDiff:  "diff",
			wantShell: "bash",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got, err := cli.ParseConfig(tc.args, os.Stdout, os.Stderr)
			assert.Nil(t, err)
			if tc.wantDiff != "" {
				assert.Equal(t, tc.wantDiff, got.Diff)
			}
			if tc.wantShell != "" {
				assert.Equal(t, tc.wantShell, got.Shell)
			}
			if len(tc.wantPreprocess) > 0 {
				assert.Equal(t, tc.wantPreprocess, got.Preprocess)
			}
			if len(tc.wantEnv) > 0 {
				assert.Equal(t, tc.wantEnv, got.Env)
			}
			if len(tc.wantPresets) > 0 {
				assert.Equal(t, tc.wantPresets, got.PresetNames)
			}
		})
	}
}


