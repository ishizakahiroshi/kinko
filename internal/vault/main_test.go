package vault

import (
	"os"
	"testing"
)

// testGenerationsMarker tells re-executed helper subprocesses that the parent
// test process already chose a private generations directory.
const testGenerationsMarker = "KINKO_TEST_GENERATIONS_FROM_PARENT"

// TestMain keeps every Save in this package away from the real
// <UserConfigDir>/kinko-generations by pointing generations at a temp dir.
func TestMain(m *testing.M) {
	// Cheaper scrypt for this package's tests only (production keeps age's default).
	testScryptWorkFactor = 10
	os.Exit(runWithPrivateGenerations(m))
}

func runWithPrivateGenerations(m *testing.M) int {
	if os.Getenv(testGenerationsMarker) == "1" && os.Getenv(EnvGenerationsDir) != "" {
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "kinko-generations-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	os.Setenv(EnvGenerationsDir, dir)
	os.Setenv(testGenerationsMarker, "1")
	return m.Run()
}
