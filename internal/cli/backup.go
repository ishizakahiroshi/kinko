package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	unlockservice "github.com/ishizakahiroshi/kinko/internal/unlock"
	"github.com/ishizakahiroshi/kinko/internal/vault"
	"golang.org/x/term"
)

// autoPushLimit bounds how long add / rm wait for copying to backup
// destinations (decision D4, 2026-11-04). get / list / run never push.
var autoPushLimit = 2 * time.Second

const backupUsage = "使い方: kinko backup <status|list|add|remove|push|verify|restore> ...\n" +
	"  kinko backup status                     世代と保存先の状態を表示する\n" +
	"  kinko backup list                       手元の世代を新しい順に表示する\n" +
	"  kinko backup add <場所>                 PC 外の保存先を登録して写す\n" +
	"  kinko backup remove <場所> [--yes]      保存先の登録を外す（写しは消さない）\n" +
	"  kinko backup push                       未転送の世代を保存先へ写す\n" +
	"  kinko backup verify <場所>              保存先の写しを読み戻して開けるか確かめる\n" +
	"  kinko backup restore <世代> <復元先>    世代を新しいファイルへ復元する（上書きしない）"

// BackupWithOptions handles the backup subcommands.
func BackupWithOptions(args []string, options CommandOptions) error {
	if len(args) == 0 {
		return errors.New(backupUsage)
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "status":
		return backupStatus(rest, options)
	case "list":
		return backupList(rest, options)
	case "add":
		return backupAdd(rest, options)
	case "remove":
		return backupRemove(rest, options)
	case "push":
		return backupPush(rest, options)
	case "verify":
		return backupVerify(rest, options)
	case "restore":
		return backupRestore(rest, options)
	default:
		return fmt.Errorf("不明なbackupサブコマンド: %s\n%s", sub, backupUsage)
	}
}

type vaultSession struct {
	result *unlockservice.OpenResult
}

func (s *vaultSession) vault() *vault.Vault { return s.result.Session.Vault() }

func openForBackup(options CommandOptions) (*vaultSession, error) {
	path, err := VaultPath()
	if err != nil {
		return nil, err
	}
	result, err := options.openVault(path)
	if err != nil {
		return nil, err
	}
	return &vaultSession{result: result}, nil
}

func backupStatus(args []string, options CommandOptions) error {
	if len(args) != 0 {
		return errors.New("使い方: kinko backup status")
	}
	s, err := openForBackup(options)
	if err != nil {
		return err
	}
	v := s.vault()
	gens, err := v.LocalGenerations()
	if err != nil {
		return err
	}
	committed, candidates := 0, 0
	var latest time.Time
	for _, g := range gens {
		if g.Committed {
			committed++
			if g.SavedAt.After(latest) {
				latest = g.SavedAt
			}
		} else {
			candidates++
		}
	}
	used, capacity, err := v.GenerationsUsage()
	if err != nil {
		return err
	}
	fmt.Printf("generations: %d\n", committed)
	if !latest.IsZero() {
		fmt.Printf("latest: %s\n", latest.Local().Format(time.RFC3339))
	}
	fmt.Printf("interrupted: %d\n", candidates)
	fmt.Printf("usage: %s / %s\n", humanBytes(used), humanBytes(capacity))

	states := v.DestinationStates()
	fmt.Printf("destinations: %d\n", len(states))
	offPC := 0
	for _, st := range states {
		fmt.Printf("destination: %s\n", st.Path)
		switch {
		case errors.Is(st.Err, vault.ErrDestinationOffline):
			fmt.Println("  state: offline")
		case errors.Is(st.Err, vault.ErrDestinationMismatch):
			fmt.Println("  state: mismatch")
		case st.Err != nil:
			fmt.Println("  state: error")
		default:
			fmt.Println("  state: ok")
		}
		switch {
		case st.Volume == "":
			fmt.Println("  same-disk: unknown")
		case st.SameDisk:
			fmt.Println("  same-disk: true")
		default:
			fmt.Println("  same-disk: false")
			offPC++
		}
		if st.Reachable {
			fmt.Printf("  pending: %d\n", st.Pending)
		} else {
			fmt.Println("  pending: unknown")
		}
		if st.VerifiedAt != "" {
			fmt.Printf("  verified: %s %s\n", st.VerifiedAt, st.VerifiedPrefix)
		} else {
			fmt.Println("  verified: never")
		}
	}
	if candidates > 0 {
		fmt.Fprintln(os.Stderr, "途中で止まった変更の世代があります（kinko backup list で interrupted と表示されます）。今の保管庫は変わっていません。")
	}
	if offPC == 0 {
		fmt.Fprintln(os.Stderr, "PC 外の保存先がありません（別のディスクと確認できたものが 0 件）。PC の故障・紛失には備えられていません。")
	}
	return nil
}

func backupList(args []string, options CommandOptions) error {
	if len(args) != 0 {
		return errors.New("使い方: kinko backup list")
	}
	s, err := openForBackup(options)
	if err != nil {
		return err
	}
	gens, err := s.vault().LocalGenerations()
	if err != nil {
		return err
	}
	for _, g := range gens {
		state := "saved"
		if g.Live {
			state = "current"
		} else if !g.Committed {
			state = "interrupted"
		}
		fmt.Printf("%s  %s  %s  %s\n", g.Hash[:12], g.SavedAt.Local().Format("2006-01-02 15:04:05"), humanBytes(g.Size), state)
	}
	fmt.Fprintf(os.Stderr, "%d 件\n", len(gens))
	return nil
}

func backupAdd(args []string, options CommandOptions) error {
	args, err := positionalArgs(args)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New("使い方: kinko backup add <場所>")
	}
	s, err := openForBackup(options)
	if err != nil {
		return err
	}
	v := s.vault()
	d, err := v.AddDestination(args[0])
	if err != nil {
		return err
	}
	if err := s.result.Session.Save(); err != nil {
		return err
	}
	printSaveNotices(v)
	fmt.Fprintf(os.Stderr, "保存先を登録しました: %s\n", d.Path)
	if d.Volume == "" {
		fmt.Fprintln(os.Stderr, "注意: この保存先のディスクを確認できません。PC 外かどうかは自分で確かめてください。")
	} else if d.SameDisk {
		fmt.Fprintln(os.Stderr, "注意: この保存先は保管庫と同じディスクです。PC の故障には備えられないため、PC 外には数えません。")
	}
	return reportPush(v.Push(time.Time{}), true)
}

func backupRemove(args []string, options CommandOptions) error {
	yes := false
	var rest []string
	for _, a := range args {
		if a == "--yes" {
			if yes {
				return errors.New("--yes を重複して指定できません")
			}
			yes = true
			continue
		}
		rest = append(rest, a)
	}
	rest, err := positionalArgs(rest)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return errors.New("使い方: kinko backup remove <場所> [--yes]")
	}
	if !yes && !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("backup removeはTTYで確認するか--yesを指定してください")
	}
	s, err := openForBackup(options)
	if err != nil {
		return err
	}
	v := s.vault()
	d, err := v.FindDestination(rest[0])
	if err != nil {
		return err
	}
	pending := "不明（保存先に接続されていません）"
	for _, st := range v.DestinationStates() {
		if st.Path == d.Path && st.Reachable {
			pending = fmt.Sprintf("%d 件", st.Pending)
		}
	}
	fmt.Fprintf(os.Stderr, "この保存先へまだ写していない世代: %s。外すと、その分の変更は PC の故障で失う可能性があります。保存先の写しは消しません。\n", pending)
	if !yes {
		ok, err := confirm("この保存先の登録を外しますか")
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("backup removeを中止しました")
		}
	}
	if err := v.RemoveDestination(d.Path); err != nil {
		return err
	}
	if err := s.result.Session.Save(); err != nil {
		return err
	}
	printSaveNotices(v)
	fmt.Fprintf(os.Stderr, "保存先の登録を外しました: %s\n", d.Path)
	return nil
}

func backupPush(args []string, options CommandOptions) error {
	if len(args) != 0 {
		return errors.New("使い方: kinko backup push")
	}
	s, err := openForBackup(options)
	if err != nil {
		return err
	}
	results := s.vault().Push(time.Time{})
	if len(results) == 0 {
		return errors.New("kinko: 保存先が登録されていません（kinko backup add <場所> で登録します）")
	}
	return reportPush(results, true)
}

func backupVerify(args []string, options CommandOptions) error {
	args, err := positionalArgs(args)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New("使い方: kinko backup verify <場所>")
	}
	s, err := openForBackup(options)
	if err != nil {
		return err
	}
	d, err := s.vault().FindDestination(args[0])
	if err != nil {
		return err
	}
	result, err := s.result.Session.VerifyDestination(d)
	if err != nil {
		return err
	}
	fmt.Printf("files: %d\n", result.Files)
	fmt.Printf("opened: %s\n", result.Generation[:12])
	fmt.Printf("current: %t\n", result.MatchesLive)
	fmt.Printf("names: %d\n", result.Names)
	if !result.MatchesLive {
		fmt.Fprintln(os.Stderr, "保存先にある最新の写しは、今の保管庫より古い世代です。kinko backup push で写してから確かめ直してください。")
	}
	return nil
}

func backupRestore(args []string, options CommandOptions) error {
	args, err := positionalArgs(args)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return errors.New("使い方: kinko backup restore <世代> <復元先>")
	}
	s, err := openForBackup(options)
	if err != nil {
		return err
	}
	src, err := s.vault().FindGeneration(args[0])
	if err != nil {
		return err
	}
	if err := s.result.Session.RestoreGeneration(src, args[1]); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "復元しました: %s\n今の保管庫は変えていません。中身を確かめるには KINKO_VAULT をこのファイルへ向けて get / list を使ってください。\n", args[1])
	return nil
}

// autoPush copies new generations after add / rm, waiting at most autoPushLimit.
// A slow or missing destination never fails the change that was already saved.
func autoPush(v *vault.Vault) {
	if len(v.BackupDestinations()) == 0 {
		return
	}
	done := make(chan []vault.PushResult, 1)
	deadline := time.Now().Add(autoPushLimit)
	go func() { done <- v.Push(deadline) }()
	select {
	case results := <-done:
		_ = reportPush(results, false)
	case <-time.After(autoPushLimit + 200*time.Millisecond):
		fmt.Fprintln(os.Stderr, "kinko: PC 外の保存先への転送が時間内に終わりませんでした（未転送の分は次の変更か kinko backup push で写します）")
	}
}

// reportPush prints a value-free summary. verbose also reports success.
func reportPush(results []vault.PushResult, verbose bool) error {
	var firstErr error
	for _, r := range results {
		switch {
		case r.Err != nil:
			fmt.Fprintf(os.Stderr, "kinko: %s へ写せませんでした（未転送 %d 件）: %v\n", r.Path, r.Pending, r.Err)
			if firstErr == nil {
				firstErr = r.Err
			}
		case r.Pending > 0:
			fmt.Fprintf(os.Stderr, "kinko: %s へ %d 件写しました。未転送 %d 件（次の変更か kinko backup push で写します）\n", r.Path, r.Copied, r.Pending)
		case verbose:
			fmt.Fprintf(os.Stderr, "%s へ %d 件写しました（未転送 0 件）\n", r.Path, r.Copied)
		}
	}
	if verbose {
		return firstErr
	}
	return nil
}

func humanBytes(n int64) string {
	const unit = 1024
	switch {
	case n < unit:
		return fmt.Sprintf("%d B", n)
	case n < unit*unit:
		return fmt.Sprintf("%.1f KiB", float64(n)/unit)
	default:
		return fmt.Sprintf("%.1f MiB", float64(n)/(unit*unit))
	}
}
