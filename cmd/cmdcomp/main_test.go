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

	const bin = "./bin/cmdcomp"

	t.Run("version", func(t *testing.T) {
		assert.Nil(t, run(t, os.Stdout, bin, "--version"))
	})

	t.Run("delimiter", func(t *testing.T) {
		for _, tc := range []struct {
			title      string
			arg        string
			want       string
			wantStatus int
		}{
			{
				title: "changed",
				arg:   bin + ` -d '---' -- echo --- echo -- a --- echo -- b`,
				want: `1c1
< echo -- a
---
> echo -- b
`,
				wantStatus: 1,
			},
			{
				title: "cmdcomp",
				arg:   fmt.Sprintf(`%[1]s -d '---' -- %[1]s --success -- echo -- a -- --- b --- c`, bin),
				want: `4c4
< > b
---
> > c
`,
				wantStatus: 1,
			},
		} {
			t.Run(tc.title, func(t *testing.T) {
				var got bytes.Buffer
				err := run(t, &got, "bash", "-c", tc.arg)
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
	})

	envEcho := filepath.Join(t.TempDir(), "envecho.sh")
	if !assert.Nil(t, os.WriteFile(envEcho, []byte(`#!/bin/bash
echo "${X}=${Y}"
`), 0755)) {
		return
	}

	for _, tc := range []struct {
		title      string
		arg        string
		want       string
		wantStatus int
	}{
		{
			title: "pass args to echo",
			arg:   "-- echo -- --debug a -- a",
			want: `1c1
< --debug a
---
> a
`,
			wantStatus: 1,
		},
		{
			title:      "no diff",
			arg:        "-- echo -- a -- a",
			want:       ``,
			wantStatus: 0,
		},
		{
			title: "success",
			arg:   "--success -- echo -- a -- b",
			want: `1c1
< a
---
> b
`,
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
			title: "preprocess left and right",
			arg:   `--leftPreprocess 'sed "s|a|c|"' --rightPreprocess 'sed "s|a|d|"' -- echo -- a -- a`,
			want: `1c1
< c
---
> d
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
		{
			title: "env script",
			arg:   `--env "X=x" --leftEnv "Y=1" --rightEnv "Y=2" -- ` + envEcho,
			want: `1c1
< x=1
---
> x=2
`,
			wantStatus: 1,
		},
		{
			title: "env",
			arg:   `--env "X=x" --leftEnv "Y=1" --rightEnv "Y=2" -- echo '$X=$Y'`,
			want: `1c1
< x=1
---
> x=2
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

	t.Run("cleanup", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out")
		arg := fmt.Sprintf(
			`--cleanup 'touch %s' -- echo -- a -- b`,
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

	t.Run("startup", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out")
		arg := fmt.Sprintf(
			`--startup 'touch %s' -- echo -- a -- b`,
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

	t.Run("all lifecycle hooks order", func(t *testing.T) {
		logFile := filepath.Join(t.TempDir(), "order.log")
		arg := fmt.Sprintf(
			`-s 'echo 1_startup >> %[1]s' -i 'echo 3_interceptor >> %[1]s' -c 'echo 5_cleanup >> %[1]s' -- bash -c -- 'echo 2_left >> %[1]s && echo same' -- 'echo 4_right >> %[1]s && echo same'`,
			logFile,
		)
		var got bytes.Buffer
		err := run(t, &got, "bash", "-c", bin+" "+arg)
		assert.Nil(t, err)
		assert.Equal(t, "", got.String())

		outBytes, err := os.ReadFile(logFile)
		assert.Nil(t, err)
		assert.Equal(t, `1_startup
2_left
3_interceptor
4_right
5_cleanup
`, string(outBytes))
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
