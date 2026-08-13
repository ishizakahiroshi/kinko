package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPassword = "correct-horse-battery-staple"

// 記事で挙げた故障モードのうち「暗号化はできたが復号できない」を潰すテスト。
// 初回の使用で発覚する類だが、発覚したときには中身が入っているので、
// 作った直後に機械で確かめておく。
func Test保管庫は書いた内容をそのまま読み戻せる(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")

	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	want := map[string]string{
		"app/db_password": "p@ss word with spaces",
		"app/token":       "0123456789abcdef",
		// 日本語・改行・記号が壊れないこと。JSON と age を通すので、
		// バイト列がそのまま戻る必要がある。
		"note/japanese":  "日本語のメモ\n2行目\tタブ",
		"note/symbols":   `{"quoted": "value"} \ ' " $ ${kinko:x}`,
		"note/empty":     "",
		"very/long/name": strings.Repeat("a", 10000),
	}
	for k, val := range want {
		if err := v.Set(k, val); err != nil {
			t.Fatalf("Set(%s): %v", k, err)
		}
	}
	if err := v.Save(testPassword); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reopened, err := Open(path, testPassword)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if reopened.Count() != len(want) {
		t.Errorf("件数 = %d, want %d", reopened.Count(), len(want))
	}
	for k, expected := range want {
		got, err := reopened.Get(k)
		if err != nil {
			t.Errorf("Get(%s): %v", k, err)
			continue
		}
		if got != expected {
			t.Errorf("Get(%s) の値が変わっている（長さ %d → %d）", k, len(expected), len(got))
		}
	}
}

func Test違うパスワードでは開けない(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")

	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := v.Set("a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(testPassword); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(path, "wrong-password-entirely"); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("err = %v, want ErrWrongPassword", err)
	}
}

func Test保管庫のファイルに平文が残らない(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")

	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	// 値だけでなく名前も暗号化される必要がある。名前だけでも
	// 「どのサービスの秘密を持っているか」が分かってしまう。
	if err := v.Set("service-name-should-be-hidden", "secret-value-should-be-hidden"); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(testPassword); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"secret-value-should-be-hidden", "service-name-should-be-hidden"} {
		if strings.Contains(string(raw), needle) {
			t.Errorf("保管庫のファイルに %q がそのまま含まれている", needle)
		}
	}
	if !strings.HasPrefix(string(raw), "age-encryption.org/") {
		t.Error("age の形式になっていない")
	}
}

func Test既存の保管庫を上書きしない(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")

	if _, err := Create(path, testPassword); err != nil {
		t.Fatal(err)
	}
	// 2 回目の Create が通ると、中身の入った保管庫が一度の操作で消える。
	if _, err := Create(path, testPassword); err == nil {
		t.Error("既存の保管庫を上書きできてしまった")
	}
}

func Test無い名前はErrNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")

	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if err := v.Delete("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete の err = %v, want ErrNotFound", err)
	}
}

func Test一覧は名前だけを返す(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")

	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"z/last", "a/first", "m/middle"} {
		if err := v.Set(k, "value-"+k); err != nil {
			t.Fatal(err)
		}
	}

	names := v.Names()
	// 並びが安定していないと、一覧の差分を取るたびに順番が変わる。
	want := []string{"a/first", "m/middle", "z/last"}
	if len(names) != len(want) {
		t.Fatalf("件数 = %d, want %d", len(names), len(want))
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("names[%d] = %s, want %s", i, names[i], want[i])
		}
	}
	// 値が混ざっていないこと。
	for _, n := range names {
		if strings.HasPrefix(n, "value-") {
			t.Errorf("一覧に値が混ざっている: %s", n)
		}
	}
}

func Test保存に失敗しても既存の保管庫を壊さない(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")

	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Set("keep", "original-value"); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(testPassword); err != nil {
		t.Fatal(err)
	}

	// 空のパスワードで保存を試みて失敗させる。
	if err := v.Save(""); err == nil {
		t.Fatal("空のパスワードで保存できてしまった")
	}

	// 失敗しても、前の内容がそのまま読めること。
	reopened, err := Open(path, testPassword)
	if err != nil {
		t.Fatalf("失敗後に開けない: %v", err)
	}
	got, err := reopened.Get("keep")
	if err != nil || got != "original-value" {
		t.Errorf("失敗後の値 = %q（err=%v）, want original-value", got, err)
	}
}
