package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

const practicalPassword = "synthetic-master-password"
const practicalValue = "synthetic-private-value"

func TestInvalidArgumentsDoNotConsumeInputOrCreateVault(t *testing.T) {
	tests := []struct {
		name string
		call func() error
	}{
		{"init extra", func() error { return Init([]string{"extra"}) }},
		{"add empty", func() error { return AddWithOptions([]string{"  "}, CommandOptions{}) }},
		{"add option", func() error { return AddWithOptions([]string{"--invalid"}, CommandOptions{}) }},
		{"get empty", func() error { return GetWithOptions([]string{" "}, CommandOptions{}) }},
		{"get extra", func() error { return GetWithOptions([]string{"name", "extra"}, CommandOptions{}) }},
		{"get option", func() error { return GetWithOptions([]string{"--invalid"}, CommandOptions{}) }},
		{"list extra", func() error { return ListWithOptions([]string{"one", "two"}, CommandOptions{}) }},
		{"list option", func() error { return ListWithOptions([]string{"--invalid"}, CommandOptions{}) }},
		{"rm empty", func() error { return RemoveWithOptions([]string{" "}, CommandOptions{}) }},
		{"rm extra", func() error { return RemoveWithOptions([]string{"name", "extra"}, CommandOptions{}) }},
		{"export empty", func() error { return ExportWithOptions([]string{" "}, CommandOptions{}) }},
		{"export extra", func() error { return ExportWithOptions([]string{"one", "two"}, CommandOptions{}) }},
		{"run option", func() error { return RunWithOptions([]string{"--env-file", "example.env", "--invalid"}, CommandOptions{}) }},
		{"run duplicate", func() error { return RunWithOptions([]string{"--env-file=a", "--env-file=b", "--", "example"}, CommandOptions{}) }},
		{"run missing command", func() error { return RunWithOptions([]string{"--env-file=a", "--"}, CommandOptions{}) }},
		{"unlock option", func() error { return UnlockWithOptions([]string{"setup", "--invalid"}, CommandOptions{}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.age")
			t.Setenv(EnvVaultPath, path)
			t.Setenv(EnvPassword, "")
			withPipedStdin(t, "input-must-stay-unread")
			if err := test.call(); err == nil {
				t.Fatal("invalid arguments accepted")
			}
			remaining, err := readLine(os.Stdin)
			if err != nil || remaining != "input-must-stay-unread" {
				t.Fatal("invalid invocation consumed stdin")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid invocation created a vault")
			}
		})
	}
}

func TestLiteralHyphenNameAndAddVariadicCompatibility(t *testing.T) {
	args, err := positionalArgs([]string{"--", "-example", "one", "--two"})
	if err != nil || len(args) != 3 || args[0] != "-example" || args[2] != "--two" {
		t.Fatal("explicit positional arguments changed")
	}
}

type failingSecretWriter struct {
	short bool
}

func (w failingSecretWriter) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, errors.New(string(p))
}

func TestGetWriterFailuresAreErrorsWithoutSecretDiagnostics(t *testing.T) {
	for _, short := range []bool{false, true} {
		err := writeSecret(failingSecretWriter{short: short}, practicalValue)
		if err == nil || strings.Contains(err.Error(), practicalValue) {
			t.Fatal("writer failure succeeded or leaked its input")
		}
	}
}

func TestDailyWorkflowInSubprocesses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	runCLI(t, path, practicalPassword+"\n"+practicalPassword+"\n", nil, 0, "init")
	out, diagnostic := runCLI(t, path, practicalValue+"\n"+practicalPassword+"\n", nil, 0, "add", "example-service/token")
	if out != "" || strings.Contains(diagnostic, practicalValue) {
		t.Fatal("add exposed a value")
	}
	out, diagnostic = runCLI(t, path, practicalPassword+"\n", nil, 0, "get", "example-service/token")
	if out != practicalValue || strings.Contains(diagnostic, practicalValue) {
		t.Fatal("get violated exact no-newline stdout contract")
	}
	out, diagnostic = runCLI(t, path, practicalPassword+"\n", nil, 0, "list")
	if out != "example-service/token\n" || strings.Contains(out+diagnostic, practicalValue) {
		t.Fatal("list exposed a value or omitted a name")
	}

	template := filepath.Join(dir, "example.env")
	if err := os.WriteFile(template, []byte("SYNTHETIC_TOKEN=${kinko:example-service/token}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := []string{"run", "--env-file", template, "--", executable, "-test.run=^TestPracticalCLIHelper$", "--", "child"}
	out, diagnostic = runCLI(t, path, practicalPassword+"\nchild-input", nil, 23, child...)
	if out != "child-stdout" || diagnostic != "child-stderr" {
		t.Fatal("run did not preserve child stdin/stdout/stderr/exit status")
	}
	// Mixed-case password variables must all be removed on every platform.
	passwordEnv := []string{EnvPassword + "=" + practicalPassword, "kinko_password=" + practicalPassword, "KiNkO_PaSsWoRd=" + practicalPassword}
	out, diagnostic = runCLI(t, path, "child-input", passwordEnv, 23, child...)
	if out != "child-stdout" || diagnostic != "child-stderr" {
		t.Fatal("run leaked a password or changed the child's streams")
	}
	missing := filepath.Join(dir, "missing.env")
	if err := os.WriteFile(missing, []byte("SYNTHETIC_TOKEN=${kinko:example-service/missing}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "child-started")
	runCLI(t, path, practicalPassword+"\n", nil, 1, "run", "--env-file", missing, "--", executable, "-test.run=^TestPracticalCLIHelper$", "--", "marker", marker)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing reference launched a child")
	}

	dest := filepath.Join(dir, "backup.age")
	runCLI(t, path, practicalPassword+"\n\n", nil, 0, "export", dest)
	source, err := vault.Open(path, practicalPassword)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := vault.Open(dest, practicalPassword)
	if err != nil || source.VaultID() != backup.VaultID() {
		t.Fatal("export did not preserve logical vault ID")
	}
	out, _ = runCLI(t, dest, practicalPassword+"\n", nil, 0, "get", "example-service/token")
	if out != practicalValue {
		t.Fatal("reopened backup changed the value")
	}
	before, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	runCLI(t, path, "", nil, 1, "export", dest)
	after, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refused export modified destination")
	}
	runCLI(t, path, practicalPassword+"\n", nil, 0, "rm", "example-service/token")
	out, _ = runCLI(t, path, practicalPassword+"\n", nil, 0, "list")
	if out != "" {
		t.Fatal("removed name remained in list")
	}
}

func TestGetClosedStdoutFailsInSubprocess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	v, err := vault.Create(path, practicalPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Set("example-service/token", practicalValue); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(practicalPassword); err != nil {
		t.Fatal(err)
	}
	out, diagnostic := runCLI(t, path, practicalPassword+"\n", nil, 1, "get-closed", "example-service/token")
	if out != "" || strings.Contains(diagnostic, practicalValue) {
		t.Fatal("failed get leaked a value")
	}
}

func runCLI(t *testing.T, path, input string, extraEnv []string, wantExit int, args ...string) (string, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, append([]string{"-test.run=^TestPracticalCLIHelper$", "--"}, args...)...)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, EnvPassword) && !strings.EqualFold(name, EnvVaultPath) {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, EnvVaultPath+"="+path)
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Stdin = strings.NewReader(input)
	var out, diagnostic bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	err = cmd.Run()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != wantExit {
		t.Fatalf("CLI %s exit mismatch (want %d): %v", args[0], wantExit, err)
	}
	return out.String(), diagnostic.String()
}

// This subprocess exercises the production command functions without building
// a distributable binary or invoking real OS credentials. main's child-exit
// mapping is mirrored here; native user-presence remains a separate gate.
func TestPracticalCLIHelper(t *testing.T) {
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		return
	}
	options := CommandOptions{MasterPasswordOnly: true}
	var err error
	switch args[0] {
	case "init":
		err = Init(args[1:])
	case "add":
		err = AddWithOptions(args[1:], options)
	case "get", "get-closed":
		if args[0] == "get-closed" {
			_ = os.Stdout.Close()
		}
		err = GetWithOptions(args[1:], options)
	case "list":
		err = ListWithOptions(args[1:], options)
	case "rm":
		err = RemoveWithOptions(args[1:], options)
	case "run":
		err = RunWithOptions(args[1:], options)
	case "export":
		err = ExportWithOptions(args[1:], options)
	case "marker":
		err = os.WriteFile(args[1], nil, 0o600)
	case "child":
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if strings.EqualFold(name, EnvPassword) {
				os.Exit(70)
			}
		}
		input, readErr := io.ReadAll(os.Stdin)
		if readErr != nil || string(input) != "child-input" || os.Getenv("SYNTHETIC_TOKEN") != practicalValue {
			os.Exit(71)
		}
		fmt.Fprint(os.Stdout, "child-stdout")
		fmt.Fprint(os.Stderr, "child-stderr")
		os.Exit(23)
	default:
		err = errors.New("unknown synthetic helper command")
	}
	if err != nil {
		var childErr *ChildExitError
		if errors.As(err, &childErr) {
			os.Exit(childErr.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
