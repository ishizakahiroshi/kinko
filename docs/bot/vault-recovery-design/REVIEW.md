---
type: reference
status: draft
tags: [kinko, recovery, design]
owner: ishizakahiroshi
related: []
last_reviewed: 2026-10-12
---

# 独立レビュー
作成者とは別担当が、固定成果SHAの設計/HTMLを読む。初回指示→成果の差分と、基準sourceとage公式制約を照合。
- 元の誤削除でbackupも消える/同disk全損/解除情報同時紛失への対策が実際に成立するか。
- 保存/世代保持/PC外copy/解除救済/復元確認をひとつで全解決としない。前提と限界の抜けを探す。
- age scrypt制約、format1互換、旧backup/旧provider、別passwordのcopy、vault_id取り違えを確認。独自暗号がなくても誤った組合せを可能としない。
- 保存中断、容量/権限不足、破損、誤password、競合、再試行、未転送、offline、保管先全消去、retentionを事故事例で追跡。
- 実secret平文fileやログが不要か。copyの存在・復号成功・必要項目の確認・新端末受入を混同しない。
- 普段の利用負担、初期設定/復元の手順、検証状態表示、利用者benefit基準の推奨を確認。AI案の4つを無条件採用していないか。
- 根拠あり/仮定/未確定/採用未決定/未実装の区別。CLAUDEのscope拡張を検討の段階で既存仕様にしない。
- 比較するなら実現可能な選択肢に同じ軸、推奨理由、メリット/不利益、判断時点、概算の限界がある。内部の通常判断を本人へ丸投げしない。
- HTMLは平易、日本語、核心が先、レスポンシブ、単一file/外部assetとJSなし、meta/title/出典、scope外差分なし。
REVIEW_RESULTS.mdに固定対象SHA、担当分離、finding/重大度/根拠/修正、実施と未検証を記録する。修正後新SHAを新規取得し再確認。最終self SHAはIssueの後続receiptで記録し自分のcommit内へ自己参照しない。実装testと設計検討を混同しない。

