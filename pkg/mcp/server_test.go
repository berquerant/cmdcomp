package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/berquerant/cmdcomp/pkg/config"
	cmdmcp "github.com/berquerant/cmdcomp/pkg/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func parseResult[T any](t *testing.T, res *sdk.CallToolResult) T {
	t.Helper()
	var out T
	if res.StructuredContent != nil {
		b, err := json.Marshal(res.StructuredContent)
		if assert.Nil(t, err) {
			_ = json.Unmarshal(b, &out)
		}
		return out
	}
	if len(res.Content) > 0 {
		if tc, ok := res.Content[0].(*sdk.TextContent); ok {
			_ = json.Unmarshal([]byte(tc.Text), &out)
		}
	}
	return out
}

func setupTestSession(t *testing.T, ctx context.Context, srv *cmdmcp.Server) *sdk.ClientSession {
	t.Helper()
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	go func() {
		_ = srv.ServeWithTransport(ctx, serverTransport)
	}()

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if !assert.Nil(t, err) {
		t.FailNow()
	}
	t.Cleanup(func() {
		_ = session.Close()
	})
	return session
}

func TestMCPServer(t *testing.T) {
	ctx := context.Background()
	srv := cmdmcp.NewServer(&config.Config{})

	t.Run("list presets", func(t *testing.T) {
		session := setupTestSession(t, ctx, srv)

		for _, tc := range []struct {
			title     string
			input     cmdmcp.ListPresetsInput
			wantNames []string
		}{
			{
				title:     "returns builtin presets",
				input:     cmdmcp.ListPresetsInput{},
				wantNames: []string{"json", "yaml", "objdiff", "dyff", "u", "uc"},
			},
		} {
			t.Run(tc.title, func(t *testing.T) {
				res, err := session.CallTool(ctx, &sdk.CallToolParams{
					Name:      "cmdcomp_list_presets",
					Arguments: tc.input,
				})
				assert.Nil(t, err)
				assert.NotNil(t, res)

				out := parseResult[cmdmcp.ListPresetsOutput](t, res)
				presetMap := make(map[string]bool)
				for _, p := range out.Presets {
					presetMap[p.Name] = true
				}
				for _, wantName := range tc.wantNames {
					assert.True(t, presetMap[wantName], "expected preset %s to be present", wantName)
				}
			})
		}
	})

	t.Run("diff tool", func(t *testing.T) {
		session := setupTestSession(t, ctx, srv)

		for _, tc := range []struct {
			title        string
			input        cmdmcp.DiffInput
			wantExitCode int
			wantHasDiff  bool
			wantStdout   string
			wantContains []string
		}{
			{
				title: "no diff",
				input: cmdmcp.DiffInput{
					CommonArgs: []string{"echo"},
					LeftArgs:   []string{"same"},
					RightArgs:  []string{"same"},
				},
				wantExitCode: 0,
				wantHasDiff:  false,
				wantStdout:   "",
			},
			{
				title: "has diff",
				input: cmdmcp.DiffInput{
					CommonArgs: []string{"echo"},
					LeftArgs:   []string{"a"},
					RightArgs:  []string{"b"},
				},
				wantExitCode: 1,
				wantHasDiff:  true,
				wantContains: []string{"1c1", "< a", "> b"},
			},
			{
				title: "with stdin literal and preprocess",
				input: cmdmcp.DiffInput{
					CommonArgs:      []string{"cat"},
					LeftPreprocess:  []string{`sed "s|hello|left|"`},
					RightPreprocess: []string{`sed "s|hello|right|"`},
					StdinContent:    "hello world\n",
				},
				wantExitCode: 1,
				wantHasDiff:  true,
				wantContains: []string{"left world", "right world"},
			},
			{
				title: "with success flag",
				input: cmdmcp.DiffInput{
					CommonArgs: []string{"echo"},
					LeftArgs:   []string{"a"},
					RightArgs:  []string{"b"},
					Success:    true,
				},
				wantExitCode: 0,
				wantHasDiff:  true,
				wantContains: []string{"1c1"},
			},
		} {
			t.Run(tc.title, func(t *testing.T) {
				res, err := session.CallTool(ctx, &sdk.CallToolParams{
					Name:      "cmdcomp_diff",
					Arguments: tc.input,
				})
				assert.Nil(t, err)
				assert.NotNil(t, res)

				out := parseResult[cmdmcp.DiffOutput](t, res)
				assert.Equal(t, tc.wantExitCode, out.ExitCode)
				assert.Equal(t, tc.wantHasDiff, out.HasDiff)
				if tc.wantStdout != "" {
					assert.Equal(t, tc.wantStdout, out.Stdout)
				}
				for _, c := range tc.wantContains {
					assert.Contains(t, out.Stdout, c)
				}
			})
		}
	})

	t.Run("dryrun tool", func(t *testing.T) {
		session := setupTestSession(t, ctx, srv)

		for _, tc := range []struct {
			title        string
			input        cmdmcp.DryRunInput
			wantContains []string
			wantErr      string
		}{
			{
				title: "generate dryrun script",
				input: cmdmcp.DryRunInput{
					DiffInput: cmdmcp.DiffInput{
						CommonArgs: []string{"echo"},
						LeftArgs:   []string{"a"},
						RightArgs:  []string{"b"},
					},
				},
				wantContains: []string{"#!/usr/bin/env bash", "echo a", "echo b", "diff "},
			},
			{
				title: "dryrun with custom diff and preprocess",
				input: cmdmcp.DryRunInput{
					DiffInput: cmdmcp.DiffInput{
						Diff:            "diff -u",
						CommonArgs:      []string{"echo"},
						LeftArgs:        []string{"a"},
						RightArgs:       []string{"b"},
						LeftPreprocess:  []string{`sed "s|a|c|"`},
						RightPreprocess: []string{`sed "s|b|d|"`},
					},
				},
				wantContains: []string{"diff -u", `sed "s|a|c|"`, `sed "s|b|d|"`},
			},
		} {
			t.Run(tc.title, func(t *testing.T) {
				res, err := session.CallTool(ctx, &sdk.CallToolParams{
					Name:      "cmdcomp_dryrun",
					Arguments: tc.input,
				})
				assert.Nil(t, err)
				assert.NotNil(t, res)

				out := parseResult[cmdmcp.DryRunOutput](t, res)
				if tc.wantErr != "" {
					assert.Contains(t, out.Error, tc.wantErr)
				} else {
					assert.Empty(t, out.Error)
				}
				for _, c := range tc.wantContains {
					assert.Contains(t, out.Script, c)
				}
			})
		}
	})
}
