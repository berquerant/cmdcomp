package cli

import "github.com/berquerant/cmdcomp/pkg/config"

func builtinConfigSet() *ConfigSet {
	cs := &ConfigSet{
		Presets: map[string]*Preset{
			"json": &Preset{
				Config: config.Config{
					Preprocess: []string{`jq -S .`},
					Diff:       "diff -u --color",
				},
			},
			"yml": &Preset{
				Config: config.Config{
					Preprocess: []string{`yq -P 'sort_keys(..)'`},
					Diff:       "diff -u --color",
				},
			},
			"k8s": &Preset{
				Config: config.Config{
					Diff: "objdiff -cv",
				},
			},
			"k8s-clean": &Preset{
				Config: config.Config{
					Preprocess: []string{
						`yq 'del(.metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp, .metadata.generation, .metadata.managedFields, .status)'`,
					},
					Diff: "objdiff -cv",
				},
			},
			"helm": &Preset{
				Config: config.Config{
					Diff: "objdiff -cv",
					Startup: []string{
						`helm repo update`,
					},
				},
			},
			"helm-chart": &Preset{
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
			"helm-values": &Preset{
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
			"dyff": &Preset{
				Config: config.Config{
					Diff: "dyff between --omit-header --set-exit-code",
				},
			},
		},
	}
	applyDefaultValuesToConfigSet(cs)
	return cs
}
