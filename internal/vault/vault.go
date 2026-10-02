// Package vault は秘密の保管庫を読み書きする。
//
// # 暗号は自分で書かない
//
// 暗号処理は filippo.io/age に丸投げする。このパッケージが持つのは
// 「JSON をどう詰めるか」「ファイルをどう安全に置き換えるか」だけで、
// 鍵導出も暗号方式も age の判断に従う。
//
// 暗号を自作すると、間違いが「動くけれど守れていない」という形で出る。
// テストは通り、日常の操作も成功し、破られたときにだけ分かる。
// そこは既に検証された実装へ預ける。
//
// # ファイルの形
//
// 平文の中身は次の JSON で、それを age で丸ごと暗号化したものが vault ファイルになる。
//
//	{
//	  "version": 1,
//	  "secrets": { "example-service/admin": "…", "package-registry/token": "…" }
//	}
//
// キー名も暗号化される。SOPS のような部分暗号化（キーは平文・値だけ暗号）に
// しないのは、こちらは git へコミットしないためである。差分を読む必要がないなら、
// キー名まで隠すほうが漏れる情報が少ない。
package vault

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
)

// FormatVersion は vault ファイルの形式。
//
// 形を変えるときはここを上げ、読み込み側で古い形も読めるようにする。
// 「読めなくなった」は秘密の全損に直結するので、後方互換は必ず残す。
const FormatVersion = 1

// ErrNotFound は指定した名前の秘密が無いことを表す。
var ErrNotFound = errors.New("kinko: 指定された名前の秘密がありません")

// ErrWrongPassword はパスワードが違う（または壊れている）ことを表す。
var ErrWrongPassword = errors.New("kinko: パスワードが違うか、保管庫が壊れています")

// ErrConflict は読み込み後に保管庫が別の操作で更新されたことを表す。
var ErrConflict = errors.New("kinko: 保管庫が別の操作で更新されました。もう一度実行してください")

// lockTimeout は書き込み用ロックを待つ上限。
const lockTimeout = 5 * time.Second

type document struct {
	Version int               `json:"version"`
	Secrets map[string]string `json:"secrets"`
}

// Vault は開いている保管庫。
type Vault struct {
	path string
	doc  document
	// rev は読み込んだ時点のファイル内容の識別子。ファイルが無いときは空。
	// 保存時に現在の内容と比べ、他の操作の変更を黙って上書きしない。
	rev string
}

// revisionOf は保管庫ファイルの現在の内容の識別子を返す。ファイルが無ければ空。
func revisionOf(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return sumOf(raw), nil
}

func sumOf(raw []byte) string {
	h := sha256.Sum256(raw)
	return fmt.Sprintf("%x", h)
}

// acquireLock は保存の間だけ保管庫への書き込みを排他する。
func acquireLock(path string) (release func(), err error) {
	lockPath := path + ".lock"
	deadline := time.Now().Add(lockTimeout)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("kinko: ロックを取得できません: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("kinko: 別の操作が保管庫を使用中です（残っている場合は %s を削除してください）", lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Create は新しい保管庫を作る。
//
// 既にファイルがある場合は失敗する。上書きすると、既存の秘密が
// 一度の操作ミスで全部消えるため。
func Create(path, password string) (*Vault, error) {
	if password == "" {
		return nil, errors.New("kinko: パスワードが空です")
	}
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("kinko: 保管庫が既にあります: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("kinko: 保管庫を確認できません: %w", err)
	}

	v := &Vault{
		path: path,
		doc:  document{Version: FormatVersion, Secrets: map[string]string{}},
	}
	if err := v.Save(password); err != nil {
		return nil, err
	}
	return v, nil
}

// Open は既存の保管庫を開く。
func Open(path, password string) (*Vault, error) {
	if password == "" {
		return nil, errors.New("kinko: パスワードが空です")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("kinko: 保管庫がありません（kinko init で作成してください）: %s", path)
		}
		return nil, fmt.Errorf("kinko: 保管庫を読めません: %w", err)
	}

	identity, err := age.NewScryptIdentity(password)
	if err != nil {
		return nil, fmt.Errorf("kinko: 鍵を組み立てられません: %w", err)
	}

	rev := sumOf(raw)

	plain, err := age.Decrypt(bytes.NewReader(raw), identity)
	if err != nil {
		// age は「パスワード違い」と「ファイル破損」を区別しない。
		// 利用者にとって最初に確かめるのはパスワードなので、そちらを前に出す。
		return nil, ErrWrongPassword
	}

	var doc document
	if err := json.NewDecoder(plain).Decode(&doc); err != nil {
		return nil, fmt.Errorf("kinko: 保管庫の中身を解釈できません: %w", err)
	}
	if doc.Version > FormatVersion {
		return nil, fmt.Errorf(
			"kinko: この保管庫は新しい形式です（形式 %d・このコマンドは %d まで）。kinko を更新してください",
			doc.Version, FormatVersion)
	}
	if doc.Secrets == nil {
		doc.Secrets = map[string]string{}
	}

	return &Vault{path: path, doc: doc, rev: rev}, nil
}

// Get は秘密を1件取り出す。
func (v *Vault) Get(name string) (string, error) {
	value, ok := v.doc.Secrets[name]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return value, nil
}

// Set は秘密を追加・更新する。
func (v *Vault) Set(name, value string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("kinko: 名前が空です")
	}
	v.doc.Secrets[name] = value
	return nil
}

// Delete は秘密を消す。
func (v *Vault) Delete(name string) error {
	if _, ok := v.doc.Secrets[name]; !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	delete(v.doc.Secrets, name)
	return nil
}

// Names は保管している名前の一覧を返す（値は返さない）。
//
// 一覧に値を混ぜないのは、`kinko list` が「何が入っているか確かめる」ための
// コマンドであり、値を見る用途は `kinko get` に分けているため。
// 一覧のたびに全部の値が画面へ出ると、その画面が新しい漏洩経路になる。
func (v *Vault) Names() []string {
	names := make([]string, 0, len(v.doc.Secrets))
	for name := range v.doc.Secrets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Count は保管している件数を返す。
func (v *Vault) Count() int { return len(v.doc.Secrets) }

// Path は保管庫の場所を返す。
func (v *Vault) Path() string { return v.path }

// Save は保管庫を書き出す。
//
// # 途中で失敗しても壊さない
//
// 同じファイルへ直接書くと、書き込みの途中で電源が落ちたときに
// 「半分だけ新しい」ファイルが残る。age の暗号ファイルは途中で切れると
// 全部読めなくなるので、それは秘密の全損になる。
// 一時ファイルへ書き切ってから置き換える。
func (v *Vault) Save(password string) error {
	if password == "" {
		return errors.New("kinko: パスワードが空です")
	}

	dir := filepath.Dir(v.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("kinko: 保管庫の置き場を作れません: %w", err)
	}

	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return fmt.Errorf("kinko: 鍵を組み立てられません: %w", err)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return fmt.Errorf("kinko: 暗号化を開始できません: %w", err)
	}
	v.doc.Version = FormatVersion
	if err := json.NewEncoder(w).Encode(v.doc); err != nil {
		return fmt.Errorf("kinko: 中身を書き出せません: %w", err)
	}
	// Close で暗号の終端が書かれる。忘れると復号できないファイルができる。
	if err := w.Close(); err != nil {
		return fmt.Errorf("kinko: 暗号化を完了できません: %w", err)
	}

	encrypted := buf.Bytes()

	// 読み込みから保存までの間に他の操作が更新していないことを、ロックの中で確かめる。
	release, err := acquireLock(v.path)
	if err != nil {
		return err
	}
	defer release()
	current, err := revisionOf(v.path)
	if err != nil {
		return fmt.Errorf("kinko: 保管庫を確認できません: %w", err)
	}
	if current != v.rev {
		return ErrConflict
	}

	tmp, err := os.CreateTemp(dir, ".kinko-*.tmp")
	if err != nil {
		return fmt.Errorf("kinko: 一時ファイルを作れません: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// 置き換えが成功していれば、この Remove は対象が無いので何もしない。
		_ = os.Remove(tmpName)
	}()

	// 本人以外に読ませない。Windows では ACL が優先されるが、
	// Linux / macOS では効く。
	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, os.ErrInvalid) {
		_ = tmp.Close()
		return fmt.Errorf("kinko: 一時ファイルの権限を設定できません: %w", err)
	}
	if _, err := io.Copy(tmp, bytes.NewReader(encrypted)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("kinko: 一時ファイルへ書けません: %w", err)
	}
	// ディスクへ落としてから置き換える。落とす前に rename すると、
	// 電源断で「名前はあるが中身が空」になり得る。
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("kinko: 一時ファイルを保存できません: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("kinko: 一時ファイルを閉じられません: %w", err)
	}

	if err := os.Rename(tmpName, v.path); err != nil {
		return fmt.Errorf("kinko: 保管庫を置き換えられません: %w", err)
	}
	v.rev = sumOf(encrypted)
	return nil
}
