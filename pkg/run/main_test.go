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
		c := config.NewConfig(
			&stdout,
			[]string{
				"echo i1 >> " + out,
				"echo i2 >> " + out,
			},
			nil,
			"diff",
			"bash",
			"--",
		)
		c.WorkDir = t.TempDir()
		c.Debug = true
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
			title:   "no args",
			c:       config.NewConfig(nil, nil, nil, "diff", "bash", "--"),
			args:    []string{},
			initErr: true,
			errMsg:  "no args",
		},
		{
			title: "left is equal to right",
			c:     config.NewConfig(nil, nil, nil, "diff", "bash", "--"),
			args:  []string{"echo", "--", "a", "--", "a"},
			want:  "",
		},
		{
			title: "left is not equal to right",
			c:     config.NewConfig(nil, nil, nil, "diff", "bash", "--"),
			args:  []string{"echo", "--", "a", "--", "b"},
			want: `1c1
< a
---
> b
`,
			errMsg: "exit status 1",
		},
		{
			title: "change delimiter",
			c:     config.NewConfig(nil, nil, nil, "diff", "bash", "---"),
			args:  []string{"echo", "---", "--", "a", "---", "b"},
			want: `1c1
< -- a
---
> b
`,
			errMsg: "exit status 1",
		},
		{
			title: "left is not equal to right without common",
			c:     config.NewConfig(nil, nil, nil, "diff", "bash", "--"),
			args:  []string{"--", "echo", "a", "--", "echo", "b"},
			want: `1c1
< a
---
> b
`,
			errMsg: "exit status 1",
		},
		{
			title: "preprocess1",
			c: config.NewConfig(nil, nil, []string{
				`sed 's|a|c|'`,
			}, "diff", "bash", "--"),
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
			c: config.NewConfig(nil, nil, []string{
				`sed 's|a|c|'`,
				`sed 's|b|d|'`,
			}, "diff", "bash", "--"),
			args: []string{"echo", "--", "a", "--", "b"},
			want: `1c1
< c
---
> d
`,
			errMsg: "exit status 1",
		},
		{
			title: "customize diff",
			c:     config.NewConfig(nil, nil, nil, "diff -u --label L --label R", "bash", "--"),
			args:  []string{"echo", "--", "a", "--", "b"},
			want: `--- L
+++ R
@@ -1 +1 @@
-a
+b
`,
			errMsg: "exit status 1",
		},
		{
			title:  "left fail",
			c:      config.NewConfig(nil, nil, nil, "diff", "bash", "--"),
			args:   []string{"--", "bash", "-c", "exit 2", "--", "echo ", "b"},
			errMsg: "exit status 2: run left",
		},
		{
			title:  "right fail",
			c:      config.NewConfig(nil, nil, nil, "diff", "bash", "--"),
			args:   []string{"--", "echo", "a", "--", "bash", "-c", "exit 2"},
			errMsg: "exit status 2: run right",
		},
		{
			title: "preprocess1 right fail",
			c: config.NewConfig(nil, nil, []string{
				`grep "a"`,
			}, "diff", "bash", "--"),
			args:   []string{"echo", "--", "a", "--", "b"},
			errMsg: "exit status 1: run preprocess[0] for right",
		},
		{
			title: "preprocess2 left fail",
			c: config.NewConfig(nil, nil, []string{
				`grep "a"`,
				`grep "b"`,
			}, "diff", "bash", "--"),
			args:   []string{"echo", "--", "a", "--", "a"},
			errMsg: "exit status 1: run preprocess[1] for left",
		},
		{
			title: "interceptor1 fail",
			c: config.NewConfig(nil, []string{
				"exit 1",
			}, nil, "diff", "bash", "--"),
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
