package cli_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/berquerant/cmdcomp/pkg/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

func TestUsageBuilder_Build(t *testing.T) {
	got := cli.UsageBuilder{}.Build()
	goldenPath := filepath.Join("testdata", "usage.golden")

	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(got), 0644))
		return
	}

	golden, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	assert.Equal(t, string(golden), got)
}
