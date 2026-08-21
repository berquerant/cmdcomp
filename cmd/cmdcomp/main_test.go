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

type e2eTestCase struct {
	title      string
	arg        string // flags and arguments passed to bin (e.g. "-x 'diff -u' -- echo -- a -- b")
	want       string
	wantStderr string
	wantStatus int
	skipDryrun bool // set to true if the dryrun script cannot be executed directly (e.g., nested cmdcomp)
}

func (tc e2eTestCase) run(t *testing.T, bin string) {
	t.Run(tc.title, func(t *testing.T) {
		t.Run("direct", func(t *testing.T) {
			var got, gotErr bytes.Buffer
			err := runWithStderr(t, &got, &gotErr, "bash", "-c", bin+" "+tc.arg)
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
			if tc.wantStderr != "" {
				assert.Contains(t, gotErr.String(), tc.wantStderr)
			}
		})

		if !tc.skipDryrun {
			t.Run("dryrun", func(t *testing.T) {
				var script bytes.Buffer
				if !assert.Nil(t, run(t, &script, "bash", "-c", bin+" --dry-run "+tc.arg), "dryrun must exit 0") {
					return
				}
				var got bytes.Buffer
				cmd := exec.Command("bash", "-c", script.String())
				cmd.Stdout = &got
				cmd.Stderr = os.Stderr
				_ = cmd.Run() // ignore exit code; only stdout content is compared
				assert.Equal(t, tc.want, got.String())
			})
		}
	})
}

func TestE2E(t *testing.T) {
	if !assert.Nil(t, run(t, os.Stdout, "make"), "should build successfully") {
		return
	}

	const bin = "./bin/cmdcomp"

	t.Run("version", func(t *testing.T) {
		assert.Nil(t, run(t, os.Stdout, bin, "--version"))
	})

	t.Run("delimiter", func(t *testing.T) {
		for _, tc := range []e2eTestCase{
			{
				title: "changed",
				arg:   `-d '---' -- echo --- echo -- a --- echo -- b`,
				want: `1c1
< echo -- a
---
> echo -- b
`,
				wantStatus: 1,
			},
			{
				title: "cmdcomp",
				arg:   fmt.Sprintf(`-d '---' -- %[1]s --success -- echo -- a -- --- b --- c`, bin),
				want: `4c4
< > b
---
> > c
`,
				wantStatus: 1,
				skipDryrun: true,
			},
		} {
			tc.run(t, bin)
		}
	})

	envEcho := filepath.Join(t.TempDir(), "envecho.sh")
	if !assert.Nil(t, os.WriteFile(envEcho, []byte(`#!/bin/bash
echo "${X}=${Y}"
`), 0755)) {
		return
	}

	for _, tc := range []e2eTestCase{
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
			arg:   `--left-preprocess 'sed "s|a|c|"' --right-preprocess 'sed "s|a|d|"' -- echo -- a -- a`,
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
			arg:   `--env "X=x" --left-env "Y=1" --right-env "Y=2" -- ` + envEcho,
			want: `1c1
< x=1
---
> x=2
`,
			wantStatus: 1,
		},
		{
			title: "env",
			arg:   `--env "X=x" --left-env "Y=1" --right-env "Y=2" -- echo '$X=$Y'`,
			want: `1c1
< x=1
---
> x=2
`,
			wantStatus: 1,
		},
		{
			title:      "left fail e2e",
			arg:        `-- bash -c -- "exit 2" -- "echo b"`,
			wantStatus: 2,
			wantStderr: "run left",
			skipDryrun: true,
		},
		{
			title:      "left fail e2e with --success",
			arg:        `--success -- bash -c -- "exit 2" -- "echo b"`,
			wantStatus: 2,
			wantStderr: "run left",
			skipDryrun: true,
		},
		{
			title:      "right fail e2e",
			arg:        `-- bash -c -- "echo a" -- "exit 2"`,
			wantStatus: 2,
			wantStderr: "run right",
			skipDryrun: true,
		},
		{
			title:      "startup fail e2e",
			arg:        `-s 'exit 3' -- echo -- a -- b`,
			wantStatus: 2,
			wantStderr: "run startup[0]",
			skipDryrun: true,
		},
		{
			title:      "left preprocess fail e2e",
			arg:        `--left-preprocess 'grep non_existent' -- echo -- a -- b`,
			wantStatus: 2,
			wantStderr: "run preprocess:left pipeline",
			skipDryrun: true,
		},
		{
			title:      "right preprocess fail e2e",
			arg:        `--right-preprocess 'grep non_existent' -- echo -- a -- b`,
			wantStatus: 2,
			wantStderr: "run preprocess:right pipeline",
			skipDryrun: true,
		},
		{
			title:      "total timeout e2e",
			arg:        `--timeout 50ms -- bash -c -- "sleep 1" -- "echo b"`,
			wantStatus: 2,
			wantStderr: "run left",
			skipDryrun: true,
		},
		{
			title:      "process timeout e2e",
			arg:        `--process-timeout 50ms -- bash -c -- "echo a" -- "sleep 1"`,
			wantStatus: 2,
			wantStderr: "run right",
			skipDryrun: true,
		},
	} {
		tc.run(t, bin)
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

	t.Run("dryrun", func(t *testing.T) {
		for _, tc := range []struct {
			title    string
			arg      string
			contains []string
		}{
			{
				title: "exits zero and outputs shebang",
				arg:   "--dry-run -- echo -- a -- b",
				contains: []string{
					"#!/usr/bin/env bash",
					"set -euo pipefail",
					"_CMDCOMP_TMPDIR=$(mktemp -d)",
					"# left",
					"echo a",
					"# right",
					"echo b",
					"# diff",
					"diff ",
				},
			},
			{
				title: "custom diff and preprocess appear in script",
				arg:   `--dry-run -x 'diff -u' -p 'sed "s|a|c|"' -- echo -- a -- b`,
				contains: []string{
					"diff -u",
					`sed "s|a|c|"`,
					"# preprocess:left",
					"# preprocess:right",
				},
			},
			{
				title: "startup and cleanup hooks appear in script",
				arg:   `--dry-run -s 'echo startup1' -c 'echo cleanup1' -- echo -- a -- b`,
				contains: []string{
					"# startup[0]",
					"echo startup1",
					"# cleanup[0]",
					"echo cleanup1",
				},
			},
			{
				title: "interceptor appears in script",
				arg:   `--dry-run -i 'echo interceptor1' -- echo -- a -- b`,
				contains: []string{
					"# interceptor[0]",
					"echo interceptor1",
				},
			},
			{
				title: "generated script is executable and produces diff output",
				// dryrun generates a script; running that script should produce actual diff
				arg: `--dry-run -- echo -- a -- b`,
			},
			// ---- multiple hooks / preprocesses / interceptors ----
			{
				title: "multiple startup hooks all appear with correct indices",
				arg:   `--dry-run -s 'echo s0' -s 'echo s1' -s 'echo s2' -- echo -- a -- b`,
				contains: []string{
					"# startup[0]", "echo s0",
					"# startup[1]", "echo s1",
					"# startup[2]", "echo s2",
				},
			},
			{
				title: "multiple cleanup hooks all appear with correct indices",
				arg:   `--dry-run -c 'echo c0' -c 'echo c1' -c 'echo c2' -- echo -- a -- b`,
				contains: []string{
					"# cleanup[0]", "echo c0",
					"# cleanup[1]", "echo c1",
					"# cleanup[2]", "echo c2",
				},
			},
			{
				title: "multiple interceptors all appear with correct indices",
				arg:   `--dry-run -i 'echo i0' -i 'echo i1' -i 'echo i2' -- echo -- a -- b`,
				contains: []string{
					"# interceptor[0]", "echo i0",
					"# interceptor[1]", "echo i1",
					"# interceptor[2]", "echo i2",
				},
			},
			{
				title: "multiple preprocess commands form a pipeline",
				arg:   `--dry-run -p 'sed "s|a|x|"' -p 'sed "s|x|y|"' -p cat -- echo -- a -- b`,
				contains: []string{
					"# preprocess:left",
					`sed "s|a|x|"`,
					`| sed "s|x|y|"`,
					"| cat",
				},
			},
			{
				title: "leftPreprocess and rightPreprocess independently",
				arg:   `--dry-run --left-preprocess 'tr a A' --left-preprocess 'tr A Z' --right-preprocess 'tr b B' -- echo -- a -- b`,
				contains: []string{
					"# preprocess:left", "tr a A", "| tr A Z",
					"# preprocess:right", "tr b B",
				},
			},
			{
				title: "all hooks and preprocesses combined",
				arg:   `--dry-run -s 'echo s0' -s 'echo s1' -i 'echo i0' -i 'echo i1' -c 'echo c0' -p 'cat' --left-preprocess 'tr a L' --right-preprocess 'tr b R' -- echo -- a -- b`,
				contains: []string{
					"# startup[0]", "echo s0",
					"# startup[1]", "echo s1",
					"# interceptor[0]", "echo i0",
					"# interceptor[1]", "echo i1",
					"# cleanup[0]", "echo c0",
					"# preprocess:left", "tr a L",
					"# preprocess:right", "tr b R",
					"# diff",
				},
			},
		} {
			t.Run(tc.title, func(t *testing.T) {
				var got bytes.Buffer
				err := run(t, &got, "bash", "-c", bin+" "+tc.arg)
				// dryrun must always exit 0
				assert.Nil(t, err)
				out := got.String()
				for _, want := range tc.contains {
					assert.Contains(t, out, want)
				}
			})
		}

		t.Run("generated script produces real diff when executed", func(t *testing.T) {
			// Capture the dry-run script, then run it with bash and verify it produces diff output.
			var script bytes.Buffer
			err := run(t, &script, "bash", "-c", bin+" --dry-run -- echo -- a -- b")
			if !assert.Nil(t, err) {
				return
			}
			var diffOut bytes.Buffer
			cmd := exec.Command("bash", "-c", script.String())
			cmd.Stdout = &diffOut
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			// diff exits 1 when files differ
			var exitErr *exec.ExitError
			if assert.True(t, errors.As(err, &exitErr)) {
				assert.Equal(t, 1, exitErr.ExitCode())
			}
			assert.Equal(t, "1c1\n< a\n---\n> b\n", diffOut.String())
		})
	})
}

func run(t *testing.T, stdout io.Writer, name string, arg ...string) error {
	t.Helper()
	return runWithStderr(t, stdout, os.Stderr, name, arg...)
}

func runWithStderr(t *testing.T, stdout, stderr io.Writer, name string, arg ...string) error {
	t.Helper()
	c := exec.Command(name, arg...)
	c.Dir = "../.."
	c.Stdout = stdout
	c.Stderr = stderr
	t.Logf("run:%v", c.Args)
	return c.Run()
}
