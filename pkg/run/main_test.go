package run_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/berquerant/cmdcomp/pkg/run"
	"github.com/stretchr/testify/assert"
)

func TestMain(t *testing.T) {
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
