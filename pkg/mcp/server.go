package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/berquerant/cmdcomp/pkg/cli"
	"github.com/berquerant/cmdcomp/pkg/config"
	"github.com/berquerant/cmdcomp/pkg/run"
	"github.com/berquerant/cmdcomp/version"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server encapsulates the MCP server for cmdcomp.
type Server struct {
	server *sdk.Server
	config *config.Config
}

type DiffInput struct {
	CommonArgs       []string `json:"common_args,omitempty" jsonschema:"Common arguments prepended to both left and right commands"`
	LeftArgs         []string `json:"left_args,omitempty" jsonschema:"Arguments specific to the left command"`
	RightArgs        []string `json:"right_args,omitempty" jsonschema:"Arguments specific to the right command"`
	Diff             string   `json:"diff,omitempty" jsonschema:"Diff command (e.g. 'diff -u', 'dyff')"`
	Shell            string   `json:"shell,omitempty" jsonschema:"Shell executable used to run subcommands (default: bash)"`
	Startup          []string `json:"startup,omitempty" jsonschema:"Commands executed before left and right commands"`
	Interceptor      []string `json:"interceptor,omitempty" jsonschema:"Commands executed between left and right commands"`
	Preprocess       []string `json:"preprocess,omitempty" jsonschema:"Filter pipeline applied to both left and right outputs"`
	LeftPreprocess   []string `json:"left_preprocess,omitempty" jsonschema:"Filter pipeline applied only to left output"`
	RightPreprocess  []string `json:"right_preprocess,omitempty" jsonschema:"Filter pipeline applied only to right output"`
	Cleanup          []string `json:"cleanup,omitempty" jsonschema:"Teardown commands executed on exit"`
	Env              []string `json:"env,omitempty" jsonschema:"Environment variables for all commands (KEY=VALUE)"`
	LeftEnv          []string `json:"left_env,omitempty" jsonschema:"Environment variables for left command only"`
	RightEnv         []string `json:"right_env,omitempty" jsonschema:"Environment variables for right command only"`
	StdinContent     string   `json:"stdin_content,omitempty" jsonschema:"Literal string passed to stdin of both commands"`
	LeftStdin        string   `json:"left_stdin,omitempty" jsonschema:"Input source for left stdin ('-' or '@filename')"`
	RightStdin       string   `json:"right_stdin,omitempty" jsonschema:"Input source for right stdin ('-' or '@filename')"`
	Snapshot         string   `json:"snapshot,omitempty" jsonschema:"Static snapshot source for both commands ('-' or '@filename')"`
	LeftSnapshot     string   `json:"left_snapshot,omitempty" jsonschema:"Static snapshot source for left command ('-' or '@filename')"`
	RightSnapshot    string   `json:"right_snapshot,omitempty" jsonschema:"Static snapshot source for right command ('-' or '@filename')"`
	UseLabel         bool     `json:"label,omitempty" jsonschema:"Pass --label arguments to diff command"`
	Success          bool     `json:"success,omitempty" jsonschema:"Exit with 0 even when diffs are detected"`
	WorkDir          string   `json:"work_dir,omitempty" jsonschema:"Working directory for temporary output files"`
	TimeoutMs        int64    `json:"timeout_ms,omitempty" jsonschema:"Timeout in milliseconds for the entire execution"`
	ProcessTimeoutMs int64    `json:"process_timeout_ms,omitempty" jsonschema:"Timeout in milliseconds for each subcommand"`
	Presets          []string `json:"presets,omitempty" jsonschema:"Preset names to apply (e.g. 'json', 'yaml', 'dyff')"`
}

type DiffOutput struct {
	Stdout   string `json:"stdout" jsonschema:"Standard output containing diff results"`
	ExitCode int    `json:"exit_code" jsonschema:"Exit code of execution (0: no diff/success, 1: diff detected, 2: failure)"`
	HasDiff  bool   `json:"has_diff" jsonschema:"True if diff was detected between outputs"`
	Error    string `json:"error,omitempty" jsonschema:"Error message if execution failed"`
}

type DryRunInput struct {
	DiffInput
}

type DryRunOutput struct {
	Script string `json:"script" jsonschema:"Generated bash script capturing the full execution pipeline"`
	Error  string `json:"error,omitempty" jsonschema:"Error message if script generation failed"`
}

type ListPresetsInput struct{}

type PresetInfo struct {
	Name       string   `json:"name" jsonschema:"Preset identifier"`
	Diff       string   `json:"diff,omitempty" jsonschema:"Configured diff command"`
	Preprocess []string `json:"preprocess,omitempty" jsonschema:"Preprocess filters"`
	CommonArgs []string `json:"common_args,omitempty" jsonschema:"Common args"`
	LeftArgs   []string `json:"left_args,omitempty" jsonschema:"Left args"`
	RightArgs  []string `json:"right_args,omitempty" jsonschema:"Right args"`
}

type ListPresetsOutput struct {
	Presets []PresetInfo `json:"presets" jsonschema:"List of available presets"`
}

// NewServer creates a new cmdcomp MCP server.
func NewServer(baseConfig *config.Config) *Server {
	impl := &sdk.Implementation{
		Name:    "cmdcomp",
		Version: version.Version,
	}
	s := sdk.NewServer(impl, nil)
	srv := &Server{
		server: s,
		config: baseConfig,
	}
	srv.registerTools()
	return srv
}

func (s *Server) registerTools() {
	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "cmdcomp_diff",
		Description: "Compare the stdout of two commands or snapshots with optional preprocessing and customizable diff tool.",
	}, s.handleDiff)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "cmdcomp_dryrun",
		Description: "Generate an executable bash script capturing the full execution pipeline without executing commands.",
	}, s.handleDryRun)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "cmdcomp_list_presets",
		Description: "List available presets from built-in configurations and config files.",
	}, s.handleListPresets)
}

func (s *Server) handleDiff(ctx context.Context, req *sdk.CallToolRequest, in DiffInput) (*sdk.CallToolResult, DiffOutput, error) {
	cfg, buf, err := s.buildConfig(in, false)
	if err != nil {
		out := DiffOutput{
			ExitCode: 2,
			Error:    err.Error(),
		}
		return nil, out, nil
	}

	execErr := run.Main(cfg)
	out := DiffOutput{
		Stdout: buf.String(),
	}

	if execErr != nil {
		if errors.Is(execErr, run.ErrDiff) {
			out.HasDiff = true
			if exitErr, ok := errors.AsType[*exec.ExitError](execErr); ok {
				if cfg.Success && exitErr.ExitCode() == 1 {
					out.ExitCode = 0
				} else {
					out.ExitCode = exitErr.ExitCode()
				}
			} else {
				out.ExitCode = 1
			}
		} else {
			out.ExitCode = 2
			out.Error = execErr.Error()
		}
	} else {
		out.ExitCode = 0
		out.HasDiff = false
	}

	return nil, out, nil
}

func (s *Server) handleDryRun(ctx context.Context, req *sdk.CallToolRequest, in DryRunInput) (*sdk.CallToolResult, DryRunOutput, error) {
	cfg, buf, err := s.buildConfig(in.DiffInput, true)
	if err != nil {
		return nil, DryRunOutput{Error: err.Error()}, nil
	}

	if err := run.Main(cfg); err != nil {
		return nil, DryRunOutput{Error: err.Error()}, nil
	}

	return nil, DryRunOutput{
		Script: buf.String(),
	}, nil
}

func (s *Server) handleListPresets(ctx context.Context, req *sdk.CallToolRequest, in ListPresetsInput) (*sdk.CallToolResult, ListPresetsOutput, error) {
	configPath := ""
	if s.config != nil {
		configPath = s.config.ConfigPath
	}
	cs, err := cli.LoadConfigSet(configPath)
	if err != nil {
		return nil, ListPresetsOutput{}, fmt.Errorf("failed to load presets: %w", err)
	}

	var presets []PresetInfo
	for name, p := range cs.Presets {
		presets = append(presets, PresetInfo{
			Name:        name,
			Diff:        p.Diff,
			Preprocess:  p.Preprocess,
			CommonArgs:  p.CommonArgs,
			LeftArgs:    p.LeftArgs,
			RightArgs:   p.RightArgs,
		})
	}
	return nil, ListPresetsOutput{Presets: presets}, nil
}

func (s *Server) buildConfig(in DiffInput, dryRun bool) (*config.Config, *bytes.Buffer, error) {
	configPath := ""
	if s.config != nil {
		configPath = s.config.ConfigPath
	}

	cs, err := cli.LoadConfigSet(configPath)
	if err != nil {
		return nil, nil, err
	}

	c := &config.Config{
		Shell:     "bash",
		Delimiter: "--",
		Diff:      "diff",
		DryRun:    dryRun,
	}

	// Apply presets
	for _, p := range in.Presets {
		presetCfg, ok := cs.Find(p)
		if !ok {
			return nil, nil, fmt.Errorf("preset not found: %s", p)
		}
		if presetCfg.Diff != "" {
			c.Diff = presetCfg.Diff
		}
		if presetCfg.Shell != "" {
			c.Shell = presetCfg.Shell
		}
		if len(presetCfg.Preprocess) > 0 {
			c.Preprocess = append(c.Preprocess, presetCfg.Preprocess...)
		}
		if len(presetCfg.LeftPreprocess) > 0 {
			c.LeftPreprocess = append(c.LeftPreprocess, presetCfg.LeftPreprocess...)
		}
		if len(presetCfg.RightPreprocess) > 0 {
			c.RightPreprocess = append(c.RightPreprocess, presetCfg.RightPreprocess...)
		}
		if len(presetCfg.Startup) > 0 {
			c.Startup = append(c.Startup, presetCfg.Startup...)
		}
		if len(presetCfg.Interceptor) > 0 {
			c.Interceptor = append(c.Interceptor, presetCfg.Interceptor...)
		}
		if len(presetCfg.Cleanup) > 0 {
			c.Cleanup = append(c.Cleanup, presetCfg.Cleanup...)
		}
		if len(presetCfg.Env) > 0 {
			c.Env = append(c.Env, presetCfg.Env...)
		}
		if len(presetCfg.LeftEnv) > 0 {
			c.LeftEnv = append(c.LeftEnv, presetCfg.LeftEnv...)
		}
		if len(presetCfg.RightEnv) > 0 {
			c.RightEnv = append(c.RightEnv, presetCfg.RightEnv...)
		}
		if len(presetCfg.CommonArgs) > 0 {
			c.CommonArgs = append(c.CommonArgs, presetCfg.CommonArgs...)
		}
		if len(presetCfg.LeftArgs) > 0 {
			c.LeftArgs = append(c.LeftArgs, presetCfg.LeftArgs...)
		}
		if len(presetCfg.RightArgs) > 0 {
			c.RightArgs = append(c.RightArgs, presetCfg.RightArgs...)
		}
	}

	// Apply tool inputs
	if in.Diff != "" {
		c.Diff = in.Diff
	}
	if in.Shell != "" {
		c.Shell = in.Shell
	}
	if in.UseLabel {
		c.UseLabel = true
	}
	if in.Success {
		c.Success = true
	}
	if in.WorkDir != "" {
		c.WorkDir = in.WorkDir
	}
	if in.TimeoutMs > 0 {
		c.Timeout = time.Duration(in.TimeoutMs) * time.Millisecond
	}
	if in.ProcessTimeoutMs > 0 {
		c.ProcessTimeout = time.Duration(in.ProcessTimeoutMs) * time.Millisecond
	}

	if in.LeftStdin != "" {
		c.LeftStdin = in.LeftStdin
	}
	if in.RightStdin != "" {
		c.RightStdin = in.RightStdin
	}
	if in.Snapshot != "" {
		c.Snapshot = in.Snapshot
	}
	if in.LeftSnapshot != "" {
		c.LeftSnapshot = in.LeftSnapshot
	}
	if in.RightSnapshot != "" {
		c.RightSnapshot = in.RightSnapshot
	}

	c.Startup = append(c.Startup, in.Startup...)
	c.Interceptor = append(c.Interceptor, in.Interceptor...)
	c.Preprocess = append(c.Preprocess, in.Preprocess...)
	c.LeftPreprocess = append(c.LeftPreprocess, in.LeftPreprocess...)
	c.RightPreprocess = append(c.RightPreprocess, in.RightPreprocess...)
	c.Cleanup = append(c.Cleanup, in.Cleanup...)
	c.Env = append(c.Env, in.Env...)
	c.LeftEnv = append(c.LeftEnv, in.LeftEnv...)
	c.RightEnv = append(c.RightEnv, in.RightEnv...)

	c.CommonArgs = append(c.CommonArgs, in.CommonArgs...)
	c.LeftArgs = append(c.LeftArgs, in.LeftArgs...)
	c.RightArgs = append(c.RightArgs, in.RightArgs...)

	var buf bytes.Buffer
	c.Writer = &buf

	if in.StdinContent != "" {
		c.Reader = strings.NewReader(in.StdinContent)
		if c.Stdin == "" && c.LeftStdin == "" && c.RightStdin == "" && c.Snapshot == "" && c.LeftSnapshot == "" && c.RightSnapshot == "" {
			c.Stdin = "-"
		}
	}

	if err := c.Init(nil); err != nil {
		return nil, nil, err
	}

	return c, &buf, nil
}

// Serve runs the MCP server on stdio.
func (s *Server) Serve(ctx context.Context) error {
	transport := &sdk.StdioTransport{}
	return s.ServeWithTransport(ctx, transport)
}

// ServeWithTransport runs the MCP server using the specified transport.
func (s *Server) ServeWithTransport(ctx context.Context, t sdk.Transport) error {
	return s.server.Run(ctx, t)
}
