# AGENTS.md

This document provides development instructions, project structure, architectural principles, and testing guidelines for AI coding agents and developers working on `cmdcomp`.

---

## 1. Project Overview

`cmdcomp` is a Go CLI tool designed to compare the stdout of two commands with optional preprocessing pipelines and customizable diff commands.

### Key Capabilities
- **Argument Delimitation**: Uses delimiters (`--` by default) to split `COMMON_ARGS`, `LEFT_ARGS`, and `RIGHT_ARGS`.
- **Lifecycle Hooks**:
  - `startup`: Pre-run setup hooks (e.g. repo updates).
  - `interceptor`: Hook run sequentially between the left command and the right command (useful for state changes like git checkouts).
  - `preprocess` / `leftPreprocess` / `rightPreprocess`: Piped filters (e.g. `jq`, `yq`, `sed`) applied to outputs before diffing.
  - `diff`: Pluggable diff tool (`diff`, `colordiff`, `dyff`, `objdiff`, etc.).
  - `cleanup`: Guaranteed teardown hooks executed even on failure.
- **Dual Execution Modes**:
  - **Real Execution**: Runs subcommands via shell/exec pipelines.
  - **Dry-run (`--dryrun`)**: Generates an executable shell script capturing the exact pipeline.

---

## 2. Directory & Architecture Structure

```
.
├── cmd/
│   └── cmdcomp/
│       ├── main.go          # CLI entry point, exit code handling
│       └── main_test.go     # End-to-end (E2E) tests with built binary
├── pkg/
│   ├── cli/
│   │   ├── builtin.go       # Built-in config presets (helm, k8s, json, yml, etc.)
│   │   ├── config.go        # Config loading & merging (preset, file, flags)
│   │   ├── flag.go          # CLI flag parsing (pflag)
│   │   ├── usage.go         # Help text & UsageBuilder
│   │   ├── usage_test.go    # Golden tests for usage output
│   │   └── testdata/        # Golden files (usage.golden)
│   ├── config/
│   │   └── config.go        # Core runtime Config struct & environment variable expansion
│   ├── execx/
│   │   └── exec.go          # Command execution & piped command runner abstraction
│   ├── run/
│   │   ├── executor.go      # Executor interface (RunHook, RunGenCmd, RunPipeline, RunDiff)
│   │   ├── real_executor.go # Real execution implementation
│   │   ├── dryrun_executor.go # Shell script generator implementation
│   │   ├── main.go          # Lifecycle orchestrator (runner) & phase error sentinels
│   │   ├── main_test.go     # Integration / pipeline unit tests
│   │   └── dryrun_test.go   # Dry-run script output tests
│   └── slicex/
│       └── slice.go         # Slice split helper utilities
├── hack/
│   ├── license.sh           # Third-party license checker script
│   └── readme.sh            # README.md generator script using bin/cmdcomp --help
├── Makefile                 # Build, test, lint, golden, and readme targets
└── README.md                # Generated documentation (do not edit manually without make README.md)
```

---

## 3. Design & Architecture Principles

1. **`Executor` Interface Pattern**:
   - All external command execution **MUST** go through the `Executor` interface ([pkg/run/executor.go](pkg/run/executor.go)).
   - `realExecutor` and `DryRunExecutor` implement the same interface. This ensures dry-run scripts and real executions never drift out of sync.
2. **Sentinel Errors & Phase Context**:
   - Execution errors are tagged with sentinel errors (`run.ErrDiff`, `run.ErrHook`, `run.ErrGenCmd`, `run.ErrPipeline`) and wrapped with the phase name (e.g. `run left`, `run right`, `run startup[0]`, `run preprocess:left pipeline`).
3. **Exit Code Conventions**:
   - `0`: No diff detected, `--dryrun`, `--version`/`--help`, or diff detected with `--success`.
   - `1`: Diff detected (diff command exited with 1).
   - `2`: Process failure (command failure, hook failure, pipeline failure, timeout, flag/config errors). Always exits with `2` even if `--success` is specified.

---

## 4. Development Workflow & Commands

Always verify changes using the Makefile targets:

```bash
# Run all unit and integration tests
make test

# Update golden files when usage/help text changes
make golden

# Generate README.md from the built binary's help output
make README.md

# Run full lint checks (licenses, check-readme, vet, go-fix)
make lint

# Build binary
make bin/cmdcomp
```

---

## 5. Coding & Testing Guidelines

- **Golden Tests**: When modifying CLI flags or usage texts, run `make golden` and `make README.md` to keep `usage.golden` and `README.md` in sync.
- **Error Identification**: When adding new execution phases or modifying process spawning, ensure errors are wrapped with clear phase descriptions so users know exactly which command failed or timed out.
- **Concurrency & Resource Safety**: Ensure temporary directories and open file handles are cleanly closed, and `defer` cleanup hooks are always run.
