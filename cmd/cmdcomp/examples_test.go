package main_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/berquerant/cmdcomp/pkg/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinExamples(t *testing.T) {
	require.NoError(t, run(t, os.Stdout, "make"))
	bin, err := filepath.Abs("../../bin/cmdcomp")
	require.NoError(t, err)
	t.Setenv("PATH", filepath.Dir(bin)+":"+os.Getenv("PATH"))

	for _, ex := range cli.BuiltinExamples {
		t.Run(ex.Title, func(t *testing.T) {
			// Extract command string after "cmdcomp "
			require.True(t, strings.HasPrefix(ex.Command, "cmdcomp "))
			cmdArgs := strings.TrimPrefix(ex.Command, "cmdcomp ")

			// 1. Test direct execution
			t.Run("direct", func(t *testing.T) {
				tmpDir := t.TempDir()
				for filename, content := range ex.SetupFiles {
					p := filepath.Join(tmpDir, filename)
					require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
					require.NoError(t, os.WriteFile(p, []byte(content), 0644))
				}

				var stdout, stderr bytes.Buffer
				c := exec.Command("bash", "-c", bin+" "+cmdArgs)
				c.Dir = tmpDir
				c.Stdout = &stdout
				c.Stderr = &stderr
				if ex.Stdin != "" {
					c.Stdin = bytes.NewBufferString(ex.Stdin)
				}
				err := c.Run()

				if ex.WantStatus == 0 {
					assert.NoError(t, err, "stderr: %s", stderr.String())
				} else {
					var exitErr *exec.ExitError
					require.True(t, errors.As(err, &exitErr), "expected exit error, got %v", err)
					assert.Equal(t, ex.WantStatus, exitErr.ExitCode(), "stderr: %s", stderr.String())
				}
			})

			// 2. Test dryrun script generation and execution parity
			t.Run("dryrun", func(t *testing.T) {
				tmpDir := t.TempDir()
				for filename, content := range ex.SetupFiles {
					p := filepath.Join(tmpDir, filename)
					require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
					require.NoError(t, os.WriteFile(p, []byte(content), 0644))
				}

				// If command already has -n / --dry-run, run directly
				dryArgs := cmdArgs
				if !strings.Contains(cmdArgs, "-n") && !strings.Contains(cmdArgs, "--dry-run") {
					dryArgs = "-n " + cmdArgs
				}

				var scriptBuf, scriptErr bytes.Buffer
				cDry := exec.Command("bash", "-c", bin+" "+dryArgs)
				cDry.Dir = tmpDir
				cDry.Stdout = &scriptBuf
				cDry.Stderr = &scriptErr
				require.NoError(t, cDry.Run(), "dryrun generation failed: %s", scriptErr.String())

				script := scriptBuf.String()
				assert.Contains(t, script, "#!/usr/bin/env bash")

				scriptFile := filepath.Join(tmpDir, "generated.sh")
				require.NoError(t, os.WriteFile(scriptFile, []byte(script), 0755))

				// Run generated script to verify execution parity
				var stdout, stderr bytes.Buffer
				cScript := exec.Command("bash", scriptFile)
				cScript.Dir = tmpDir
				cScript.Stdout = &stdout
				cScript.Stderr = &stderr
				if ex.Stdin != "" {
					cScript.Stdin = bytes.NewBufferString(ex.Stdin)
				}

				err := cScript.Run()
				if ex.WantStatus == 0 {
					assert.NoError(t, err, "dryrun script failed: %s", stderr.String())
				} else {
					var exitErr *exec.ExitError
					require.True(t, errors.As(err, &exitErr), "dryrun script expected exit error, got %v", err)
					assert.Equal(t, ex.WantStatus, exitErr.ExitCode(), "dryrun script stderr: %s", stderr.String())
				}
			})
		})
	}
}
