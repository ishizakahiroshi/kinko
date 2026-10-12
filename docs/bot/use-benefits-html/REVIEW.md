---
type: reference
status: stable
tags: [kinko, review]
owner: ishizakahiroshi
related: [README.md, FACTS.md]
last_reviewed: 2026-10-12
---

# 内容と表示の検収

実装者と別担当がHTMLと固定sourceを読む。独立担当は修正せず、review対象commit SHAと指摘をREVIEW_RESULTS.mdまたは管理Issueへ提出。

- 最初の画面だけで利用者に起きる変化が分かるか。CLI一覧やCI報告が主役になっていないか。
- 同じ例のbefore/afterで、ディスクにあるもの、AIが読めるもの、子アプリへ渡るものが区別できるか。
- 暗号化金庫・参照テンプレート・runの説明とコマンドは実装に一致するか。
- 金庫内の非参照値と、親から継承される環境変数を混同していないか。
- 完全なAI隔離、子アプリの漏洩防止、起動済みアプリの値自動更新を保証していないか。
- 実機認証・正式配布・GUI・同期・チーム共有を実装済み/検収済みと描いていないか。
- 本物の秘密、具体的token、私有URL、ローカルpath、外部script/font/imageがないか。
- 説明が1ファイルで完結し、幅が狭い時も図の対応と文字が読めるか。

browserを使えない場合は未実施と書く。手元担当が全HTMLを読んで1280px/500pxで描画し、PNGを目視して初めて全体検収とする。事実・安全境界の指摘は一括で返し、修正後の同じ提出SHAを再確認する。
