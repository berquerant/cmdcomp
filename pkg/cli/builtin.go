package cli

func builtinConfigSet() *ConfigSet {
	cs := &ConfigSet{
		Presets: map[string]*Config{
			"json": &Config{
				Preprocess: []string{`jq -S .`},
				Diff:       "diff -u --color",
			},
			"yaml": &Config{
				Preprocess: []string{`yq -P 'sort_keys(..)'`},
				Diff:       "diff -u --color",
			},
			"objdiff": &Config{
				Diff: "objdiff -cv",
			},
			"dyff": &Config{
				Diff: "dyff between --omit-header --set-exit-code",
			},
			"u": &Config{
				Diff: "diff -u",
			},
			"uc": &Config{
				Diff: "diff -u --color",
			},
		},
	}
	applyDefaultValuesToConfigSet(cs)
	return cs
}
