package run_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/berquerant/cmdcomp/pkg/run"
	"github.com/stretchr/testify/assert"
)

func TestMain(t *testing.T) {
	t.Run("cleanup", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "output")
		var stdout bytes.Buffer
		c := &config.Config{
			Writer: &stdout,
			Cleanup: []string{
				"echo i1 >> " + out,
				"echo i2 >> " + out,
			},
			Diff:      "diff",
			Shell:     "bash",
			Delimiter: "--",
			WorkDir:   t.TempDir(),
			Debug:     true,
		}
		c.SetupLogger(os.Stderr)
		assert.Nil(t, c.Init([]string{
			"echo", "--", "a", "--", "b",
		}))
		err := run.Main(c)
		assert.NotNil(t, err)
		var exitErr *exec.ExitError
		assert.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 1, exitErr.ExitCode())
		assert.Equal(t, `1c1
< a
---
> b
`, stdout.String())
		outBytes, err := os.ReadFile(out)
		assert.Nil(t, err)
		assert.Equal(t, `i1
i2
`, string(outBytes))
	})
	t.Run("interceptor", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "output")
		var stdout bytes.Buffer
		c := &config.Config{
			Writer: &stdout,
			Interceptor: []string{
				"echo i1 >> " + out,
				"echo i2 >> " + out,
			},
			Diff:      "diff",
			Shell:     "bash",
			Delimiter: "--",
			WorkDir:   t.TempDir(),
			Debug:     true,
		}
		c.SetupLogger(os.Stderr)
		assert.Nil(t, c.Init([]string{
			"echo", "--", "a", "--", "b",
		}))
		err := run.Main(c)
		assert.NotNil(t, err)
		var exitErr *exec.ExitError
		assert.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 1, exitErr.ExitCode())
		assert.Equal(t, `1c1
< a
---
> b
`, stdout.String())
		outBytes, err := os.ReadFile(out)
		assert.Nil(t, err)
		assert.Equal(t, `i1
i2
`, string(outBytes))
	})
	t.Run("startup", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "output")
		var stdout bytes.Buffer
		c := &config.Config{
			Writer: &stdout,
			Startup: []string{
				"echo s1 >> " + out,
				"echo s2 >> " + out,
			},
			Diff:      "diff",
			Shell:     "bash",
			Delimiter: "--",
			WorkDir:   t.TempDir(),
			Debug:     true,
		}
		c.SetupLogger(os.Stderr)
		assert.Nil(t, c.Init([]string{
			"bash", "-c", "--", "cat " + out, "--", "printf 's1\\ns2\\n'",
		}))
		err := run.Main(c)
		assert.Nil(t, err)
		assert.Equal(t, "", stdout.String())
	})
	t.Run("startup, interceptor, and cleanup order", func(t *testing.T) {
		logFile := filepath.Join(t.TempDir(), "order.log")
		var stdout bytes.Buffer
		c := &config.Config{
			Writer: &stdout,
			Startup: []string{
				"echo 1_startup >> " + logFile,
			},
			Interceptor: []string{
				"echo 3_interceptor >> " + logFile,
			},
			Cleanup: []string{
				"echo 5_cleanup >> " + logFile,
			},
			Diff:      "diff",
			Shell:     "bash",
			Delimiter: "--",
			WorkDir:   t.TempDir(),
			Debug:     true,
		}
		c.SetupLogger(os.Stderr)
		assert.Nil(t, c.Init([]string{
			"bash", "-c", "--", "echo 2_left >> " + logFile + " && echo same", "--", "echo 4_right >> " + logFile + " && echo same",
		}))
		err := run.Main(c)
		assert.Nil(t, err)
		assert.Equal(t, "", stdout.String())

		outBytes, err := os.ReadFile(logFile)
		assert.Nil(t, err)
		assert.Equal(t, `1_startup
2_left
3_interceptor
4_right
5_cleanup
`, string(outBytes))
	})

	for _, tc := range []struct {
		title   string
		c       *config.Config
		args    []string
		want    string
		initErr bool
		errMsg  string
	}{
		{
			title: "no args",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
			},
			args:    []string{},
			initErr: true,
			errMsg:  "no args",
		},
		{
			title: "left is equal to right",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
			},
			args: []string{"echo", "--", "a", "--", "a"},
			want: "",
		},
		{
			title: "left is not equal to right",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
			},
			args: []string{"echo", "--", "a", "--", "b"},
			want: `1c1
< a
---
> b
`,
			errMsg: "exit status 1",
		},
		{
			title: "use label with unified diff",
			c: &config.Config{
				Diff:      "diff -u",
				Shell:     "bash",
				Delimiter: "--",
				UseLabel:  true,
			},
			args: []string{"echo", "--", "a", "--", "b"},
			want: `--- echo___a
+++ echo___b
@@ -1 +1 @@
-a
+b
`,
			errMsg: "exit status 1",
		},
		{
			title: "change delimiter",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "---",
			},
			args: []string{"echo", "---", "--", "a", "---", "b"},
			want: `1c1
< -- a
---
> b
`,
			errMsg: "exit status 1",
		},
		{
			title: "left is not equal to right without common",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
			},
			args: []string{"--", "echo", "a", "--", "echo", "b"},
			want: `1c1
< a
---
> b
`,
			errMsg: "exit status 1",
		},
		{
			title: "preprocess1",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				Preprocess: []string{
					`sed 's|a|c|'`,
				},
			},
			args: []string{"echo", "--", "a", "--", "b"},
			want: `1c1
< c
---
> b
`,
			errMsg: "exit status 1",
		},
		{
			title: "preprocess2",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				Preprocess: []string{
					`sed 's|a|c|'`,
					`sed 's|b|d|'`,
				},
			},
			args: []string{"echo", "--", "a", "--", "b"},
			want: `1c1
< c
---
> d
`,
			errMsg: "exit status 1",
		},
		{
			title: "left preprocess",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				LeftPreprocess: []string{
					`sed 's|a|c|'`,
				},
			},
			args: []string{"echo", "--", "a", "--", "a"},
			want: `1c1
< c
---
> a
`,
			errMsg: "exit status 1",
		},
		{
			title: "right preprocess",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				RightPreprocess: []string{
					`sed 's|b|d|'`,
				},
			},
			args: []string{"echo", "--", "b", "--", "b"},
			want: `1c1
< b
---
> d
`,
			errMsg: "exit status 1",
		},
		{
			title: "right preprocess with preprocess",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				Preprocess: []string{
					`sed 's|a|b|'`,
				},
				RightPreprocess: []string{
					`sed 's|b|d|'`,
				},
			},
			args: []string{"echo", "--", "a", "--", "b"},
			want: `1c1
< b
---
> d
`,
			errMsg: "exit status 1",
		},
		{
			title: "customize diff",
			c: &config.Config{
				Diff:      "diff -u --label L --label R",
				Shell:     "bash",
				Delimiter: "--",
			},
			args: []string{"echo", "--", "a", "--", "b"},
			want: `--- L
+++ R
@@ -1 +1 @@
-a
+b
`,
			errMsg: "exit status 1",
		},
		{
			title: "left fail",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
			},
			args:   []string{"bash", "-c", "--", "exit 2", "--", "echo b"},
			errMsg: "exit status 2: run left",
		},
		{
			title: "right fail",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
			},
			args:   []string{"bash", "-c", "--", "echo", "a", "--", "exit 2"},
			errMsg: "exit status 2: run right",
		},
		{
			title: "preprocess right fail",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				Preprocess: []string{
					`grep "a"`,
				},
			},
			args:   []string{"echo", "--", "a", "--", "b"},
			errMsg: "preprocess",
		},
		{
			title: "preprocess left fail",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				Preprocess: []string{
					`grep "a"`,
					`grep "b"`,
				},
			},
			args:   []string{"echo", "--", "a", "--", "a"},
			errMsg: "preprocess",
		},
		{
			title: "startup1 fail",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				Startup: []string{
					`exit 1`,
				},
			},
			args:   []string{"echo", "--", "a", "--", "b"},
			errMsg: "exit status 1: run startup[0]",
		},
		{
			title: "interceptor1 fail",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				Interceptor: []string{
					`exit 1`,
				},
			},
			args:   []string{"echo", "--", "a", "--", "b"},
			errMsg: "exit status 1: run interceptor[0]",
		},
		{
			title: "env",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				LeftEnv: []string{
					"X=a",
				},
				RightEnv: []string{
					"X=b",
				},
			},
			args: []string{"bash", "-c", "echo $X"},
			want: `1c1
< a
---
> b
`,
			errMsg: "exit status 1",
		},
		{
			title: "total timeout exceeded in left process",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				Timeout:   50 * time.Millisecond,
			},
			args:   []string{"bash", "-c", "--", "sleep 1", "--", "echo b"},
			errMsg: "run left",
		},
		{
			title: "process timeout exceeded in right process",
			c: &config.Config{
				Diff:           "diff",
				Shell:          "bash",
				Delimiter:      "--",
				ProcessTimeout: 50 * time.Millisecond,
			},
			args:   []string{"bash", "-c", "--", "echo a", "--", "sleep 1"},
			errMsg: "run right",
		},
		{
			title: "process timeout exceeded in preprocess",
			c: &config.Config{
				Diff:           "diff",
				Shell:          "bash",
				Delimiter:      "--",
				ProcessTimeout: 50 * time.Millisecond,
				Preprocess: []string{
					"sleep 1",
				},
			},
			args:   []string{"echo", "--", "a", "--", "b"},
			errMsg: "pipeline",
		},
		{
			title: "cleanup runs even if process timeout is set",
			c: &config.Config{
				Diff:           "diff",
				Shell:          "bash",
				Delimiter:      "--",
				ProcessTimeout: 50 * time.Millisecond,
				Cleanup: []string{
					"sleep 0.1",
				},
			},
			args: []string{"echo", "--", "a", "--", "a"},
			want: "",
		},
		{
			title: "stdin from reader (-) without diff",
			c: &config.Config{
				Diff:      "diff",
				Shell:     "bash",
				Delimiter: "--",
				Stdin:     "-",
				Reader:    bytes.NewBufferString("hello from stdin\n"),
			},
			args: []string{"cat"},
			want: "",
		},
		{
			title: "stdin from reader (-) with diff via sed",
			c: &config.Config{
				Diff:            "diff",
				Shell:           "bash",
				Delimiter:       "--",
				Stdin:           "-",
				Reader:          bytes.NewBufferString("hello world\n"),
				LeftPreprocess:  []string{`sed 's|world|left|'`},
				RightPreprocess: []string{`sed 's|world|right|'`},
			},
			args: []string{"cat"},
			want: `1c1
< hello left
---
> hello right
`,
			errMsg: "exit status 1",
		},
		{
			title: "stdin from file (@) without diff",
			c: func() *config.Config {
				f := filepath.Join(t.TempDir(), "input.txt")
				_ = os.WriteFile(f, []byte("file content\n"), 0644)
				return &config.Config{
					Diff:      "diff",
					Shell:     "bash",
					Delimiter: "--",
					Stdin:     "@" + f,
				}
			}(),
			args: []string{"cat"},
			want: "",
		},
		{
			title: "stdin from file (@) with diff via grep",
			c: func() *config.Config {
				f := filepath.Join(t.TempDir(), "input.txt")
				_ = os.WriteFile(f, []byte("alpha\nbeta\n"), 0644)
				return &config.Config{
					Diff:      "diff",
					Shell:     "bash",
					Delimiter: "--",
					Stdin:     "@" + f,
				}
			}(),
			args: []string{"grep", "--", "alpha", "--", "beta"},
			want: `1c1
< alpha
---
> beta
`,
			errMsg: "exit status 1",
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			var out bytes.Buffer
			tc.c.Writer = &out
			tc.c.WorkDir = t.TempDir()
			tc.c.Debug = true
			tc.c.SetupLogger(os.Stderr)
			err := tc.c.Init(tc.args)
			if x := tc.errMsg; x != "" && tc.initErr {
				assert.ErrorContains(t, err, x)
				return
			}
			assert.Nil(t, err)
			err = run.Main(tc.c)
			if x := tc.errMsg; x != "" {
				assert.ErrorContains(t, err, x, "%v", err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tc.want, out.String())
		})
	}
}
