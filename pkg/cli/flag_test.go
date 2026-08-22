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
				ConfigPath: configPath,
				PresetName: "base",
				Diff:       "diff",
				Delimiter:  "--",
				Shell:      "bash",
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
				PresetName: "u",
				Diff:       "diff -u",
				Delimiter:  "--",
				Shell:      "bash",
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
				PresetName: "uc",
				Diff:       "diff -u --color",
				Delimiter:  "--",
				Shell:      "bash",
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
				PresetName: "u",
				Diff:       "diff -u",
				Delimiter:  "--",
				Shell:      "bash",
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
			name: "stdin and snapshot short flags",
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
			name: "config short flag",
			args: []string{
				"-C", configPath,
				"--preset", "base",
				"--", "echo", "x",
			},
			want: &cli.Config{
				ConfigPath: configPath,
				PresetName: "base",
				Diff:       "diff",
				Delimiter:  "--",
				Shell:      "bash",
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
