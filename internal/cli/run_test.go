package cli

import (
	"strings"
	"testing"
)

func TestChildEnvironmentDoesNotInheritVaultPassword(t *testing.T) {
	parent := []string{
		"PATH=C:\\tools",
		EnvPassword + "=parent-master-value",
		"kinko_password=case-variant-value",
		"APP_MODE=test",
		"=C:=C:\\work",
	}
	expanded := []string{"APP_TOKEN=individual-value"}

	got := childEnvironment(parent, expanded)

	for _, entry := range got {
		name, _, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(name, EnvPassword) {
			t.Fatalf("子環境に %s が残っている: %q", EnvPassword, entry)
		}
	}

	for _, want := range []string{
		"PATH=C:\\tools",
		"APP_MODE=test",
		"=C:=C:\\work",
		"APP_TOKEN=individual-value",
	} {
		if !containsString(got, want) {
			t.Errorf("子環境に %q が無い: %#v", want, got)
		}
	}
}

func TestExpandRejectsReservedPasswordVariable(t *testing.T) {
	for _, name := range []string{EnvPassword, "kinko_password"} {
		t.Run(name, func(t *testing.T) {
			_, err := expand(name+"=value\n", nil)
			if err == nil {
				t.Fatalf("予約変数 %s を設定できてしまった", name)
			}
			if !strings.Contains(err.Error(), EnvPassword) {
				t.Errorf("エラーに予約変数名が無い: %v", err)
			}
		})
	}
}

func TestExpandRejectsEmptyEnvironmentName(t *testing.T) {
	_, err := expand(" =value\n", nil)
	if err == nil {
		t.Fatal("空の環境変数名を受理してしまった")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
