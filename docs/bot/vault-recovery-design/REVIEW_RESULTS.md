---
type: reference
status: done
tags: [kinko, recovery, design]
owner: ishizakahiroshi
related: []
last_reviewed: 2026-10-12
---

# #P-20261012-004 kinko：独立レビュー記録

取得・確認日: 2026-10-12。作成者と別コンテキストのreviewerを分離。
指示parent: `897cab7223fc047ed8ea458583749f42776d278e`。
挙動source: `503c7d9f10536a06ed1581e93f597fe8913bd94b`。
前調査: `77be9f6ab436d4d3c451ab7d45e29f3f230342f3`。

## 初回独立レビュー

固定対象SHA: `b5167cb6c17e4a8e3573694c556f7728ee6c50f1`。
reviewerは指示4資料、候補DESIGN/HTML、固定source、公式ageをGitHub/Webから新規取得し照合。作者のローカルdraftをレビュー対象にしていない。暗号制約の補助調査担当とも別にした。

| ID | 重大度 | 指摘と根拠 | 対応 |
|---|---|---|---|
| R1 | 中 | DESIGN旧L102–104は確定通知とdirectory Syncの順が曖昧。候補・台帳公開の耐久確定、原本確定後queue登録前の中断回復が不足 | 前世代→変更後世代→未公開候補台帳の各file/directory耐久確定を原本置換前の必須条件へ。原本directory・read-back→確定台帳耐久化→通知の順に統一。queueは世代・確定台帳・転送receiptから再構成。DESIGN §5/12とHTMLを修正 |
| R2 | 中 | DESIGN旧L65–78/131のmount pathは、unmount後local directoryへ落ちてもhash読戻しが成功し得る | 登録時と転送前後に媒体volume／別PC接続先・share identityとmarkerを照合。pathやmarker単独では判定せず、local fallback作成なし。不明ならPC外成功を進めない。DESIGN §4/12とHTMLを修正 |
| R3 | 低・改善 | 暗号化台帳のstatus時の解除条件と旧password epochが読めない場合の扱いが不明 | 詳細status/台帳更新には該当解除sessionが必要。未解除・旧台帳不明は未確認、未転送0/保護済と推定しない。getへの追加解除なし。読めないepochの世代は整理しない。DESIGN §9とHTMLを修正 |

上記を修正版へ反映し、以下の固定SHAで独立再レビューを完了した。

## 修正後の独立再レビュー

固定対象SHA: `b13f6b3c998a4e6784861cc07df09199c8b8604f`。
reviewerはGitHubから4ファイルを新規取得し、初回との差分・全文・source整合を確認。
判定: **設計資料の提出ブロッカーなし**。R1/R2/R3は解消。新たな意味上のブロッカーは見つからなかった。

- R1: 世代・候補台帳を原本置換前に耐久化し、原本と確定台帳の耐久確認後に成功通知。queue登録前の停止も未転送集合を再構成。
- R2: 転送前後の媒体/接続先identity、local fallback禁止、不明時の外部成功停止を確認。
- R3: 詳細statusの対応解除、未確認表示、旧epoch保全を確認。
- local世代の原本directory外配置、同diskをPC全損対策としない点も整合。
- HTMLのタグ対応、meta/title/viewport、8 details、3表のスクロール、外部assets/JS/form/装飾border-leftなしを再確認。CSSと冒頭の核心は初回から不変。

再取得したGit blob SHA:
- DESIGN.md: `9caac3c5f0e21fd7f2939418ed0b11ed31f138f7`
- HTML: `e9cfd513e367dd262642cde6d0dab1fc6de3cc39`
- REVIEW_RESULTS.md: `c72b1d7eb3123941be3da440ebf26250a634d0d6`
- PROGRESS.md: `17c5b633dc31d3521f8351d36e34f1e08251bca7`

この判定はb13f6b3の内容に対するもの。その後の手元指摘L-01を以下へ記録し、修正版をもう一度同じ独立reviewerが確認する。

## 手元指摘L-01への修正

手元は初回候補b5167cbのDESIGN/HTML全文と1280/500表示を確認したと2026-10-12に報告。そこで中程度L-01を指摘: 未設定でも未転送pinを持つと、全世代を永久保持し容量枠で停止するため、localだけで開始可能という説明と矛盾する。

修正: 未設定では外部queue/pinなし、通常local保持。初回登録は現存全確定世代と最新baselineを対象にし、登録途中の停止・同時変更も再構成。設定済み切断は未転送pinを継続し容量不足でwrite停止。意図的解除/保存先変更は未転送の期間・件数・失うPC外保護を説明し本人の明示確認を要求。local世代即時削除・他dest pin解除・PC外copy削除・自動解除なし。DESIGN §4/5/10/12とHTMLへ反映。

### L-01修正後の独立再レビュー

固定対象SHA: `199a0b4ac4590c95f6b6e363ec4d95d2bec21177`。
同reviewerがGitHubから4ファイルを新規取得。判定は**設計資料の提出ブロッカーなし**。

- L-01解消: 未設定、初回設定、設定済切断、意図的解除のqueue/pinと保持を区別し、他保存先とlocal保護を維持。
- 登録台帳の耐久化、登録snapshot以降の新世代再構成、collectionの協調lockにより、登録中断・並行書込みも方式案として整合。
- R1/R2/R3の退行なし。DESIGN/HTML/検証matrix間の矛盾なし。
- HTML静的検査に問題なし。CSSと冒頭は初回から不変。実描画・製品testは実施していない。

再取得したGit blob SHA:
- DESIGN.md: `22fb89f27e7d7b38db011ef3f6f882a6ada0f808`
- HTML: `14c4d1e6b2aa1ffd5fe7b67143d872d1ee653cad`
- REVIEW_RESULTS.md: `f90b7fc19c9aa94ed18625b7e5a9a71717bdabe1`
- PROGRESS.md: `b795c54727779bd7d14ac6ba368aa153d4bd09eb`

最終提出はDESIGN/HTMLをこの再レビュー対象から変更せず、review結果と進捗記録だけ追記する。最終固定SHAのread-back・差分・全成果hashと最終確認は、自己参照を避けIssue5の後続receiptへ記録する。

## 初回で確認できたこと

- age v1仕様、v1.3.1 APIとscrypt実装、kinko依存pin・Open/Save/Exportと整合。同fileへのscrypt混在不可を正しく反映。
- 別公開鍵fileは現行Openで直接読めない。別password export、旧IDなしの橋渡し、provider path結合の説明は妥当。
- 初回secretと変更前後、read/write失敗分離、非上書き復元と逆流隔離、保持と完全消去の限界、本人判断と未実装の区別を確認。
- HTMLのmeta/title/viewport、日本語、8 details、表のoverflow wrapper、650px responsive、外部assets/JS/form/装飾border-leftなしを確認。便益・不利益・成立条件が詳細より先。

## 作成側の検査と保存確認

- 初回候補4ファイルを固定SHAからread-backし、ローカルUTF-8本文との完全一致を確認。
- Git compareで指定parentからdocs/bot/vault-recovery-design内4ファイルのみ変更、1 commit、parent一致を確認。ref更新なし。
- 初回・修正版の静的HTML検査: タグの対応、metadata、title、viewport、相対リンク、外部依存なしを確認。折りたたみ外の本文約980字（metadata/末尾を含む）。
- 固定sourceの公式secrets-scanを資料4ファイルの隔離indexへ実行: 構造4規則、検出0。KB_ROOT/FAMILY_ROOT未設定で私有watchlist検査は未実施。製品testではない。

## 未実施・残件

- R1–R3とL-01の修正後固定SHAの独立再レビューは完了。最終objectのread-back/hash確認と証跡は自己参照を避けIssue5へ記録する。
- HTML実描画は既知のクラウドChromium socket EPERM制約により未実施。無駄な再試行なし。初回b5167cbの1280/500は手元が確認済みと報告。修正後最終版は手元検収待ち。
- 製品build/test、実金庫、credential、account、実保存先接続、実復元・電源断耐久・OS実機acceptanceは未実施。設計検討の完了と製品の受入を混同しない。
- 最終提出SHAは自己参照せず、Issue5の後続receiptへ記録する。
