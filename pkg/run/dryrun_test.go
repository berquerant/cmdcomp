package run_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/berquerant/cmdcomp/pkg/run"
	"github.com/stretchr/testify/assert"
)

func TestDryRun(t *testing.T) {
	newConfig := func(t *testing.T, args []string, modify func(*config.Config)) (*config.Config, *bytes.Buffer) {
		t.Helper()
		var stdout bytes.Buffer
		c := &config.Config{
			DryRun:    true,
			Shell:     "bash",
			Delimiter: "--",
			Diff:      "diff",
			Writer:    &stdout,
		}
		modify(c)
		c.SetupLogger(os.Stderr)
		if err := c.Init(args); err != nil {
			t.Fatalf("Init: %v", err)
		}
		return c, &stdout
	}

	for _, tc := range []struct {
		name   string
		args   []string
		modify func(*config.Config)
		// each string in contains must be a substring of the output
		contains []string
		// ordered pairs [before, after]: before must appear earlier in output than after
		order [][2]string
		// if true, TempDir must be empty after Main
		noTempDir bool
	}{
		{
			name: "preamble",
			args: []string{"echo", "--", "a", "--", "b"},
			contains: []string{
				"#!/usr/bin/env bash\n",
				"set -euo pipefail",
				"_CMDCOMP_TMPDIR=$(mktemp -d)",
				`trap 'rm -rf "$_CMDCOMP_TMPDIR"' EXIT`,
			},
		},
		{
			name: "left and right gen commands",
			args: []string{"echo", "--", "hello", "--", "world"},
			contains: []string{
				"# left", "_CMDCOMP_LEFT=", "echo hello",
				"# right", "_CMDCOMP_RIGHT=", "echo world",
			},
		},
		{
			name: "customized diff command",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Diff = "diff -u"
			},
			contains: []string{"# diff", "diff -u"},
		},
		{
			name: "preprocess pipeline",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Preprocess = []string{`sed 's|a|c|'`, "cat"}
			},
			contains: []string{
				"# preprocess:left", "_CMDCOMP_PREPROCESS_LEFT=",
				"# preprocess:right", "_CMDCOMP_PREPROCESS_RIGHT=",
				`sed 's|a|c|'`,
				`< "$_CMDCOMP_LEFT"`,  // input redirect on first cmd
				"| cat",               // pipe to second cmd
			},
		},
		{
			name: "left-only preprocess: right diff ref stays raw",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.LeftPreprocess = []string{"grep x"}
			},
			contains: []string{
				"# preprocess:left", "grep x",
				`"$_CMDCOMP_RIGHT"`, // right skips preprocess, diff uses raw var
			},
		},
		{
			name: "startup hooks order",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Startup = []string{"echo startup1", "echo startup2"}
			},
			contains: []string{
				"# startup[0]", "echo startup1",
				"# startup[1]", "echo startup2",
			},
			order: [][2]string{
				{"# startup[0]", "# left"},
			},
		},
		{
			name: "cleanup hooks order",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Cleanup = []string{"echo cleanup1"}
			},
			contains: []string{"# cleanup[0]", "echo cleanup1"},
			order: [][2]string{
				{"# diff", "# cleanup[0]"},
			},
		},
		{
			name: "interceptor between left and right",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Interceptor = []string{"echo interceptor1"}
			},
			contains: []string{"# interceptor[0]", "echo interceptor1"},
			order: [][2]string{
				{"# left", "# interceptor[0]"},
				{"# interceptor[0]", "# right"},
			},
		},
		{
			name: "label option adds --label flags to diff line",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.UseLabel = true
				c.Diff = "diff -u"
			},
			contains: []string{"--label", "echo___a", "echo___b"},
		},
		{
			name: "env vars appear as inline prefix",
			args: []string{"bash", "-c", "echo $X", "--", "", "--", ""},
			modify: func(c *config.Config) {
				c.LeftEnv = []string{"X=left"}
				c.RightEnv = []string{"X=right"}
			},
			contains: []string{"X=left", "X=right"},
		},
		{
			name:      "no tempdir created",
			args:      []string{"echo", "--", "a", "--", "b"},
			noTempDir: true,
		},
		{
			name: "diff references preprocess output vars",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Preprocess = []string{"cat"}
			},
			contains: []string{
				`"$_CMDCOMP_PREPROCESS_LEFT"`,
				`"$_CMDCOMP_PREPROCESS_RIGHT"`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			modify := tc.modify
			if modify == nil {
				modify = func(*config.Config) {}
			}
			c, stdout := newConfig(t, tc.args, modify)

			err := run.Main(c)
			assert.Nil(t, err, "dryrun must always exit successfully")

			out := stdout.String()

			for _, want := range tc.contains {
				assert.Contains(t, out, want)
			}
			for _, pair := range tc.order {
				before, after := pair[0], pair[1]
				assert.Less(t,
					strings.Index(out, before),
					strings.Index(out, after),
					"%q must appear before %q", before, after,
				)
			}
			if tc.noTempDir {
				assert.Empty(t, c.TempDir, "TempDir must not be created in dryrun mode")
			}
		})
	}
}
