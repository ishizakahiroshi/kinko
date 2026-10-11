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
	"strings"
)

// ErrLocked is fail-closed: a live writer or a leftover lock needs attention.
var ErrLocked = errors.New("kinko: 保管庫は書込み中、または未解除の lock があります")

// ErrConflict means the snapshot changed since Open; reopen before retrying.
var ErrConflict = errors.New("kinko: 保管庫が変更されています。開き直して再実行してください")

// canonicalPath pins a stable absolute target, including existing symlinks and
// symlinked parent directories. Dangling symlinks are never creation targets.
func canonicalPath(path string, create bool) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("kinko: 保管庫のパスが空です")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("kinko: 保管庫のパスを解決できません: %w", err)
	}
	if create {
		if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
			return "", fmt.Errorf("kinko: 保管庫の置き場を作れません: %w", err)
		}
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			return "", errors.New("kinko: 保管庫は通常のファイルである必要があります")
		}
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("kinko: 保管庫のパスを解決できません: %w", err)
	}
	if _, err := os.Lstat(abs); err == nil || !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("kinko: 保管庫のリンクを解決できません")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", fmt.Errorf("kinko: 保管庫の親ディレクトリを解決できません: %w", err)
	}
	return filepath.Join(parent, filepath.Base(abs)), nil
}

// An exclusive directory is portable and requires no extra dependency. Never
// steal a lock by age/PID: an apparently stale owner might still be committing.
func lockWriter(path string) (func() error, error) {
	lockPath := path + ".lock"
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("kinko: 書込み lock を作れません: %w", err)
	}
	return func() error {
		if err := os.Remove(lockPath); err != nil {
			return fmt.Errorf("kinko: 書込み lock を解除できません: %w", err)
		}
		return nil
	}, nil
}

func (v *Vault) checkSnapshot() error {
	info, err := os.Lstat(v.path)
	if errors.Is(err, os.ErrNotExist) && !v.loaded {
		return nil
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrConflict
		}
		return fmt.Errorf("kinko: 保管庫を確認できません: %w", err)
	}
	if !v.loaded {
		return fmt.Errorf("kinko: 保管庫が既にあります: %w", ErrConflict)
	}
	if !info.Mode().IsRegular() {
		return ErrConflict
	}
	raw, err := os.ReadFile(v.path)
	if err != nil {
		return fmt.Errorf("kinko: 保管庫を再確認できません: %w", err)
	}
	if sha256.Sum256(raw) != v.snapshot {
		return ErrConflict
	}
	return nil
}

// Export publishes a complete encrypted copy without an intermediate empty
// vault, without overwriting a destination, and without unsafe rollback removal.
// The caller must persist an ID on legacy source vaults before calling Export.
func (v *Vault) Export(path, password string) error {
	if v.VaultID() == "" {
		return errors.New("kinko: 旧保管庫を保存してから書き出してください")
	}
	canonical, err := canonicalPath(path, true)
	if err != nil {
		return err
	}
	out := &Vault{path: canonical, doc: v.doc}
	return out.Save(password)
}

// Preserve unknown optional metadata when updating a format 1 document.
func (d *document) UnmarshalJSON(raw []byte) error {
	fields, err := decodeUniqueObject(raw)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(fields["version"], &d.Version); err != nil {
		return errors.New("invalid vault version")
	}
	secretFields, err := decodeUniqueObject(fields["secrets"])
	if err != nil {
		return errors.New("invalid vault secrets")
	}
	d.Secrets = make(map[string]string, len(secretFields))
	for name, rawValue := range secretFields {
		valueJSON := bytes.TrimSpace(rawValue)
		// encoding/json accepts null for a Go string as its zero value. Check
		// the JSON type explicitly so malformed values never become empty secrets.
		if len(valueJSON) == 0 || valueJSON[0] != '"' {
			return errors.New("invalid vault secret value")
		}
		var value string
		if err := json.Unmarshal(valueJSON, &value); err != nil {
			return errors.New("invalid vault secret value")
		}
		d.Secrets[name] = value
	}
	if id, ok := fields["vault_id"]; ok {
		if string(id) == "null" || json.Unmarshal(id, &d.VaultID) != nil {
			return errors.New("invalid vault ID")
		}
	}
	delete(fields, "version")
	delete(fields, "secrets")
	delete(fields, "vault_id")
	d.Extra = fields
	return nil
}

func (d document) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(d.Extra)+3)
	for key, value := range d.Extra {
		fields[key] = value
	}
	version, err := json.Marshal(d.Version)
	if err != nil {
		return nil, err
	}
	secrets, err := json.Marshal(d.Secrets)
	if err != nil {
		return nil, err
	}
	fields["version"] = version
	fields["secrets"] = secrets
	if d.VaultID != "" {
		id, err := json.Marshal(d.VaultID)
		if err != nil {
			return nil, err
		}
		fields["vault_id"] = id
	}
	return json.Marshal(fields)
}

func decodeUniqueObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("invalid vault document")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, errors.New("invalid vault document")
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("invalid vault field")
		}
		if _, exists := fields[key]; exists {
			return nil, errors.New("duplicate vault field")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, errors.New("invalid vault field")
		}
		fields[key] = value
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, errors.New("invalid vault document")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("invalid vault document")
	}
	return fields, nil
}
