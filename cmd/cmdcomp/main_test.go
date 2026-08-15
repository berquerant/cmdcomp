package main_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		delimCases := []struct {
			title      string
			arg        string // full bash command (includes bin)
			want       string
			wantStatus int
			// skipDryrun marks cases where the generated script cannot be
			// executed standalone (e.g. it calls bin itself as a subprocess).
			skipDryrun bool
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
				arg:  fmt.Sprintf(`%[1]s -d '---' -- %[1]s --success -- echo -- a -- --- b --- c`, bin),
				want: `4c4
< > b
---
> > c
`,
				wantStatus: 1,
				// The generated script would invoke bin as a subprocess; skip
				// because bin may not be in PATH when the script is executed.
				skipDryrun: true,
			},
		}

		for _, tc := range delimCases {
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

		// For each non-skipped delimiter case, verify that the dry-run script,
		// when executed with bash, produces the same stdout as the direct run.
		t.Run("dryrun script equivalence", func(t *testing.T) {
			for _, tc := range delimCases {
				if tc.skipDryrun {
					continue
				}
				t.Run(tc.title, func(t *testing.T) {
					// Insert --dryrun immediately after the binary name in the arg.
					dryrunArg := strings.Replace(tc.arg, bin+" ", bin+" --dryrun ", 1)
					var script bytes.Buffer
					if !assert.Nil(t, run(t, &script, "bash", "-c", dryrunArg), "dryrun must exit 0") {
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
	})

	envEcho := filepath.Join(t.TempDir(), "envecho.sh")
	if !assert.Nil(t, os.WriteFile(envEcho, []byte(`#!/bin/bash
echo "${X}=${Y}"
`), 0755)) {
		return
	}

	// generalCases is the single source of truth for all non-side-effect test scenarios.
	// The same slice is consumed by both the direct-execution loop and the
	// dry-run script equivalence loop, so new cases are covered by both automatically.
	generalCases := []struct {
		title      string
		arg        string // flags and positional args only (bin is NOT included)
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
	}

	// Direct-execution loop: same behaviour as before.
	for _, tc := range generalCases {
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

	// Dry-run script equivalence loop: for every case in generalCases, verify that
	// the script emitted by --dryrun, when executed with bash, produces the same
	// stdout as the direct run. Exit code of the script is intentionally ignored
	// because --success only affects cmdcomp's own exit code, not diff's.
	t.Run("dryrun script equivalence", func(t *testing.T) {
		for _, tc := range generalCases {
			t.Run(tc.title, func(t *testing.T) {
				var script bytes.Buffer
				if !assert.Nil(t, run(t, &script, "bash", "-c", bin+" --dryrun "+tc.arg), "dryrun must exit 0") {
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
				arg:   "--dryrun -- echo -- a -- b",
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
				arg:   `--dryrun -x 'diff -u' -p 'sed "s|a|c|"' -- echo -- a -- b`,
				contains: []string{
					"diff -u",
					`sed "s|a|c|"`,
					"# preprocess:left",
					"# preprocess:right",
				},
			},
			{
				title: "startup and cleanup hooks appear in script",
				arg:   `--dryrun -s 'echo startup1' -c 'echo cleanup1' -- echo -- a -- b`,
				contains: []string{
					"# startup[0]",
					"echo startup1",
					"# cleanup[0]",
					"echo cleanup1",
				},
			},
			{
				title: "interceptor appears in script",
				arg:   `--dryrun -i 'echo interceptor1' -- echo -- a -- b`,
				contains: []string{
					"# interceptor[0]",
					"echo interceptor1",
				},
			},
			{
				title: "generated script is executable and produces diff output",
				// dryrun generates a script; running that script should produce actual diff
				arg: `--dryrun -- echo -- a -- b`,
			},
			// ---- multiple hooks / preprocesses / interceptors ----
			{
				title: "multiple startup hooks all appear with correct indices",
				arg:   `--dryrun -s 'echo s0' -s 'echo s1' -s 'echo s2' -- echo -- a -- b`,
				contains: []string{
					"# startup[0]", "echo s0",
					"# startup[1]", "echo s1",
					"# startup[2]", "echo s2",
				},
			},
			{
				title: "multiple cleanup hooks all appear with correct indices",
				arg:   `--dryrun -c 'echo c0' -c 'echo c1' -c 'echo c2' -- echo -- a -- b`,
				contains: []string{
					"# cleanup[0]", "echo c0",
					"# cleanup[1]", "echo c1",
					"# cleanup[2]", "echo c2",
				},
			},
			{
				title: "multiple interceptors all appear with correct indices",
				arg:   `--dryrun -i 'echo i0' -i 'echo i1' -i 'echo i2' -- echo -- a -- b`,
				contains: []string{
					"# interceptor[0]", "echo i0",
					"# interceptor[1]", "echo i1",
					"# interceptor[2]", "echo i2",
				},
			},
			{
				title: "multiple preprocess commands form a pipeline",
				arg:   `--dryrun -p 'sed "s|a|x|"' -p 'sed "s|x|y|"' -p cat -- echo -- a -- b`,
				contains: []string{
					"# preprocess:left",
					`sed "s|a|x|"`,
					`| sed "s|x|y|"`,
					"| cat",
				},
			},
			{
				title: "leftPreprocess and rightPreprocess independently",
				arg:   `--dryrun --leftPreprocess 'tr a A' --leftPreprocess 'tr A Z' --rightPreprocess 'tr b B' -- echo -- a -- b`,
				contains: []string{
					"# preprocess:left", "tr a A", "| tr A Z",
					"# preprocess:right", "tr b B",
				},
			},
			{
				title: "all hooks and preprocesses combined",
				arg:   `--dryrun -s 'echo s0' -s 'echo s1' -i 'echo i0' -i 'echo i1' -c 'echo c0' -p 'cat' --leftPreprocess 'tr a L' --rightPreprocess 'tr b R' -- echo -- a -- b`,
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
			err := run(t, &script, "bash", "-c", bin+" --dryrun -- echo -- a -- b")
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
	c := exec.Command(name, arg...)
	c.Dir = "../.."
	c.Stdout = stdout
	c.Stderr = os.Stderr
	t.Logf("run:%v", c.Args)
	return c.Run()
}
