# #P-20261011-012 kinko：実用 CLI の構築

2026-10-11。作業対象と指示保存先は `ishizakahiroshi/kinko`（公開）。依頼者は dots と手元 Codex の往復による構築を依頼し、通常の仕様判断を委任した。不明点は最後の HTML に残し、独立して進められる作業を継続する。

## 基準・提出先

- 挙動基準 SHA: `9d463955b06e5b25bc5c80492eb1ded4f5761fe9`。
- 指示と現行 source の branch: `dots/practical-cli-construction-P-20261011-012`。main は古く、端末解除と監査修正が未反映。main だけを読んで実装しない。
- 上記 branch の固定指示 commit を読んでから、同じ branch に実装・test・文書を保存し、main 向け Draft PR を提出する。既存修正を除去しない。main との全差分と、挙動基準からの今回差分を区別する。
- 管理 Issue: 初回公開後に本書と PROGRESS.md へ追加する。依頼者が作成したこの Issue だけに状態を記録する。
- 手元 Codex が全差分をレビューし、必要な修正を同じ会話で返す。独立 dots 担当のレビューは REVIEW.md に従う。

## 目的と完成する体験

ローカル開発者が、実値を平文設定ファイルへ残さず、保管庫を作成→値を入力→名前だけ確認→必要な1件を取得→テンプレートから子コマンドを起動→暗号化バックアップ→再度開く、という日常操作を通して使える CLI を完成させる。既存 CRUD/run/export は実装済みなので、空の新製品を作り直さず、破損・失敗・同時操作・入力誤りを含めて実用に足りない部分を補う。

## C1: 保存と読み込みを実用にする

`internal/vault/vault.go` とその caller を精査し、合成データで失敗を再現して修正する。

1. 現状は Create が Stat→Save、Save が temp→Rename で、writer coordination がない。同時 init が既存 vault を置換せず、別プロセスの add/rm が互いの更新を黙って消さない仕組みを実装する。新規依存なしの OS primitive と明確な競合エラーを優先し、読取→変更→保存を保護する。stale snapshot を新しいデータへ上書きしない。対象の alias/symlink と cleanup、失敗時の lock 解放も考慮し、保証できないケースは文書で明示する。
2. age の認証された復号ストリームを終端まで検証する。切れた暗号文・改変・JSON末尾・無効 version を正常な vault として受理しないか再現確認する。現行 format 1 と vault_id 未設定の旧 format 1 は読み込み互換を残す。独自暗号を追加しない。
3. 保存失敗、置換失敗、誤 password、競合で、元の暗号文と内容を保持する。平文の値・名前をログや一時 file に置かない。既存 destination の export 上書きを防ぐ。export は同じ論理 vault_id を保持する。
4. 変更が不要と判定した項目は、理由と実行証跡を残す。単に「テストが緑」で閉じない。

## C2: 日常操作を通して使えることを検証・修正する

`main.go`、`internal/cli/`、`internal/unlock/` の source と実際の caller を読む。合成 vault のみを使う。

1. 余分な引数、空名前、不正 option が password prompt や vault 書込み前に拒否されるか確認し、仕様と実装を整合させる。add の既存可変引数の互換性を変更する場合は利用者への影響を明記する。
2. non-TTY の連続入力、入力省略の add、get の改行なし stdout、list の値非表示、run の子 stdin/stdout/stderr・exit code、missing reference 時の未起動、password の case-insensitive 非継承を一連の test で検証する。
3. get の出力失敗を成功扱いにしないこと、値が診断へ混ざらないこと、OS provider の cancel/timeout/stale 後に黙って password fallback しないことを確認する。
4. 必要な不具合修正と、すぐ試せる予約名だけの template 例・手順を整える。README、docs/reference_cli.md、docs/reference_architecture.md を挙動と一緒に更新する。新しい GUI・network service・同期・独自暗号・大規模機能を追加しない。

## C3: 実装者から独立したレビューと提出

実装者とは別 context の dots 担当が REVIEW.md の項目を全差分で確認し、finding を修正→新 SHA で再レビューする。共有 Git 操作・共通ファイルの統合は実装担当1人が直列で所有する。最後に手元 Codex のレビューへ提出する。

必要な検査: gofmt（変更 Go のみ）、`go test -count=1 ./...`、`go vet ./...`、`node scripts/secrets-scan.mjs --all-tracked --block`。依存取得・実行ができなければ未実行と記録し、Draft PR の既存 CI を使う。Ubuntu の race 検査の常設化を `.github/workflows/ci.yml` の既存 test job へ追加してよい。CI の既存3 OSと govulncheck を削除・skip・弱化しない。依存 update が必要なら finding と対応案を返して依頼者の判断を受ける。

## 許可と境界

許可: 上記案件 branch への source/test/reference/examples/案件資料の commit・push、main 向け Draft PR 作成・更新、本管理 Issue のコメント、独立レビュー指摘の修正、合成データの test/vet/secrets-scan、既存 CI の race 検査追加。指示 branch の GitHub 公開は今回依頼の一部であり、保存の実行確認が必要なら承認 URL と対象操作だけを直ちに返す。

対象: `main.go`、`internal/**`、`README.md`、`docs/reference_*.md`、本案件 `docs/bot/practical-cli-construction/**`、`examples/**`（必要なら）、`.github/workflows/ci.yml` の race 追加のみ。

禁止: main/develop への直 push・merge、tag/release/deploy、配布 artifact の build、新規依存・課金サービス・他 repo/他案件の変更、実 vault/password/token/OS credential の取得・保存・削除、Windows PIN設定変更。実機 acceptance は手元担当の別ゲートであり、cross build・fake provider・CI を Windows Hello の利用者操作の成功として数えない。

止まる条件: 保存の外部承認要求、範囲外の変更が必須、新規依存/credential/課金が必須、同じ原因の CI failure 2回、重大 finding が2回修正しても残る、1工程90分超。該当箇所を checkpoint として残し、独立して安全に進められる項目は続ける。秘密を取得して解決しない。

## 受付・報告

同じ Slack スレッドだけで往復する。毎回1行目は `#P-20261011-012 kinko：<内容>`。受付で案件番号/衝突有無/読んだ指示 commit/対象 branch/OS/toolchain/依存 cache/test 実行可否/担当分離能力を返す。受付→実装→独立レビュー→修正提出の区切りで Issue に報告する。

提出物: Draft PR URL、code SHA、review対象 SHA、finding と修正結果、実行 command/exit code・skip・未実行、未完了実機ゲート、戻し方、通常判断と残った不明点をまとめた self-contained `results/report.html`。未解決の安全性問題を公開HTMLに詳細再現付きで放置せず、まず同じ会話へ短く報告する。会話 locator・ローカル絶対パス・実データ・秘密値を公開資料へ入れない。
