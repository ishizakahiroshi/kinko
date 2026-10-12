---
type: reference
status: draft
tags: [kinko, explanation, dots]
owner: ishizakahiroshi
related: [FACTS.md, REVIEW.md, PROGRESS.md, STYLE.md]
last_reviewed: 2026-10-12
---

# #P-20261012-002 kinko: 利用前後の変化と嬉しい点を説明するHTML

ユーザー依頼: kinkoを使うと何がどう嬉しくなるのか、dotsにHTMLで説明させてほしい。
読者は開発用のAPIキーを扱い、AIコーディングツールを使う本人。まず利用者の体験を説明する。

## 対象と提出先

- repository: `ishizakahiroshi/kinko`（公開）。指示と成果も公開可能な内容だけにする。
- branch: `dots/use-benefits-html-P-20261012-002`。
- 挙動の固定SHA: `503c7d9f10536a06ed1581e93f597fe8913bd94b`。この候補のREADME、reference、CLI sourceを読む。mainだけで現行候補の挙動を判断しない。
- HTMLだけの案件。PR作成、既存構築PR #2への追加、main mergeは不要。
- 管理Issue: https://github.com/ishizakahiroshi/kinko/issues/3 。送信・受付・提出・検収はこのIssueへ記録。
- 成果: 同階層 `design_kinko-use-benefits_2026-10-12.html`。正式なコード変更・製品GUIではなく説明資料。

## 内容と完成する体験

1. 冒頭の一画面で「実値入り.envを金庫と参照テンプレートへ置き換える。アプリに必要な秘密を渡せる」が分かる。
2. 同じ架空の開発プロジェクトを使い、利用前と利用後を左右の図で比較する。利用前: .envにAPIキー/DBパスワードの値→AIがファイル全体を読むとまとめて会話へ。利用後: ディスクの暗号化金庫＋参照名だけの.env.tmpl→AIにはテンプレートの構造が見える→kinko runで起動アプリに値を渡す。例に実値や本物そっくりのtokenを使わない。
3. 嬉しい点は利用者の場面で説明する。「AIへ設定の構造を見せやすい」「同じ参照を使うプロジェクトの値を一箇所で更新」「暗号化したままバックアップ」。更新は次回起動時に反映され、起動済みアプリへの自動更新はない。
4. 日常の流れ: init→値省略addの非表示対話入力→listで名前確認→普段はrun→必要時export。getは値を画面へ出すことを明示し、AIに秘密を見せず使う例の中心にしない。
5. 小さな正しいコマンド例を載せる。テンプレート例は `API_TOKEN=${kinko:example-service/api-token}`。起動例 `kinko run --env-file .env.tmpl -- your-command` は実行例であり、この資料から実行しない。
6. 安全性の境界は本文の自然な説明で示す。AIが金庫を解除してgetを実行できれば値を読める。子アプリは渡された値を読め、出力・保存すれば漏れる。runは親環境も引き継ぎ、既存の全秘密を自動除去する隔離装置ではない。KINKO_PASSWORDは子へ引き継がない。
7. 現在はローカルCLIの候補。GUI/クラウド同期/チーム共有/自動更新の機能を作ったように描かない。OS端末認証はadapterと自動検査があるが実機未確認、正式配布は未完了。技術・CI・SHAは末尾detailsへまとめ、主役にしない。

## 見た目

単一HTML、CSS/図はインライン、外部依存なし、JSなし。STYLE.mdの下地CSSをそのまま埋め込み、追加CSSで左右比較と流れを作る。クリーム背景・オレンジ・余白を使う。図はinline SVGまたは意味のあるHTML配置。SVGの小さい文字、長い固定幅、表のはみ出しを避ける。幅が狭い時は比較を縦に並べる。最初の図と本文を1280pxと500px幅で手元が描画して確認する。

`<!DOCTYPE html>`直後にdocsweep-meta: type design / status planned（手元検収後done）/ tags [kinko, usage, benefits] / owner ishizakahiroshi / related [] / last_reviewed 2026-10-12 / docsweep_policy archive_with_release。

説明資料として作る。意思決定の質問、採用フォーム、価格比較、言語選定、Go/Rustの速度優位の宣伝は入れない。

## 許可と境界

許可: この固定sourceと指示の読取り、HTML/レビュー/PROGRESSの作成、管理Issueへの報告。別担当による内容の独立レビュー。成果をこのbranchの固定commit objectとして保存し、SHA/path/bytes/SHA256を提出する。Git refの更新は手元1担当が行うため不要。

禁止: source/test/依存/workflow/既存資料の変更、main/既存構築branchの更新、PR/merge/release/deploy、実vault/credential/API/passwordへのアクセス、実コマンド例の実行、新しい外部サービス、画像生成。依頼範囲を広げない。

## 受付と提出

毎回1行目は `#P-20261012-002 kinko：<内容>`。受付で番号衝突有無、読んだ固定指示SHA、HTML作成・保存能力、browserの有無、実装/独立レビュー担当の分離を返す。

提出はHTMLのcommit object（parentは送信の最終指示SHA）、path、bytes、SHA256、独立レビュー対象SHAと結果、表示検査の実行有無。保存できない場合は承認要求や取消の原文を秘密抜きで短く報告し、再試行や別のGit書込みを繰り返さず、作成済みHTML全文を会話へ渡す。手元が保存して検収する。
