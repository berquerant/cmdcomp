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
			"branch": &Config{
				Config: config.Config{
					Interceptor: []string{
						`git switch ${RIGHT}`,
					},
					Cleanup: []string{
						`git switch ${ORIG}`,
					},
					Diff: "diff -u --color",
				},
			},
		},
	}
	applyDefaultValuesToConfigSet(cs)
	return cs
}
