package main_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestE2E(t *testing.T) {
	if !assert.Nil(t, run(t, os.Stdout, "make"), "should build successfully") {
		return
	}

	const bin = "./dist/cmdcomp"
	for _, tc := range []struct {
		title      string
		arg        string
		want       string
		wantStatus int
	}{
		{
			title:      "no diff",
			arg:        "-- echo -- a -- a",
			want:       ``,
			wantStatus: 0,
		},
		{
			title: "echo",
			arg:   "-- echo -- a -- b",
			want: `1c1
< a
---
> b
`,
			wantStatus: 1,
		},
		{
			title: "customized diff",
			arg:   "-x 'diff -u --label L --label R' -- echo -- a -- b",
			want: `--- L
+++ R
@@ -1 +1 @@
-a
+b
`,
			wantStatus: 1,
		},
		{
			title: "preprocess sed",
			arg:   `-p 'sed "s|a|c|"' -- echo -- a -- b`,
			want: `1c1
< c
---
> b
`,
			wantStatus: 1,
		},
		{
			title: "preprocess awk",
			arg:   `-p "awk '{print \$1\"x\"}'" -- echo -- a -- b`,
			want: `1c1
< ax
---
> bx
`,
			wantStatus: 1,
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			var got bytes.Buffer
			err := run(t, &got, "bash", "-c", bin+" "+tc.arg)
			if tc.wantStatus == 0 {
				assert.Nil(t, err)
			} else {
				var exitErr *exec.ExitError
				if !assert.True(t, errors.As(err, &exitErr)) {
					return
				}
				assert.Equal(t, tc.wantStatus, exitErr.ExitCode())
			}
			assert.Equal(t, tc.want, got.String())
		})
	}

	t.Run("interceptor", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out")
		arg := fmt.Sprintf(
			`-i 'touch %s' -- echo -- a -- b`,
			out,
		)
		var got bytes.Buffer
		err := run(t, &got, "bash", "-c", bin+" "+arg)
		var exitErr *exec.ExitError
		if !assert.True(t, errors.As(err, &exitErr)) {
			return
		}
		assert.Equal(t, 1, exitErr.ExitCode())
		assert.FileExists(t, out)
		assert.Equal(t, `1c1
< a
---
> b
`, got.String())
	})
}

func run(t *testing.T, stdout io.Writer, name string, arg ...string) error {
	t.Helper()
	c := exec.Command(name, arg...)
	c.Dir = "../.."
	c.Stdout = stdout
	c.Stderr = os.Stderr
	t.Logf("run:%v", c.Args)
	return c.Run()
}
