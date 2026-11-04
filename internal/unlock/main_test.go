package unlock

import (
	"os"
	"testing"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

// TestMain keeps every vault write in this package away from the real
// <UserConfigDir>/kinko-generations by pointing generations at a temp dir.
func TestMain(m *testing.M) {
	os.Exit(runWithPrivateGenerations(m))
}

func runWithPrivateGenerations(m *testing.M) int {
	dir, err := os.MkdirTemp("", "kinko-generations-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	os.Setenv(vault.EnvGenerationsDir, dir)
	return m.Run()
}
