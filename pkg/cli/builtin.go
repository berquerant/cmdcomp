package cli

import "github.com/berquerant/cmdcomp/pkg/config"

func builtinConfigSet() *ConfigSet {
	cs := &ConfigSet{
		Presets: map[string]*Config{
			"json": &Config{
				Config: config.Config{
					Preprocess: []string{`jq -S .`},
					Diff:       "diff -u --color",
				},
			},
			"yml": &Config{
				Config: config.Config{
					Preprocess: []string{`yq -P 'sort_keys(..)'`},
					Diff:       "diff -u --color",
				},
			},
			"k8s": &Config{
				Config: config.Config{
					Diff: "objdiff -cv",
				},
			},
			"k8s-clean": &Config{
				Config: config.Config{
					Preprocess: []string{
						`yq 'del(.metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp, .metadata.generation, .metadata.managedFields, .status)'`,
					},
					Diff: "objdiff -cv",
				},
			},
			"helm": &Config{
				Config: config.Config{
					Diff: "objdiff -cv",
					Startup: []string{
						`helm repo update`,
					},
				},
			},
			"helm-chart": &Config{
				Config: config.Config{
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
			},
			"helm-values": &Config{
				Config: config.Config{
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
			},
			"dyff": &Config{
				Config: config.Config{
					Diff: "dyff between --omit-header --set-exit-code",
				},
			},
		},
	}
	applyDefaultValuesToConfigSet(cs)
	return cs
}
