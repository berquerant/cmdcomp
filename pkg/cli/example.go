package cli

import (
	"fmt"
	"strings"
)

// Example represents a documented, executable CLI usage example.
type Example struct {
	Title       string
	Description string
	ShellDoc    string            // optional shell comments explaining the workflow
	Command     string            // executable cmdcomp command string (e.g. "cmdcomp -P u -- echo -- a -- b")
	SetupFiles  map[string]string // files created in working dir before execution
	Stdin       string            // stdin input piped into cmdcomp
	WantStatus  int               // expected exit status (0 or 1)
}

// BuiltinExamples is the single source of truth for all documented CLI examples in usage & README.
var BuiltinExamples = []Example{
	{
		Title:       "Basic Comparison",
		Description: "Compare the standard output of two commands:",
		ShellDoc: `# Equivalent shell workflow:
# echo a > leftfile
# echo b > rightfile
# diff leftfile rightfile`,
		Command:    "cmdcomp -- echo -- a -- b",
		WantStatus: 1,
	},
	{
		Title:       "Custom Diff Tool & Label",
		Description: "Use a customized diff tool (e.g. 'diff -u', 'colordiff') and pass argument labels with '-l' / '--label':",
		ShellDoc:   `# Unified diff with labels:`,
		Command:    "cmdcomp -x 'diff -u' -l -- echo -- a -- b",
		WantStatus: 1,
	},
	{
		Title:       "Common Preprocess Pipeline",
		Description: "Apply filter pipelines (e.g. jq, yq, sed) to both outputs before diffing:",
		ShellDoc:   `# Filter both command outputs through sed:`,
		Command:    `cmdcomp -p 'sed "s|a|c|"' -- echo -- a -- b`,
		WantStatus: 1,
	},
	{
		Title:       "Asymmetric Preprocessing (Left & Right)",
		Description: "Apply specific preprocess filters only to the left or right command output in addition to common filters:",
		ShellDoc:   `# Replace strings differently on left vs right side:`,
		Command:    `cmdcomp --left-preprocess 'sed "s|a|c|"' --right-preprocess 'sed "s|a|d|"' -- echo -- a -- a`,
		WantStatus: 1,
	},
	{
		Title:       "Startup Hooks & Advanced Preprocessing",
		Description: "Run setup commands sequentially before running the compare commands:",
		ShellDoc:   `# Run startup hook before comparing filtered outputs:`,
		Command:    `cmdcomp -s 'echo "starting setup"' -p 'sed "s|foo|bar|"' -x 'diff -u' -- echo -- foo1 -- foo2`,
		WantStatus: 1,
	},
	{
		Title:       "Sequential Execution with Interceptor",
		Description: "Execute left command first, run interceptor hooks (e.g. git checkout, state update), then run right command:",
		ShellDoc:   `# Run interceptor hook between left and right commands:`,
		Command:    `cmdcomp -i 'echo "state changed"' -x 'diff -u' -- echo -- left -- right`,
		WantStatus: 1,
	},
	{
		Title:       "Cleanup Hooks & Working Directory",
		Description: "Ensure teardown hooks run on exit and optionally preserve temporary files in a specified directory:",
		ShellDoc:   `# Preserve temp files and clean up resources:`,
		SetupFiles: map[string]string{
			"tmp-workdir/.keep": "",
		},
		Command:    `cmdcomp --work-dir ./tmp-workdir -c 'echo "cleanup done"' -- echo -- a -- b`,
		WantStatus: 1,
	},
	{
		Title:       "Environment Variables (Common, Left, Right)",
		Description: "Pass environment variables to all commands or exclusively to the left or right side:",
		ShellDoc:   `# Inject environment variables per side:`,
		Command:    `cmdcomp -e 'COMMON=1' --left-env 'TARGET=left' --right-env 'TARGET=right' -- bash -c 'echo "$COMMON:$TARGET"'`,
		WantStatus: 1,
	},
	{
		Title:       "Replicating Standard Input (--stdin)",
		Description: "Replicate input from standard input ('-') or a file ('@filename') into the stdin of subcommands:",
		ShellDoc: `# Pass shared stdin to subcommands:
# echo -e "alpha\nbeta" | cmdcomp -I - -- grep -- alpha -- beta

# Pass input from file to left command and stdin to right command:`,
		SetupFiles: map[string]string{
			"data.txt": "alpha\n",
		},
		Stdin:      "beta\n",
		Command:    `cmdcomp --left-stdin '@data.txt' --right-stdin '-' -- cat`,
		WantStatus: 1,
	},
	{
		Title:       "Snapshot Comparison (--snapshot)",
		Description: "Bypass command execution on one or both sides and compare directly against static files or stdin:",
		ShellDoc: `# Compare a pre-generated baseline snapshot file against live command output:
# cmdcomp --left-snapshot '@baseline.txt' -p 'sed "s|x|y|"' -- echo -- live

# Compare two static snapshot files through common preprocess filters without commands:`,
		SetupFiles: map[string]string{
			"baseline.txt": "live\n",
			"file1.json":   "{\"val\": 10}\n",
			"file2.json":   "{\"val\": 20}\n",
		},
		Command:    `cmdcomp --left-snapshot '@file1.json' --right-snapshot '@file2.json' -p 'sed "s|10|20|"'`,
		WantStatus: 0,
	},
	{
		Title:       "Dry Run Mode (--dry-run)",
		Description: "Generate an executable bash script capturing the exact execution pipeline without running any commands:",
		ShellDoc:   `# Output shell script for inspection or reproduction:`,
		Command:    `cmdcomp -n -x 'diff -u' -p 'sed "s|v1|common|"' -- echo -- api-v1 -- echo -- api-v2`,
		WantStatus: 0,
	},
	{
		Title:       "Exit Code & Success Override (--success)",
		Description: "Exit with 0 even when diffs are detected (useful for CI summary steps without failing build):",
		ShellDoc:   `# Return exit code 0 on diff:`,
		Command:    "cmdcomp --success -- echo -- a -- b",
		WantStatus: 0,
	},
	{
		Title:       "Custom Delimiter",
		Description: "Change the argument delimiter from '--' to another token:",
		ShellDoc:   `# Use '---' as delimiter when subcommands themselves take '--':`,
		Command:    "cmdcomp --delimiter '---' -- echo --- echo -- a --- echo -- b",
		WantStatus: 1,
	},
}

// RenderExamples formats BuiltinExamples into markdown sections for usage and README.
func (u UsageBuilder) renderExamples() string {
	var sb strings.Builder
	for i, ex := range BuiltinExamples {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", ex.Title, ex.Description))
		var codeBlock strings.Builder
		if ex.ShellDoc != "" {
			codeBlock.WriteString(ex.ShellDoc)
			codeBlock.WriteString("\n")
		}
		codeBlock.WriteString(ex.Command)
		sb.WriteString(u.code("shell", codeBlock.String()))
	}
	return sb.String()
}
