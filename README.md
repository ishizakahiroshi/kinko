# kinko

`kinko` は、開発用の秘密を age で暗号化して保管し、必要なときに名前を指定して
1 件ずつ取り出す小さな CLI です。

AI CLI や検索コマンドが設定ファイル全体を読んだときに、関係のない秘密までまとめて
画面やログへ流れる事故を減らすために作っています。ディスク上には暗号化した保管庫と、
参照名だけを書いたテンプレートを置きます。

> [!WARNING]
> 現在は prerelease です。主要コマンドと OS adapter の共通実装はありますが、実在の OS credential
> への保存、各 OS の実機 acceptance、バージョン付きの配布成果物、複数端末同期は未完了です。

## なぜ作ったか

平文の設定ファイルを AI コーディングツールや検索コマンドが読むと、目的とは関係のない
秘密まで一緒に画面へ出ることがあります。

- 検索パターンが広すぎて、目的の行の隣にあった API key まで出た
- 改行コードの違いで設定ファイル全体が diff になった
- 一部だけ表示するつもりが、伏せ字処理を通さず全体を出した

どれも「ファイル全体を読める」ことが事故の前提です。そこで、実値入りの設定ファイルを
置かず、名前を指定して必要な 1 件だけを取り出す形にしました。操作を取り違えた場合の
露出範囲を小さくするための CLI です。

## できること

- 保管庫全体を age のパスフレーズ暗号化で保護する
- 秘密を `サービス名/用途` のような名前で追加、取得、削除する
- 値を表示せず、名前だけを一覧する
- `${kinko:名前}` を含む env テンプレートを展開し、子プロセスだけへ渡す
- 別の暗号化済み保管庫へエクスポートする
- Windows Hello / Credential Manager、macOS Keychain、Linux Secret Serviceへ接続する共通解除境界

暗号方式は自作せず、Go の [`filippo.io/age`](https://pkg.go.dev/filippo.io/age) を
利用しています。

## インストール

Go 1.25 以降が必要です。

```sh
go install github.com/ishizakahiroshi/kinko@latest
```

ソースから試す場合は、リポジトリのルートで `go run . <command>` を使えます。

## 使い方

```sh
# 保管庫を作る。パスワードは画面に表示されません
kinko init

# 秘密を追加する。値を省くと安全な対話入力になります
kinko add example-service/api-token

# 名前だけを確認する
kinko list

# 1 件だけ標準出力へ取り出す
kinko get example-service/api-token

# 削除する
kinko rm example-service/api-token

# 別の暗号化保管庫へバックアップする
kinko export backup.age

# 端末解除の状態を確認する／明示的に登録・無効化する
kinko unlock status
kinko unlock setup --dry-run
```

`add` の値をコマンドライン引数で渡すこともできますが、プロセス一覧やシェル履歴に
残るため非推奨です。

## 子プロセスへ秘密を渡す

実値を含まないテンプレートを作ります。このテンプレートはリポジトリで管理できます。

```dotenv
API_TOKEN=${kinko:example-service/api-token}
APP_MODE=development
```

テンプレートを展開し、その環境を持つ子プロセスを起動します。

```sh
kinko run --env-file .env.tmpl -- your-command --flag
```

実値を埋めた `.env` ファイルは作りません。`KINKO_PASSWORD` を自動処理で使った場合も、
そのマスターパスワードは子プロセスの環境から除外します。テンプレート側で
`KINKO_PASSWORD` を設定することも拒否します。

ただし、渡した個別の秘密は子プロセスから見えます。子プロセスが環境変数を表示、保存、
外部送信すれば漏洩します。`kinko run` は信頼できないプログラムを安全にする仕組みでは
ありません。

## コマンド

| コマンド | 内容 |
|---|---|
| `kinko init` | 空の暗号化保管庫を作る |
| `kinko add <名前> [値]` | 秘密を追加または更新する。値の省略を推奨 |
| `kinko get <名前>` | 秘密を改行なしで標準出力へ出す |
| `kinko list [接頭辞]` | 値を出さず、名前を並べる |
| `kinko rm <名前>` | 秘密を削除する |
| `kinko run --env-file <テンプレート> -- <コマンド>` | テンプレートを展開して子プロセスを起動する |
| `kinko export <出力先>` | 別の暗号化保管庫へ全件を書き出す |
| `kinko unlock setup [--dry-run] [--yes] [--replace]` | この端末の OS credential へ登録する |
| `kinko unlock status` | provider、security level、binding、状態だけを表示する |
| `kinko unlock disable [--dry-run] [--yes]` | この端末の credential だけを削除する |
| `kinko version` | バージョンを表示する |

厳密な引数、別名、出力契約は [CLI reference](docs/reference_cli.md) を参照してください。

### 端末解除

`add`、`get`、`list`、`rm`、`run`、`export` の source vault は、TTYで実行した場合に対応する
OS providerを1回だけ試します。providerが取得した master password でvaultを開き、暗号化JSONの
`vault_id`を照合します。照合に失敗した古いcredentialは、別のpasswordへ黙ってfallbackせず停止します。

空でない `KINKO_PASSWORD`、non-TTY、`kinko --master-password <command> ...` では OS UIを出しません。
未設定・未対応・UI表示前の利用不能だけは master passwordへ安全にfallbackし、OS認証のcancel、
retry exhaustion、timeout、DeviceBusy、request開始後のasync failure、stale credentialはその invocationを
停止します。provider呼び出しには既定10秒のdeadlineを適用し、`unlock status`と`unlock setup --dry-run`
は`availability`（Windows Helloの未設定・deviceなし・policy・busyを含む）も表示します。保存せずに
状態だけを確認します。Windowsは実ビルド22000未満では未対応としてHello interopとCredential Managerを呼びません。
実保存はTTYと明示確認が必要で、既存credentialの更新には`--replace`を指定します。

format 1の任意metadataとして`vault_id`を使う方式は2026-08-13に承認済みです。旧writerが未知fieldを
落とすと端末解除はstale credentialとして停止しますが、master passwordによる復旧は残ります。別々に
作成したvaultには異なるIDを付け、export backupだけは同じ論理vaultとしてsourceのIDを引き継ぎます。

## 設計の要点

- 暗号化は [age](https://github.com/FiloSottile/age) に委ね、独自暗号を実装しない
- 値だけでなく名前も暗号化し、利用サービス名を vault file に残さない
- 保存は一時 file へ書き切ってから置き換え、途中失敗で既存 vault を直接壊さない
- env template に missing reference があれば、空値で子プロセスを起動せず失敗する
- 保管庫全体を開ける `KINKO_PASSWORD` は、`run` の子プロセスへ継承しない

詳しいデータフロー、vault format、脅威モデルは
[Architecture reference](docs/reference_architecture.md) を参照してください。

## 保管庫とパスワード

保管庫の既定位置は、OS のユーザー設定ディレクトリ配下の `kinko/vault.age` です。
`KINKO_VAULT` で別の場所を指定できます。

パスワードの解除優先順位は、空でない `KINKO_PASSWORD`、TTYでの OS provider、端末からの
非表示入力です。non-TTYでは OS UIを起動せず、既存の標準入力契約へ戻ります。
`KINKO_PASSWORD` は自動処理に使えますが、環境変数は同じユーザーの別プロセスやログから見える
場合があります。手作業では設定しないでください。OS providerを明示的に使わない場合は
`kinko --master-password <command> ...` を使います。

パスワードを忘れた場合の復旧手段はありません。`kinko export` で別パスワードの
バックアップを作り、保管庫本体とは別の安全な場所へ置いてください。

## 安全性の境界

`kinko` が減らすのは、主に「平文の設定ファイル全体を不用意に読んだため、複数の秘密が
まとめて漏れる」リスクです。次の問題までは解決しません。

- `kinko get` の出力を履歴、ログ、ファイルへ保存する操作
- 子プロセスや、その子孫プロセスによる環境変数の表示や外部送信
- 端末、ユーザーアカウント、実行中プロセス自体の侵害
- マルウェア、管理者権限、メモリダンプ、OS の swap に対する完全な防御
- チーム共有、権限分離、監査ログ、自動ローテーション、クラウド同期

## 開発

```powershell
go test -count=1 ./...
go vet ./...
```

このリポジトリでは、公開前に secrets-scan も実行します。

```powershell
node scripts/secrets-scan.mjs --all-tracked --block
```

Windows で pre-commit hook を有効にする場合は、次を 1 回実行します。

```powershell
pwsh -File scripts/install-hooks.ps1
```

## 背景

設計を始める前に、1Password、age、SOPS、Windows/macOS/Linux の標準秘密保管を
一次資料から比較しました。その過程と「既存製品で足りると分かったうえで、なぜ小さく
自作するのか」は、次の記事に残しています。

- [secret manager を自作するために、1Password と age と SOPS と OS 標準の秘密保管を白書ベースで読み解いた](https://qiita.com/ishizakahiroshi/items/b999eff900b43a7a8b52)

記事は背景と当初構想、リポジトリのコードと reference は現在の仕様です。両者が異なる
場合は、現在のコードを優先します。

1Password、SOPS + age、OS 標準の秘密保管で要件を満たせるなら、それらを選ぶ方が
適切です。`kinko` は既存製品を置き換える万能な secret manager ではなく、ローカルの
小さな用途と「一度に取り出す秘密を絞る」設計に焦点を当てています。

## ライセンス

[MIT License](LICENSE)
