package cli

import (
	"os"
	"testing"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

const testGenerationsMarker = "KINKO_TEST_GENERATIONS_FROM_PARENT"

// TestMain keeps every vault write in this package (including the re-executed
// CLI helper subprocesses) away from the real <UserConfigDir>/kinko-generations.
func TestMain(m *testing.M) {
	os.Exit(runWithPrivateGenerations(m))
}

func runWithPrivateGenerations(m *testing.M) int {
	if os.Getenv(testGenerationsMarker) == "1" && os.Getenv(vault.EnvGenerationsDir) != "" {
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "kinko-generations-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	os.Setenv(vault.EnvGenerationsDir, dir)
	os.Setenv(testGenerationsMarker, "1")
	return m.Run()
}
