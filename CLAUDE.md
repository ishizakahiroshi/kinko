<!-- このファイルはプロジェクト固有ルールのみを書く。個人/グローバル AI ルール
（言語・確認スタイル・出力フォーマット等）は各 AI ツールのグローバル設定へ。
fresh public clone でも有効な内容に保つこと。 -->

# kinko 開発ガイド

## プロジェクト概要

`kinko` は、開発用の秘密を age で暗号化してローカル保管し、名前を指定して 1 件ずつ
取得する Go CLI。平文の設定ファイル全体を AI CLI、検索コマンド、diff が読んだ結果、
関係のない秘密まで画面や会話ログへ流れる事故を構造的に減らす。

現在は prerelease。マスターパスワード方式の vault CRUD、名前一覧、env template を
展開する `run`、暗号化済み export、OSごとの端末解除 adapter まで実装済み。実在する
OS credentialへの保存、各OSの実機acceptance、同期、正式なrelease配布は未完了であり、
実機確認済みとして扱わない。

## やらないこと（スコープ外）

- 暗号 primitive や独自暗号 format を自作しない。暗号化は `filippo.io/age` に委ねる
- team 向け ACL、監査ログ、承認 workflow、network service、cloud sync を持たない
- GUI、Web UI、自動 updater を現段階の標準 scope にしない
- SOPS のような部分暗号化や Git 管理可能な vault を目指さない。vault は Git に入れない
- OS credential、生体認証、Secret Serviceの機能を追加・変更するときは、対象planの
  provider契約、fallback、実機acceptanceを同時に更新する。平文configやCLI wrapperで代替しない
- 侵害済み OS や悪意ある子プロセスから secret を守れると主張しない

## 技術スタック

| 分類 | 技術 |
|---|---|
| 言語 | Go 1.25.0 |
| 暗号化 | `filippo.io/age` v1.3.1 |
| 端末入力 | `golang.org/x/term` v0.45.0 |
| Linux desktop credential | `github.com/godbus/dbus/v5` v5.2.2（Linux buildのみ） |

## ディレクトリ構成

- `main.go`: command dispatch、usage、version、top-level error 表示
- `internal/cli/`: command 実装、path/password 入力、env template 展開、子 process 起動
- `internal/unlock/`: 共通解除service、vault ID照合、OS別provider、fallback分類
- `internal/vault/`: age 暗号化、versioned JSON、名前単位の CRUD、安全な file 置換
- `docs/reference_architecture.md`: 設計原則、脅威モデル、vault format、安全性の境界
- `docs/reference_cli.md`: command、環境変数、template grammar、exit/output 契約
- `scripts/secrets-scan.mjs`: public source へ実環境識別子や秘密を入れない content gate
- `.githooks/pre-commit`: staged file の secrets-scan
- `.github/workflows/secrets-scan.yml`: CI の tracked file scan

## 主要コマンド

- 全 test: `go test -count=1 ./...`
- 静的検査: `go vet ./...`
- format: `gofmt -w <変更した .go>`
- source から CLI: `go run . <command>`
- tracked file scan: `node scripts/secrets-scan.mjs --all-tracked --block`
- staged file scan: `node scripts/secrets-scan.mjs --staged --block`
- hook 有効化（Windows）: `pwsh -File scripts/install-hooks.ps1`

`go test -race -count=1 ./...` は CGO と C compiler が利用できる環境で実行する。
実行できない場合は通常 test と同一視せず、環境制約として報告する。

## AI 作業共通ルール

ビルド・コミット禁止、secrets-scan 責務、plan/bugfix/pending md の作成ルール等の AI 作業共通ルールは、各利用者のグローバル AI 設定に従う（作者環境の例: `~/.claude/CLAUDE.md` および `~/.claude/guides/`）。

### 実 secret を扱わない

- 実在する vault、password、token、host、顧客名を読まない、作らない、test に使わない
- `KINKO_VAULT` を使う検証は、OS temp 配下に新規作成した task 専用 directory だけを使う
- test data は `example.com`、`example-service` 等の明らかな予約・一般化表現に限定する
- synthetic value も会話へ出す必要はない。比較は test process 内で完結させる
- 検証用 vault と template は、対象を再確認して検証後に削除する

### 変更時に守る安全不変条件

- vault の名前と値は両方とも age 暗号文の内側に置く
- `Create` は既存 vault を上書きしない
- `Save` は既存 file への直接書き込みを避け、同一 directory の一時 file を sync/close 後に
  rename する。既存内容を先に削除しない
- vaultの必須構造を変える場合は `FormatVersion` を上げ、既存 version の読み込み互換を残す。
  現行format 1の任意`vault_id`は旧JSONが読める互換metadataとして2026-08-13に承認済み。旧writerで
  消えた場合は端末解除をstale credentialとして停止し、master passwordによる復旧を残す
- `list` は値を出さない。`get` の stdout は secret 本体だけで末尾改行を付けない
- prompt、件数、警告、成功通知を secret の stdout へ混ぜない。diagnostic は stderr を使う
- `run` は実値入り一時 file を作らない
- `run` は `KINKO_PASSWORD` を case-insensitive に子環境から除外し、template からの再注入も拒否する
- template の参照が 1 件でも missing なら、空値で子 process を起動しない
- child command の stdin/stdout/stderr と非 0 exit code を保つ
- concurrency lock は未実装。同じ vault への同時 writer を安全と仮定しない

### OS adapter の検証境界

- OS adapter は capability gate または availability 判定を、credential store の read/write/delete より先に通す。既知の unsupported 状態では後続の native API と credential store を呼ばない
- cgo / FFI を跨ぐ secret の一時コピーは zeroable storage に置き、所有権境界ごとに release 前の消去を確認する。source contract test は実機 acceptance の代わりにしない
- cross-build と test compile は buildability の証明であり、OS実機 acceptance、release readiness、production release の証明とは分ける
- 敵対レビューの finding は plan の文章だけで閉じず、コード・回帰 test・provenance・closeout を再確認してから完了扱いにする

### CLI と文書を一緒に更新する

command、alias、環境変数、template grammar、既定 path、stdout/stderr、exit code を変更したら、
同じ変更で次を確認する。

1. unit test または integration test
2. `README.md`
3. `docs/reference_cli.md`
4. security boundary が変わる場合は `docs/reference_architecture.md`

背景記事は設計開始時点の記録であり、現在の仕様を上書きしない。現行仕様の優先順位は
code/test、reference、README、記事の順とする。

### 検証の最低ライン

- Go code 変更: `gofmt`, `go test -count=1 ./...`, `go vet ./...`
- vault format / persistence 変更: round-trip、誤 password、平文非残存、既存 file 非破壊を検証
- `run` 変更: template expand、missing reference、child env、password 非継承、exit code を検証
- public 文書・fixture 変更: secrets-scan を実行
- build artifact (`kinko.exe` 等) はユーザーの明示依頼がない限り生成しない
- commit、push、release はユーザーの明示 scope がない限り行わない

## Obsidian artifacts

If `docs/obsidian/README.md` exists, use it as an index for related knowledge artifacts.
Use the repository-relative `docs/obsidian` entry. Do not write to a central absolute
path and do not silently fall back to `docs/local` when the entry is missing.

## secrets-scan（このリポジトリの配線）

書く瞬間の責務（固有名詞の一般化・fixture は合成データ等）は上記「AI 作業共通ルール」の参照先に従う。このリポジトリ固有の配線は以下:

- scanner: `scripts/secrets-scan.mjs`（手動実行: `node scripts/secrets-scan.mjs --staged --block`）
- layer 2: pre-commit hook（husky or `.githooks/`）/ layer 3: `.github/workflows/secrets-scan.yml` / layer 4: release ゲート
- env (full coverage に必要・未設定なら構造 regex のみで継続): `KB_ROOT` / `FAMILY_ROOT`。設定詳細は `scripts/secrets-scan.mjs` の冒頭コメント
- 参照実装・設計詳細: `worklog-bridge` リポの `docs/local/secrets-scan-design/`（gitignored・公開しない）

## 関連ドキュメント

| 項目 | パス |
|---|---|
| ユーザー向け README | `README.md` |
| 設計・脅威モデル | `docs/reference_architecture.md` |
| CLI 契約 | `docs/reference_cli.md` |
| 設計背景の記事 | `https://qiita.com/ishizakahiroshi/items/b999eff900b43a7a8b52` |
| Codex/他 AI 用入口 | `AGENTS.md` |
| ローカル作業ノート（非公開） | `docs/local/`（存在する場合） |
| Obsidian knowledge artifacts | `docs/obsidian/`（存在する場合。作業キューではない） |
