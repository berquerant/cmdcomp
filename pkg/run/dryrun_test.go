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
		// ---- multiple hooks / preprocesses / interceptors ----
		{
			name: "multiple startup hooks all appear in index order",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Startup = []string{"echo s0", "echo s1", "echo s2"}
			},
			contains: []string{
				"# startup[0]", "echo s0",
				"# startup[1]", "echo s1",
				"# startup[2]", "echo s2",
			},
			order: [][2]string{
				{"# startup[0]", "# startup[1]"},
				{"# startup[1]", "# startup[2]"},
				{"# startup[2]", "# left"},
			},
		},
		{
			name: "multiple cleanup hooks all appear in index order after diff",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Cleanup = []string{"echo c0", "echo c1", "echo c2"}
			},
			contains: []string{
				"# cleanup[0]", "echo c0",
				"# cleanup[1]", "echo c1",
				"# cleanup[2]", "echo c2",
			},
			order: [][2]string{
				{"# diff", "# cleanup[0]"},
				{"# cleanup[0]", "# cleanup[1]"},
				{"# cleanup[1]", "# cleanup[2]"},
			},
		},
		{
			name: "multiple interceptors all appear between left and right",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Interceptor = []string{"echo i0", "echo i1", "echo i2"}
			},
			contains: []string{
				"# interceptor[0]", "echo i0",
				"# interceptor[1]", "echo i1",
				"# interceptor[2]", "echo i2",
			},
			order: [][2]string{
				{"# left", "# interceptor[0]"},
				{"# interceptor[0]", "# interceptor[1]"},
				{"# interceptor[1]", "# interceptor[2]"},
				{"# interceptor[2]", "# right"},
			},
		},
		{
			name: "multiple preprocess commands form a single pipeline per side",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Preprocess = []string{`sed 's|a|x|'`, `sed 's|x|y|'`, "cat"}
			},
			contains: []string{
				`sed 's|a|x|'`,
				`| sed 's|x|y|'`,
				"| cat",
				// all three commands appear on one pipeline line per side
				"# preprocess:left",
				"# preprocess:right",
			},
		},
		{
			name: "independent leftPreprocess and rightPreprocess pipelines",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.LeftPreprocess = []string{"tr a A", "tr A Z"}
				c.RightPreprocess = []string{"tr b B", "tr B Y"}
			},
			contains: []string{
				// left pipeline
				"# preprocess:left", "tr a A", "| tr A Z",
				// right pipeline
				"# preprocess:right", "tr b B", "| tr B Y",
			},
		},
		{
			name: "common preprocess prepended before side-specific preprocess",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Preprocess = []string{"cat"}
				c.LeftPreprocess = []string{"tr a A"}
				c.RightPreprocess = []string{"tr b B"}
			},
			contains: []string{
				// left: common then left-specific
				"# preprocess:left",
				"cat < \"$_CMDCOMP_LEFT\" | tr a A",
				// right: common then right-specific
				"# preprocess:right",
				"cat < \"$_CMDCOMP_RIGHT\" | tr b B",
			},
		},
		{
			name: "all hooks and preprocesses combined",
			args: []string{"echo", "--", "a", "--", "b"},
			modify: func(c *config.Config) {
				c.Startup = []string{"echo s0", "echo s1"}
				c.Interceptor = []string{"echo i0", "echo i1"}
				c.Cleanup = []string{"echo c0", "echo c1"}
				c.Preprocess = []string{`sed 's|a|x|'`}
				c.LeftPreprocess = []string{"tr x L"}
				c.RightPreprocess = []string{"tr x R"}
			},
			contains: []string{
				"# startup[0]", "echo s0",
				"# startup[1]", "echo s1",
				"# left",
				"# interceptor[0]", "echo i0",
				"# interceptor[1]", "echo i1",
				"# right",
				"# preprocess:left", "tr x L",
				"# preprocess:right", "tr x R",
				"# diff",
				"# cleanup[0]", "echo c0",
				"# cleanup[1]", "echo c1",
			},
			order: [][2]string{
				{"# startup[1]", "# left"},
				{"# left", "# interceptor[0]"},
				{"# interceptor[1]", "# right"},
				{"# diff", "# cleanup[0]"},
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
