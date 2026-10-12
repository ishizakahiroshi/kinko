---
type: reference
status: done
tags: [kinko, recovery, design]
owner: ishizakahiroshi
related: []
last_reviewed: 2026-10-12
---

# #P-20261012-004 kinko：復旧方式の検討進捗

更新日: 2026-10-12。管理Issue: https://github.com/ishizakahiroshi/kinko/issues/5 。状態の正本はIssueコメント。
専用branch: `dots/vault-recovery-design-P-20261012-004`。refは更新せず、指定parentからcommit objectを保存する。

- 固定指示 `897cab7223fc047ed8ea458583749f42776d278e` のREADME/FACTS/REVIEW/STYLE/PROGRESSを全文確認。
- source `503c7d9f10536a06ed1581e93f597fe8913bd94b` と前調査 `77be9f6ab436d4d3c451ab7d45e29f3f230342f3` を読取り、前案件は再開していない。
- 推奨1案: 現暗号方式の前後世代保存、PC外への削除非連動追記、既存passwordの別置き控え、世代を明示した復元確認。
- age公式v1とv1.3.1 API/実装のscrypt単独制約を確認。新復旧鍵は初期案の必須条件にしない。
- 初回候補 `b5167cb6c17e4a8e3573694c556f7728ee6c50f1` を独立review。中2件（耐久確定順序とqueue再構成、媒体identity照合）、低1件（status解除条件）を修正。
- 修正版 `b13f6b3c998a4e6784861cc07df09199c8b8604f` の4ファイルをreviewerが新規取得し再review完了。R1/R2/R3解消、設計資料の提出ブロッカーなし。その後の手元指摘L-01（中：未設定と設定済切断のqueue/pin分離）をDESIGN/HTML/matrixへ修正。修正版 `199a0b4ac4590c95f6b6e363ec4d95d2bec21177` を同reviewerがGitHubから新規取得して独立再review完了。L-01解消、R1–R3退行なし、提出ブロッカーなし。
- 最終提出は199a0b4のDESIGN/HTMLを変更せず、review結果と進捗記録のみ追記。
- 採用未決定・未実装。実保存先未指定。scope拡張が必要な点と未検証を分離した。
- 製品source/test/deps/workflow/CLAUDE/READMEは変更せず、build/製品test/実金庫/credential/account操作なし。
- 初回HTML静的検査、固定SHAのread-back、docs4ファイル差分、公式secrets-scan構造4規則で検出0を確認。私有watchlistは未設定。クラウドChromiumは既知socket EPERM制約。初回b5167cbの1280/500は手元が確認済みと報告。修正後最終版の実表示は手元検収待ち。
- 最終提出SHA、review対象SHA、各成果path/bytes/SHA256、read-back、残件は自己参照を避けIssue5の後続receiptへ提出する。
- 残件は手元の内容・1280/500表示検収、実利用前の保存先・控えの実態確認。今回の検討は採用待ちで止めず提出し、製品実装・接続へは進まない。
