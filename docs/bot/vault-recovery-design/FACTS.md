---
type: reference
status: draft
tags: [kinko, recovery, design]
owner: ishizakahiroshi
related: []
last_reviewed: 2026-10-12
---

# 確認済みの出発点
確認日: 2026-10-12。製品挙動SHA: 503c7d9f10536a06ed1581e93f597fe8913bd94b。
前調査最終資料SHA: 77be9f6ab436d4d3c451ab7d45e29f3f230342f3。前案件P-20261012-003/Issue4の調査・独立再review・手元資料検収は完了。今回の設計を採用済み/実装済みに読み替えない。
- Go CLI、ageによるfile単位の暗号化。名前と値は暗号文内。現在format1、任意vault_id。
- Create/exportは既存destinationを上書きしない。Saveは同directoryの暗号化tempを書きSync/closeしてrename、writer lockとciphertext snapshotで協調競合を拒否。directory fsyncやnetwork/sync FS排他保証なし。
- Setは現在値置換、Deleteはmapから除去し保存。自動世代backup、項目履歴、ゴミ箱、統合restore、自動PC外保存は未実装。
- ExportWithOptionsはinternal/cli/run.go。別passwordの暗号化exportが可能、blankならsource passwordを明示再利用。backupはsource vault_idを引継ぐ。
- 既存backupはKINKO_VAULTを向け対応passwordで開く。provider登録はpath由来で、OS credentialの移行を暗号文copyが行うわけではない。
- 使用可能な登録済OS providerが残ればsourceを開いて別passwordへexportする経路あり。ただし全OS native acceptance未完了。
- age.NewScryptRecipientを使用。多recipient/復旧鍵の併用可能性は本指示では未確認。公式実装/資料で調べる。
- 現行CLAUDE.mdはcloud sync/network serviceをscope外としている。提案は採用・規則変更ではない。
- 現在はprerelease。大量の実秘密を移し元fileを消す受入はしていない。
根拠: internal/vault/vault.go、persistence.go、internal/cli/run.go、internal/unlock/、docs/reference_architecture.md、CLAUDE.md。前調査の公式23URL/未確定は既存RESEARCH.mdを参照（全文が長いため）。
本人指定のbackup保管先、1Password利用有無、課金許可、保存期間は未確認。実環境を推測しない。

