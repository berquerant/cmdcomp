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
3. **Configuration & Environment Variables**:
   - `pkg/config/config.go` (`config.Config`) is the **single source of truth** for all runtime and CLI configuration options.
   - CLI flags are in kebab-case (`--show-cmd-log`, `--dry-run`, `--left-preprocess`, etc.).
   - All options can be configured via environment variables with prefix `CMDCOMP_` (e.g. `CMDCOMP_DIFF`, `CMDCOMP_SHOW_CMD_LOG`).
   - Configuration Precedence: **Default/Preset < Environment Variables (`CMDCOMP_*`) < CLI Flags**.
   - Slices for commands (`startup`, `interceptor`, `preprocess`, `cleanup`) use `sep:"\n"` for newline separation in env vars. Slices for env vars (`env`, `left-env`, `right-env`) use `sep:","`.
4. **Exit Code Conventions**:
   - `0`: No diff detected, `--dry-run`, `--version`/`--help`, or diff detected with `--success`.
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

- **Table-Driven Tests**: Write test codes using table-driven tests unless there is a specific reason not to.
- **Error Identification**: When adding new execution phases or modifying process spawning, ensure errors are wrapped with clear phase descriptions so users know exactly which command failed or timed out.
- **Concurrency & Resource Safety**: Ensure temporary directories and open file handles are cleanly closed, and `defer` cleanup hooks are always run.

---

## 6. Managing Generated Files & Handling Diffs

`make lint` runs `git diff --exit-code` on `NOTICE` and `README.md`. Therefore, when these files or golden files change, you **MUST review the generated diff and `git add` them**; otherwise, `make lint` will fail.

### 1. `NOTICE` (Third-Party Licenses)
- **Trigger**: Added, upgraded, or removed a Go module in `go.mod`.
- **How to update**:
  ```bash
  ./hack/license.sh report > NOTICE
  ```
  *(Requires network access to fetch upstream licenses)*.
- **Next steps**: Review `git diff NOTICE`, and run `git add NOTICE`.

### 2. `pkg/cli/testdata/usage.golden` (CLI Usage & Help Text)
- **Trigger**: Added/renamed flags, updated descriptions/examples, or changed `pkg/cli/usage.go`.
- **How to update**:
  ```bash
  make golden
  ```
- **Next steps**: Review `git diff pkg/cli/testdata/usage.golden`, and run `git add pkg/cli/testdata/usage.golden`.

### 3. `README.md` (Generated Documentation)
- **Trigger**: CLI flags, usage outputs, or built-in presets have changed.
- **How to update**:
  ```bash
  make README.md
  ```
- **Next steps**: Review `git diff README.md`, and run `git add README.md`.

### Complete Update & Verification Workflow
When modifying flags or dependencies:
```bash
# 1. Regenerate
make golden
make README.md
./hack/license.sh report > NOTICE

# 2. Review diffs and stage
git diff
git add pkg/cli/testdata/usage.golden README.md NOTICE

# 3. Verify
make lint
make test
```

---

## 7. Post-Verification Review Checklist (After `test` and `lint` Pass)

After code changes are completed and `make lint` / `make test` succeed, you **MUST** perform the following review and cleanup steps before finalizing work:

1. **Goal Alignment & Regression Check**:
   - Review the entire diff against the baseline branch (`main`).
   - Check if the changes accurately achieve the requested goals.
   - Check whether any existing behavior was broken or modified unintentionally (if modified, verify that the breaking change was explicitly intended). If inappropriate, fix it.
2. **Readability & Maintainability Review**:
   - Review the overall diff for code clarity, duplicate logic (DRY), or awkward constructs that could hinder future maintenance.
   - Clarify intent with concise comments where necessary or refactor redundant code.
3. **Documentation & Spec Synchronization**:
   - Check if `README.md`, CLI usage / help text (`pkg/cli/usage.go`, `usage.golden`), and `AGENTS.md` are completely synchronized with the latest codebase state.
   - If missing information or outdated examples are found, update and regenerate them immediately.

