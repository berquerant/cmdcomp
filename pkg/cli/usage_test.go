package cli_test

import (
	"os"
	"testing"

	"github.com/berquerant/cmdcomp/pkg/cli"
	"github.com/stretchr/testify/assert"
)

func TestUsageBuilder_Build(t *testing.T) {
	got := cli.UsageBuilder{}.Build()
	golden, err := os.ReadFile("testdata/usage.golden")
	if os.IsNotExist(err) {
		// ゴールデンファイルが存在しない場合は作成
		err := os.MkdirAll("testdata", 0755)
		assert.NoError(t, err)
		err = os.WriteFile("testdata/usage.golden", []byte(got), 0644)
		assert.NoError(t, err)
		return
	}
	assert.NoError(t, err)
	assert.Equal(t, string(golden), got)
}
