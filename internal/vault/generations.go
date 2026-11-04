package vault

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

// EnvGenerationsDir overrides where local generations are kept. Tests and users
// who want generations on another local disk set it; the default is
// <UserConfigDir>/kinko-generations, outside the default vault directory so that
// deleting the vault directory does not also delete its generations.
const EnvGenerationsDir = "KINKO_GENERATIONS_DIR"

// generationMetaKey is the optional format 1 field that records lineage inside
// the encrypted document. Older kinko versions keep unknown fields unchanged.
const generationMetaKey = "generation"

const (
	generationExt  = ".age"
	committedExt   = ".ok"
	collectionSalt = "kinko/generations/v1\x00"
)

// Retention defaults (DESIGN.md chapter 10, provisional). Tests shrink them.
var (
	retainCount             = 32
	retainAge               = 30 * 24 * time.Hour
	generationCapacity      = int64(512 << 20)
	generationWarnRatio     = 0.8
	pinExt                  = ".pin"
	errNoGenerationCapacity = errors.New("kinko: 世代の保存領域が上限（512 MiB）に達しています。古い世代を整理できないため、変更を止めました（読み出しはできます）")
)

// ErrGenerationsFull means the generations could not be kept within the
// capacity limit without removing protected generations. The vault is unchanged.
var ErrGenerationsFull = errNoGenerationCapacity

// ErrGenerationConflict means a generation file with the same name already
// exists but holds different bytes. kinko never overwrites a generation.
var ErrGenerationConflict = errors.New("kinko: 同じ名前の世代が別の内容で既にあります。世代の保存場所を確認してください")

// GenerationMeta is the lineage stored inside each encrypted generation.
type GenerationMeta struct {
	ID      string `json:"id"`
	Parent  string `json:"parent"`
	SavedAt string `json:"saved_at"`
}

// GenerationsRoot returns the directory that holds all local generation collections.
func GenerationsRoot() (string, error) {
	if dir := os.Getenv(EnvGenerationsDir); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("kinko: 世代の置き場を解決できません: %w", err)
	}
	return filepath.Join(base, "kinko-generations"), nil
}

// collectionName is an opaque directory name derived from vault_id. The ID
// itself is not written outside the encrypted document.
func collectionName(vaultID string) string {
	sum := sha256.Sum256([]byte(collectionSalt + vaultID))
	return hex.EncodeToString(sum[:16])
}

type generationStore struct {
	dir string
}

func openGenerationStore(vaultID string) (*generationStore, error) {
	if !validVaultID(vaultID) {
		return nil, errors.New("kinko: 世代を保存する保管庫の識別子がありません")
	}
	root, err := GenerationsRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, collectionName(vaultID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("kinko: 世代の置き場を作れません: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return nil, errors.New("kinko: 世代の置き場がディレクトリではありません")
	}
	return &generationStore{dir: dir}, nil
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *generationStore) path(hash string) string {
	return filepath.Join(s.dir, hash+generationExt)
}

func (s *generationStore) markerPath(hash string) string {
	return filepath.Join(s.dir, hash+committedExt)
}

// put stores ciphertext as a new, never-overwritten generation and returns its
// hash. An existing generation with identical bytes is reused.
func (s *generationStore) put(ciphertext []byte) (string, error) {
	hash := hashHex(ciphertext)
	final := s.path(hash)
	if existing, err := os.ReadFile(final); err == nil {
		if hashHex(existing) != hash {
			return "", ErrGenerationConflict
		}
		return hash, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("kinko: 世代を確認できません: %w", err)
	}

	tmp, err := os.CreateTemp(s.dir, ".gen-*.tmp")
	if err != nil {
		return "", fmt.Errorf("kinko: 世代の一時ファイルを作れません: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, os.ErrInvalid) {
		_ = tmp.Close()
		return "", fmt.Errorf("kinko: 世代の権限を設定できません: %w", err)
	}
	if _, err := io.Copy(tmp, bytes.NewReader(ciphertext)); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("kinko: 世代を書けません: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("kinko: 世代を保存できません: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("kinko: 世代を閉じられません: %w", err)
	}
	if err := os.Link(tmpName, final); err != nil {
		if errors.Is(err, os.ErrExist) {
			existing, readErr := os.ReadFile(final)
			if readErr == nil && hashHex(existing) == hash {
				return hash, nil
			}
			return "", ErrGenerationConflict
		}
		return "", fmt.Errorf("kinko: 世代を公開できません: %w", err)
	}
	if err := syncDir(s.dir); err != nil {
		return "", err
	}
	readBack, err := os.ReadFile(final)
	if err != nil || hashHex(readBack) != hash {
		return "", errors.New("kinko: 保存した世代を読み戻して確認できません")
	}
	return hash, nil
}

// markCommitted records that the generation became (or was) the live vault.
func (s *generationStore) markCommitted(hash string) error {
	f, err := os.OpenFile(s.markerPath(hash), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return fmt.Errorf("kinko: 世代の確定を記録できません: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("kinko: 世代の確定を記録できません: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("kinko: 世代の確定を記録できません: %w", err)
	}
	return syncDir(s.dir)
}

type generationFile struct {
	hash      string
	size      int64
	modified  time.Time
	committed bool
	pinned    bool
}

func (s *generationStore) list() ([]generationFile, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("kinko: 世代を一覧できません: %w", err)
	}
	present := map[string]bool{}
	for _, e := range entries {
		present[e.Name()] = true
	}
	var out []generationFile
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != generationExt {
			continue
		}
		hash := name[:len(name)-len(generationExt)]
		if len(hash) != 64 {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("kinko: 世代を確認できません: %w", err)
		}
		out = append(out, generationFile{
			hash:      hash,
			size:      info.Size(),
			modified:  info.ModTime(),
			committed: present[hash+committedExt],
			pinned:    present[hash+pinExt],
		})
	}
	return out, nil
}

func (s *generationStore) usage() (int64, error) {
	files, err := s.list()
	if err != nil {
		return 0, err
	}
	var total int64
	for _, f := range files {
		total += f.size
	}
	return total, nil
}

// prune removes committed generations that no retention rule keeps. It never
// removes the live generation, uncommitted candidates, pinned generations, or
// generations for which keep (may be nil) returns true, such as ones not yet
// copied to every backup destination.
func (s *generationStore) prune(live string, now time.Time, keep func(string) bool) error {
	files, err := s.list()
	if err != nil {
		return err
	}
	var committed []generationFile
	for _, f := range files {
		if f.committed {
			committed = append(committed, f)
		}
	}
	sort.Slice(committed, func(i, j int) bool { return committed[i].modified.After(committed[j].modified) })
	var firstErr error
	for i, f := range committed {
		if i < retainCount || now.Sub(f.modified) <= retainAge || f.hash == live || f.pinned {
			continue
		}
		if keep != nil && keep(f.hash) {
			continue
		}
		if err := os.Remove(s.path(f.hash)); err != nil && !errors.Is(err, os.ErrNotExist) {
			if firstErr == nil {
				firstErr = fmt.Errorf("kinko: 古い世代を整理できません: %w", err)
			}
			continue
		}
		_ = os.Remove(s.markerPath(f.hash))
	}
	if err := syncDir(s.dir); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// syncDir makes a directory entry durable where the OS supports it. Windows
// cannot open a directory for FlushFileBuffers through os.File, so the step is
// skipped there; the documentation states this weaker guarantee.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("kinko: ディレクトリを同期できません: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("kinko: ディレクトリを同期できません: %w", err)
	}
	return nil
}

// stampGeneration writes fresh lineage into the document before encryption.
// parent is the ciphertext hash of the generation being replaced ("" for a new vault).
func (d *document) stampGeneration(parent string) error {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("kinko: 世代の識別子を作れません: %w", err)
	}
	meta := GenerationMeta{
		ID:      hex.EncodeToString(raw),
		Parent:  parent,
		SavedAt: time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if d.Extra == nil {
		d.Extra = map[string]json.RawMessage{}
	}
	d.Extra[generationMetaKey] = encoded
	return nil
}

// Generation returns the lineage recorded in this opened vault or generation.
// ok is false for documents saved before generations existed.
func (v *Vault) Generation() (GenerationMeta, bool) {
	raw, ok := v.doc.Extra[generationMetaKey]
	if !ok {
		return GenerationMeta{}, false
	}
	var meta GenerationMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return GenerationMeta{}, false
	}
	return meta, true
}

// SaveNotices returns short, value-free notes from the last Save (for example,
// that the generations area is nearly full). The CLI prints them to stderr.
func (v *Vault) SaveNotices() []string {
	return append([]string(nil), v.notices...)
}

// SnapshotHash returns the SHA-256 (hex) of the ciphertext this Vault was opened
// from or last saved as. Generation files are named by this hash.
func (v *Vault) SnapshotHash() string {
	return hex.EncodeToString(v.snapshot[:])
}

// GenerationsDir returns the collection directory for this vault, or "" when
// the vault has no ID yet.
func (v *Vault) GenerationsDir() (string, error) {
	if v.doc.VaultID == "" {
		return "", nil
	}
	root, err := GenerationsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, collectionName(v.doc.VaultID)), nil
}
