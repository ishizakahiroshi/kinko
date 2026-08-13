package cli

import (
	"errors"
	"fmt"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

// Init は新しい保管庫を作る。
func Init(args []string) error {
	path, err := VaultPath()
	if err != nil {
		return err
	}

	password, err := ReadPassword("保管庫のパスワード: ")
	if err != nil {
		return err
	}
	if len(password) < 8 {
		return errors.New("kinko: パスワードは8文字以上にしてください")
	}

	// 打ち間違えたまま作ると、次に開くときまで気づけない。
	// そのときには中身が入っていて、取り出せない状態になっている。
	confirm, err := ReadPassword("もう一度入力してください: ")
	if err != nil {
		return err
	}
	if password != confirm {
		return errors.New("kinko: パスワードが一致しません")
	}

	v, err := vault.Create(path, password)
	if err != nil {
		return err
	}

	fmt.Printf("保管庫を作りました: %s\n", v.Path())
	fmt.Println("パスワードを忘れると中身は取り出せません。復旧手段はありません。")
	return nil
}

// Add は秘密を追加・更新する。
//
//	kinko add <名前>            値は入力を求める（画面に出ない）
//	kinko add <名前> <値>       値を引数で渡す（ps と履歴に残るので非推奨）
func Add(args []string) error {
	return AddWithOptions(args, CommandOptions{})
}

// Get は秘密を標準出力へ出す。
//
// 改行を付けないのは、`$(kinko get x)` で取り込んだときに末尾の改行が
// 混ざらないようにするため。
func Get(args []string) error {
	return GetWithOptions(args, CommandOptions{})
}

// List は名前の一覧を出す（値は出さない）。
func List(args []string) error {
	return ListWithOptions(args, CommandOptions{})
}

// Remove は秘密を消す。
func Remove(args []string) error {
	return RemoveWithOptions(args, CommandOptions{})
}
