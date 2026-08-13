package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
	if len(args) < 1 {
		return errors.New("使い方: kinko add <名前> [値]")
	}
	name := args[0]

	var value string
	if len(args) >= 2 {
		value = strings.Join(args[1:], " ")
		fmt.Fprintln(os.Stderr,
			"注意: 引数で渡した値はプロセス一覧とシェルの履歴に残ります。値を省略すると入力を求めます。")
	} else {
		v, err := ReadValue(fmt.Sprintf("%s の値: ", name))
		if err != nil {
			return err
		}
		value = v
	}

	path, err := VaultPath()
	if err != nil {
		return err
	}
	password, err := ReadPassword("保管庫のパスワード: ")
	if err != nil {
		return err
	}
	v, err := vault.Open(path, password)
	if err != nil {
		return err
	}
	if err := v.Set(name, value); err != nil {
		return err
	}
	if err := v.Save(password); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "保存しました: %s\n", name)
	return nil
}

// Get は秘密を標準出力へ出す。
//
// 改行を付けないのは、`$(kinko get x)` で取り込んだときに末尾の改行が
// 混ざらないようにするため。
func Get(args []string) error {
	if len(args) != 1 {
		return errors.New("使い方: kinko get <名前>")
	}

	path, err := VaultPath()
	if err != nil {
		return err
	}
	password, err := ReadPassword("保管庫のパスワード: ")
	if err != nil {
		return err
	}
	v, err := vault.Open(path, password)
	if err != nil {
		return err
	}

	value, err := v.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Print(value)
	return nil
}

// List は名前の一覧を出す（値は出さない）。
func List(args []string) error {
	path, err := VaultPath()
	if err != nil {
		return err
	}
	password, err := ReadPassword("保管庫のパスワード: ")
	if err != nil {
		return err
	}
	v, err := vault.Open(path, password)
	if err != nil {
		return err
	}

	prefix := ""
	if len(args) >= 1 {
		prefix = args[0]
	}

	count := 0
	for _, name := range v.Names() {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		fmt.Println(name)
		count++
	}
	fmt.Fprintf(os.Stderr, "%d 件\n", count)
	return nil
}

// Remove は秘密を消す。
func Remove(args []string) error {
	if len(args) != 1 {
		return errors.New("使い方: kinko rm <名前>")
	}

	path, err := VaultPath()
	if err != nil {
		return err
	}
	password, err := ReadPassword("保管庫のパスワード: ")
	if err != nil {
		return err
	}
	v, err := vault.Open(path, password)
	if err != nil {
		return err
	}
	if err := v.Delete(args[0]); err != nil {
		return err
	}
	if err := v.Save(password); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "削除しました: %s\n", args[0])
	return nil
}
