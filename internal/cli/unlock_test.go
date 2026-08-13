package cli

import (
	"strings"
	"testing"

	unlockservice "github.com/ishizakahiroshi/kinko/internal/unlock"
)

func TestParseUnlockFlags(t *testing.T) {
	flags, err := parseUnlockFlags([]string{"--dry-run", "--yes", "--replace"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !flags.dryRun || !flags.yes || !flags.replace {
		t.Fatalf("flags = %#v", flags)
	}
	if _, err := parseUnlockFlags([]string{"--replace"}, false); err == nil {
		t.Fatal("disableでreplaceを受理しました")
	}
	if _, err := parseUnlockFlags([]string{"--unknown"}, true); err == nil {
		t.Fatal("unknown flagを受理しました")
	}
}

func TestUnlockFallbackMessageDoesNotIncludeProviderDetails(t *testing.T) {
	message := unlockFallbackMessage(unlockservice.ErrorUnavailable)
	if strings.Contains(message, "synthetic") || strings.Contains(message, "vault_id") {
		t.Fatalf("fallback message contains secret-like detail: %s", message)
	}
}
