---
type: reference
status: draft
tags: [kinko, evidence]
owner: ishizakahiroshi
related: []
last_reviewed: 2026-10-12
---

# kinkoの確認済み現状
確認日: 2026-10-12。基準SHA: 503c7d9f10536a06ed1581e93f597fe8913bd94b。
- Go CLI、age暗号化。名前と値をともに暗号文内に置く。保存単位は金庫file。
- Create/exportは既存destinationを上書きしない。Saveは同directoryのtempを書きSync/close→rename。協調writerのlockとsnapshotで競合を拒否。
- 単一fileの破壊や故障に備えた自動世代backup、ゴミ箱、項目履歴、統合restore command、offsite同期は未実装。
- exportは手動で別のage暗号化fileを作る。source vault_idを引き継ぎ、destination passwordを変えられる。
- backupを開くにはKINKO_VAULTをそのfileへ向け、backup用master password等の解除手段を使う。OS providerにはvault passwordとIDが保存されるが、実機acceptanceは未完了。
- master password紛失だけで必ず復旧不能とは限らない。使用可能なOS provider、別passwordで作ったbackupなどを区別する。
- ファイルの保存保護、合成roundtrip成功は、実ディスク故障からの復旧やpower-loss耐久保証とは別。
根拠: internal/vault/vault.go（Create, Delete, Save）、internal/cli/run.go（ExportWithOptions）、internal/unlock/、docs/reference_architecture.md（保存、backupからの復元、安全性境界）。
参照: 上記source（全文が長く、関数と根拠を調査担当が直接確認するため）。
1Passwordの現行仕様はこのFACTSでは未確認。公式取得が本依頼の作業である。

