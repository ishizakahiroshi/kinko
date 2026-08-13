// kinko は秘密を暗号化して保管し、必要なときだけ 1 件ずつ取り出す道具。
//
// # なぜ作るのか
//
// 開発機で AI CLI を並行して動かしていると、平文の設定ファイルに置いた秘密が
// うっかり画面へ出る事故が起きる。grep が余計な行を拾う、diff が全行を出す、
// といった形で、どれも「ファイル全体を読める」ことが前提になっている。
//
// ファイル全体を読む経路そのものを無くせば、その形の事故は起こせなくなる。
// kinko は名前を指定して 1 件だけ取り出す。取り違えても手に入るのは 1 件。
//
// # 使い方
//
//	kinko init                                    保管庫を作る
//	kinko add <名前>                              秘密を入れる（値は入力を求める）
//	kinko get <名前>                              秘密を 1 件出す
//	kinko list [接頭辞]                           名前の一覧（値は出さない）
//	kinko rm <名前>                               秘密を消す
//	kinko run --env-file <テンプレ> -- <コマンド>  参照を実値へ展開して起動する
//	kinko export <出力先>                         別ファイルへ書き出す（バックアップ）
//	kinko unlock status                           端末解除の状態を表示する
//
// # 暗号は自作していない
//
// 暗号化は filippo.io/age に丸投げしている。この道具が持つのは CLI の使い勝手と
// ファイルの置き換え方だけで、鍵導出も暗号方式も age の判断に従う。
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ishizakahiroshi/kinko/internal/cli"
	"github.com/ishizakahiroshi/kinko/internal/vault"
)

// version はビルド時に埋め込む。
//
//	go build -ldflags "-X main.version=v0.1.0"
var version = "dev"

func main() {
	if err := run(); err != nil {
		// 使い方の誤りと、保管庫の状態による失敗を、同じ形で出す。
		// 利用者にとってはどちらも「次に何をすればいいか」が知りたいだけなので、
		// 種類ごとに書式を変えない。
		fmt.Fprintln(os.Stderr, err)

		if errors.Is(err, vault.ErrWrongPassword) {
			fmt.Fprintln(os.Stderr, "パスワードを確かめてください。忘れた場合、中身を取り出す手段はありません。")
		}
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	options := cli.CommandOptions{}
	if len(args) > 0 && args[0] == "--master-password" {
		options.MasterPasswordOnly = true
		args = args[1:]
	}
	if len(args) == 0 {
		usage()
		return errors.New("コマンドを指定してください")
	}

	command, args := args[0], args[1:]

	switch command {
	case "init":
		return cli.Init(args)
	case "add", "set":
		return cli.AddWithOptions(args, options)
	case "get":
		return cli.GetWithOptions(args, options)
	case "list", "ls":
		return cli.ListWithOptions(args, options)
	case "rm", "remove", "delete":
		return cli.RemoveWithOptions(args, options)
	case "run":
		return cli.RunWithOptions(args, options)
	case "export":
		return cli.ExportWithOptions(args, options)
	case "unlock":
		return cli.UnlockWithOptions(args, options)
	case "version", "--version", "-v":
		fmt.Println(version)
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("不明なコマンド: %s", command)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `kinko — 秘密を暗号化して保管し、1 件ずつ取り出す

使い方:
  kinko init                                     保管庫を作る
  kinko add <名前> [値]                          秘密を入れる（値を省くと入力を求める）
  kinko get <名前>                               秘密を 1 件出す
  kinko list [接頭辞]                            名前の一覧（値は出さない）
  kinko rm <名前>                                秘密を消す
  kinko run --env-file <テンプレ> -- <コマンド>   参照を実値へ展開して起動する
  kinko export <出力先>                          別ファイルへ書き出す（バックアップ）
  kinko unlock setup [オプション]                端末解除を登録する
  kinko unlock status                            端末解除の状態を表示する
  kinko unlock disable [オプション]              端末解除を無効にする

オプション:
  kinko --master-password <command> ...          OS providerを使わずpasswordを入力する

環境変数:
  KINKO_VAULT     保管庫の場所（既定はユーザー設定ディレクトリ配下）
  KINKO_PASSWORD  パスワード（自動処理用。手で使うときは設定しないこと）

テンプレの書き方（このファイルはリポジトリへ入れてよい）:
  DB_PASSWORD=${kinko:myapp/db_password}
  SMTP_USER=noreply@example.com
`)
}
