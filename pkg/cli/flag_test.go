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
    config:
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
				Diff:      "diff",
				Delimiter: "--",
				Shell:     "bash",
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
			name: "shell short flag",
			args: []string{
				"-S", "sh",
				"--", "echo", "x",
			},
			want: &cli.Config{
				Diff:      "diff",
				Delimiter: "--",
				Shell:     "sh",
				CommonArgs: []string{
					"echo", "x",
				},
			},
		},
		{
			name: "timeout and processTimeout",
			args: []string{
				"--timeout", "1m",
				"--processTimeout", "5s",
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
			got.TempDir = ""
			assert.Equal(t, tc.want, got)
		})
	}
}
