package cli

import (
	"errors"
	"fmt"
	"unicode/utf8"

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
	if utf8.RuneCountInString(password) < 8 {
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
