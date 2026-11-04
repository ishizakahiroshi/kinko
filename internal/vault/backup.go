package vault

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Backup destinations are kept inside the encrypted document, so no plaintext
// configuration file is added and a write can push while the vault is open.
const destinationsKey = "backup_destinations"

const (
	destinationSubdir = "kinko-backup"
	markerFileName    = ".kinko-destination"
	verifiedFileName  = ".kinko-verified"
)

var (
	// ErrDestinationOffline means the destination root is not reachable now
	// (for example, the USB drive is not connected). Nothing was copied.
	ErrDestinationOffline = errors.New("kinko: 保存先が見つかりません（接続されていない可能性があります）")
	// ErrDestinationMismatch means the destination is reachable but is not the
	// one that was registered (marker or disk identity differs). Nothing was copied.
	ErrDestinationMismatch = errors.New("kinko: 保存先が登録したものと違います（目印またはディスクの識別が一致しません）")
)

// Destination is one user-chosen place outside this PC that receives
// append-only copies of committed generations.
type Destination struct {
	Path     string `json:"path"`
	Marker   string `json:"marker"`
	Volume   string `json:"volume,omitempty"`
	SameDisk bool   `json:"same_disk,omitempty"`
	AddedAt  string `json:"added_at"`
}

// DestinationState is the live state of one destination.
type DestinationState struct {
	Destination
	Reachable      bool
	Err            error
	Pending        int
	VerifiedAt     string
	VerifiedPrefix string
}

// PushResult reports one destination after a push attempt.
type PushResult struct {
	Path    string
	Copied  int
	Pending int
	Err     error
}

// BackupDestinations returns the registered destinations.
func (v *Vault) BackupDestinations() []Destination {
	raw, ok := v.doc.Extra[destinationsKey]
	if !ok {
		return nil
	}
	var out []Destination
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func (v *Vault) setDestinations(list []Destination) error {
	if v.doc.Extra == nil {
		v.doc.Extra = map[string]json.RawMessage{}
	}
	if len(list) == 0 {
		delete(v.doc.Extra, destinationsKey)
		return nil
	}
	encoded, err := json.Marshal(list)
	if err != nil {
		return err
	}
	v.doc.Extra[destinationsKey] = encoded
	return nil
}

func cleanDestinationPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("kinko: 保存先の場所が空です")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("kinko: 保存先の場所を解決できません: %w", err)
	}
	return filepath.Clean(abs), nil
}

func samePath(a, b string) bool {
	if pathsCaseInsensitive {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (v *Vault) destinationCollection(d Destination) string {
	return filepath.Join(d.Path, destinationSubdir, collectionName(v.doc.VaultID))
}

// AddDestination registers a destination in memory. The caller must Save to
// keep it. The root must already exist: kinko never creates a missing drive's
// mount point, so an unplugged drive cannot silently become a local folder.
func (v *Vault) AddDestination(path string) (Destination, error) {
	if _, err := v.EnsureID(); err != nil {
		return Destination{}, err
	}
	root, err := cleanDestinationPath(path)
	if err != nil {
		return Destination{}, err
	}
	for _, d := range v.BackupDestinations() {
		if samePath(d.Path, root) {
			return Destination{}, errors.New("kinko: この保存先は既に登録されています")
		}
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return Destination{}, ErrDestinationOffline
	}
	d := Destination{Path: root, AddedAt: time.Now().UTC().Format(time.RFC3339)}
	col := v.destinationCollection(d)
	if err := os.MkdirAll(col, 0o700); err != nil {
		return Destination{}, fmt.Errorf("kinko: 保存先にフォルダを作れません: %w", err)
	}
	marker, err := readMarker(col)
	if errors.Is(err, os.ErrNotExist) {
		marker, err = writeMarker(col)
	}
	if err != nil {
		return Destination{}, err
	}
	d.Marker = marker
	d.Volume = volumeID(root)
	if d.Volume != "" {
		genRoot, _ := GenerationsRoot()
		for _, local := range []string{filepath.Dir(v.path), genRoot} {
			if local != "" && volumeID(local) == d.Volume {
				d.SameDisk = true
			}
		}
	}
	list := append(v.BackupDestinations(), d)
	if err := v.setDestinations(list); err != nil {
		return Destination{}, err
	}
	return d, nil
}

// RemoveDestination unregisters a destination in memory. Copies already on the
// destination are not deleted. The caller must Save.
func (v *Vault) RemoveDestination(path string) error {
	root, err := cleanDestinationPath(path)
	if err != nil {
		return err
	}
	var kept []Destination
	found := false
	for _, d := range v.BackupDestinations() {
		if samePath(d.Path, root) {
			found = true
			continue
		}
		kept = append(kept, d)
	}
	if !found {
		return errors.New("kinko: その保存先は登録されていません")
	}
	return v.setDestinations(kept)
}

// FindDestination returns a registered destination by path.
func (v *Vault) FindDestination(path string) (Destination, error) {
	root, err := cleanDestinationPath(path)
	if err != nil {
		return Destination{}, err
	}
	for _, d := range v.BackupDestinations() {
		if samePath(d.Path, root) {
			return d, nil
		}
	}
	return Destination{}, errors.New("kinko: その保存先は登録されていません")
}

func readMarker(col string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(col, markerFileName))
	if err != nil {
		return "", err
	}
	marker := strings.TrimSpace(string(raw))
	if len(marker) != 32 {
		return "", ErrDestinationMismatch
	}
	return marker, nil
}

func writeMarker(col string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("kinko: 目印を作れません: %w", err)
	}
	marker := hex.EncodeToString(raw)
	f, err := os.OpenFile(filepath.Join(col, markerFileName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("kinko: 目印を書けません: %w", err)
	}
	if _, err := io.WriteString(f, marker+"\n"); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("kinko: 目印を書けません: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("kinko: 目印を書けません: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("kinko: 目印を書けません: %w", err)
	}
	return marker, syncDir(col)
}

// checkDestination confirms the destination is reachable and is the registered one.
func (v *Vault) checkDestination(d Destination) (string, error) {
	info, err := os.Stat(d.Path)
	if err != nil || !info.IsDir() {
		return "", ErrDestinationOffline
	}
	col := v.destinationCollection(d)
	marker, err := readMarker(col)
	if err != nil || marker != d.Marker {
		return "", ErrDestinationMismatch
	}
	if d.Volume != "" && volumeID(d.Path) != d.Volume {
		return "", ErrDestinationMismatch
	}
	return col, nil
}

func (v *Vault) localStore() (*generationStore, error) {
	if v.doc.VaultID == "" {
		return nil, errors.New("kinko: 保管庫の識別子がありません")
	}
	return openGenerationStore(v.doc.VaultID)
}

// pendingFor lists committed local generations that the destination lacks.
func pendingFor(local *generationStore, col string) ([]string, error) {
	files, err := local.list()
	if err != nil {
		return nil, err
	}
	var pending []string
	for _, f := range files {
		if !f.committed {
			continue
		}
		if _, err := os.Stat(filepath.Join(col, f.hash+generationExt)); errors.Is(err, os.ErrNotExist) {
			pending = append(pending, f.hash)
		} else if err != nil {
			return nil, fmt.Errorf("kinko: 保存先を確認できません: %w", err)
		}
	}
	return pending, nil
}

// unsentKeeper returns a predicate that protects generations not yet on every
// destination. An unreachable or mismatched destination protects everything.
func (v *Vault) unsentKeeper() func(string) bool {
	dests := v.BackupDestinations()
	if len(dests) == 0 {
		return nil
	}
	return func(hash string) bool {
		for _, d := range dests {
			col, err := v.checkDestination(d)
			if err != nil {
				return true
			}
			if _, err := os.Stat(filepath.Join(col, hash+generationExt)); err != nil {
				return true
			}
		}
		return false
	}
}

// Push copies committed generations that each destination lacks. It only adds
// files: nothing is deleted or overwritten at the destination. It stops copying
// when the deadline passes (zero means no deadline); remaining files stay pending.
func (v *Vault) Push(deadline time.Time) []PushResult {
	dests := v.BackupDestinations()
	if len(dests) == 0 {
		return nil
	}
	results := make([]PushResult, 0, len(dests))
	local, err := v.localStore()
	if err != nil {
		for _, d := range dests {
			results = append(results, PushResult{Path: d.Path, Err: err})
		}
		return results
	}
	for _, d := range dests {
		results = append(results, v.pushOne(local, d, deadline))
	}
	return results
}

func (v *Vault) pushOne(local *generationStore, d Destination, deadline time.Time) PushResult {
	result := PushResult{Path: d.Path}
	col, err := v.checkDestination(d)
	if err != nil {
		result.Err = err
		if files, listErr := local.list(); listErr == nil {
			for _, f := range files {
				if f.committed {
					result.Pending++
				}
			}
		}
		return result
	}
	pending, err := pendingFor(local, col)
	if err != nil {
		result.Err = err
		return result
	}
	remote := &generationStore{dir: col}
	for i, hash := range pending {
		if !deadline.IsZero() && time.Now().After(deadline) {
			result.Pending = len(pending) - i
			return result
		}
		data, err := os.ReadFile(local.path(hash))
		if err != nil || hashHex(data) != hash {
			result.Err = errors.New("kinko: 手元の世代を読めないか、内容が名前と一致しません")
			result.Pending = len(pending) - i
			return result
		}
		if _, err := remote.put(data); err != nil {
			result.Err = err
			result.Pending = len(pending) - i
			return result
		}
		result.Copied++
	}
	// Confirm the destination is still the registered one after copying.
	if _, err := v.checkDestination(d); err != nil {
		result.Err = err
	}
	return result
}

// GenerationInfo describes one local generation without opening it.
type GenerationInfo struct {
	Hash      string
	SavedAt   time.Time
	Size      int64
	Committed bool
	Live      bool
}

// LocalGenerations lists local generations, newest first.
func (v *Vault) LocalGenerations() ([]GenerationInfo, error) {
	store, err := v.localStore()
	if err != nil {
		return nil, err
	}
	files, err := store.list()
	if err != nil {
		return nil, err
	}
	live := v.SnapshotHash()
	out := make([]GenerationInfo, 0, len(files))
	for _, f := range files {
		out = append(out, GenerationInfo{Hash: f.hash, SavedAt: f.modified, Size: f.size, Committed: f.committed, Live: f.hash == live})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SavedAt.After(out[j].SavedAt) })
	return out, nil
}

// GenerationsUsage returns bytes used by local generations and the capacity.
func (v *Vault) GenerationsUsage() (used, capacity int64, err error) {
	store, err := v.localStore()
	if err != nil {
		return 0, generationCapacity, err
	}
	used, err = store.usage()
	return used, generationCapacity, err
}

// DestinationStates reports each destination's live state without copying.
func (v *Vault) DestinationStates() []DestinationState {
	var out []DestinationState
	local, localErr := v.localStore()
	for _, d := range v.BackupDestinations() {
		state := DestinationState{Destination: d}
		col, err := v.checkDestination(d)
		if err != nil {
			state.Err = err
		} else {
			state.Reachable = true
			if localErr != nil {
				state.Err = localErr
			} else if pending, err := pendingFor(local, col); err != nil {
				state.Err = err
			} else {
				state.Pending = len(pending)
			}
			if raw, err := os.ReadFile(filepath.Join(col, verifiedFileName)); err == nil {
				fields := strings.Fields(string(raw))
				if len(fields) == 2 {
					state.VerifiedPrefix, state.VerifiedAt = fields[0], fields[1]
				}
			}
		}
		out = append(out, state)
	}
	return out
}

// VerifyResult reports a destination check.
type VerifyResult struct {
	Files       int
	Damaged     int
	Generation  string
	MatchesLive bool
	Names       int
}

// VerifyDestination re-reads every copy at the destination and checks its hash,
// then opens the newest copy (the live generation when present) with password.
// The result is recorded at the destination as an opaque prefix and time.
func (v *Vault) VerifyDestination(d Destination, password string) (VerifyResult, error) {
	var result VerifyResult
	col, err := v.checkDestination(d)
	if err != nil {
		return result, err
	}
	entries, err := os.ReadDir(col)
	if err != nil {
		return result, fmt.Errorf("kinko: 保存先を読めません: %w", err)
	}
	live := v.SnapshotHash()
	var newest string
	var newestTime time.Time
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != generationExt {
			continue
		}
		hash := strings.TrimSuffix(name, generationExt)
		data, err := os.ReadFile(filepath.Join(col, name))
		result.Files++
		if err != nil || hashHex(data) != hash {
			result.Damaged++
			continue
		}
		if hash == live {
			newest, newestTime = hash, time.Now().Add(24*time.Hour)
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(newestTime) {
			newest, newestTime = hash, info.ModTime()
		}
	}
	if newest == "" {
		return result, errors.New("kinko: 保存先に開ける世代がありません")
	}
	opened, err := Open(filepath.Join(col, newest+generationExt), password)
	if err != nil {
		return result, err
	}
	result.Generation = newest
	result.MatchesLive = newest == live
	result.Names = opened.Count()
	if result.Damaged > 0 {
		return result, fmt.Errorf("kinko: 保存先に壊れた世代が %d 件あります", result.Damaged)
	}
	record := newest[:12] + " " + time.Now().UTC().Format(time.RFC3339) + "\n"
	if err := os.WriteFile(filepath.Join(col, verifiedFileName), []byte(record), 0o600); err != nil {
		return result, fmt.Errorf("kinko: 確認結果を記録できません: %w", err)
	}
	return result, nil
}

// FindGeneration resolves a hash prefix (at least 8 hex characters) to a
// generation file, searching local generations first and then destinations.
func (v *Vault) FindGeneration(prefix string) (string, error) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if len(prefix) < 8 {
		return "", errors.New("kinko: 世代は 8 文字以上の hash で指定してください（kinko backup list で確認できます）")
	}
	var dirs []string
	if store, err := v.localStore(); err == nil {
		dirs = append(dirs, store.dir)
	}
	for _, d := range v.BackupDestinations() {
		if col, err := v.checkDestination(d); err == nil {
			dirs = append(dirs, col)
		}
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var matches []string
		for _, e := range entries {
			name := e.Name()
			if filepath.Ext(name) == generationExt && strings.HasPrefix(name, prefix) {
				matches = append(matches, name)
			}
		}
		if len(matches) > 1 {
			return "", errors.New("kinko: その hash に当たる世代が複数あります。もっと長く指定してください")
		}
		if len(matches) == 1 {
			return filepath.Join(dir, matches[0]), nil
		}
	}
	return "", errors.New("kinko: その hash の世代が見つかりません")
}

// RestoreGeneration copies a generation byte-for-byte to a new file after
// checking its hash and opening it with password. It never overwrites dest
// and never touches the current vault.
func RestoreGeneration(src, dest, password string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("kinko: 世代を読めません: %w", err)
	}
	if hashHex(data)+generationExt != filepath.Base(src) {
		return errors.New("kinko: 世代の内容が名前と一致しません（壊れている可能性があります）")
	}
	if _, err := Open(src, password); err != nil {
		return err
	}
	target, err := canonicalPath(dest, true)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(target); err == nil {
		return errors.New("kinko: 復元先に既にファイルがあります。上書きはしません")
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".kinko-restore-*.tmp")
	if err != nil {
		return fmt.Errorf("kinko: 復元用の一時ファイルを作れません: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, os.ErrInvalid) {
		_ = tmp.Close()
		return fmt.Errorf("kinko: 復元先の権限を設定できません: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("kinko: 復元先へ書けません: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("kinko: 復元先を保存できません: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("kinko: 復元先を閉じられません: %w", err)
	}
	if err := os.Link(tmpName, target); err != nil {
		return fmt.Errorf("kinko: 復元先を公開できません（既にある場合は上書きしません）: %w", err)
	}
	readBack, err := os.ReadFile(target)
	if err != nil || hashHex(readBack) != hashHex(data) {
		return errors.New("kinko: 復元したファイルを読み戻して確認できません")
	}
	return nil
}
