package cli

func builtinConfigSet() *ConfigSet {
	cs := &ConfigSet{
		Presets: map[string]*Config{
			"json": &Config{
				Preprocess: []string{`jq -S .`},
				Diff:       "diff -u --color",
			},
			"yml": &Config{
				Preprocess: []string{`yq -P 'sort_keys(..)'`},
				Diff:       "diff -u --color",
			},
			"k8s": &Config{
				Diff: "objdiff -cv",
			},
			"k8s-clean": &Config{
				Preprocess: []string{
					`yq 'del(.metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp, .metadata.generation, .metadata.managedFields, .status)'`,
				},
				Diff: "objdiff -cv",
			},
			"helm": &Config{
				Diff: "objdiff -cv",
				Startup: []string{
					`helm repo update`,
				},
			},
			"helm-chart": &Config{
				Diff: "diff -u --color",
				Startup: []string{
					`helm repo update`,
				},
				Preprocess: []string{`yq -P 'sort_keys(..)'`},
				CommonArgs: []string{
					"helm", "show", "chart", "$CHART",
				},
				LeftArgs: []string{
					"--version", "$LEFT",
				},
				RightArgs: []string{
					"--version", "$RIGHT",
				},
			},
			"helm-values": &Config{
				Diff: "diff -u --color",
				Startup: []string{
					`helm repo update`,
				},
				Preprocess: []string{`yq -P 'sort_keys(..)'`},
				CommonArgs: []string{
					"helm", "show", "values", "$CHART",
				},
				LeftArgs: []string{
					"--version", "$LEFT",
				},
				RightArgs: []string{
					"--version", "$RIGHT",
				},
			},
			"dyff": &Config{
				Diff: "dyff between --omit-header --set-exit-code",
			},
		},
	}
	applyDefaultValuesToConfigSet(cs)
	return cs
}
