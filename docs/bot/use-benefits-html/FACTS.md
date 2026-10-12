---
type: reference
status: stable
tags: [kinko, facts]
owner: ishizakahiroshi
related: [README.md, REVIEW.md]
last_reviewed: 2026-10-12
---

# 根拠（2026-10-12）

固定source: `503c7d9f10536a06ed1581e93f597fe8913bd94b`。

| 事実 | source |
|---|---|
| 設定全体の読取りで関係ない秘密まで会話へ出る事故を減らす目的 | root README.md 冒頭・なぜ作ったか |
| 名前と値をageの暗号文内に保存 | internal/vault/vault.go、docs/reference_architecture.md |
| listは名前のみ、getは値をstdoutへ出す | internal/cli/commands.go、root README.md |
| addの値省略で非表示対話入力 | internal/cli/commands.go、docs/reference_cli.md |
| `${kinko:名前}` を実値へ展開するが実値入り.envを作らない | internal/cli/run.go、root README.md 子プロセスへ秘密を渡す |
| 親環境を継承しKINKO_PASSWORDを大小文字無視で除く | internal/cli/run.go のchildEnvironment、同テスト |
| 未登録参照なら子コマンドを起動しない | internal/cli/run.go、docs/reference_cli.md |
| デフォルトはユーザー設定ディレクトリの共通vault、KINKO_VAULTで変更可 | internal/cli/config.go、docs/reference_cli.md |
| 値の変更後は次のOpen/runで新しい値を読む | internal/unlock/service.go、internal/cli/run.go |
| exportは既存destinationを拒否する暗号化backup。cloud syncはしない | internal/vault/vault.go、root README.md |
| Hello/Keychain/Secret Serviceの実機受入と正式配布は未完了 | root README.md prerelease注記、docs/bot/practical-cli-construction/PROGRESS.md |

構築候補の記録: https://github.com/ishizakahiroshi/kinko/pull/2 と https://github.com/ishizakahiroshi/kinko/issues/1#issuecomment-6110749337 。性能比較はなく、Rust版もない。今回のHTMLで言語優位やインストール済み/運用済みを主張しない。

実データ、私有会話URL、ローカル絶対pathはこの公開資料に入れない。サンプルは予約名・抽象的な「秘密の値」の表記だけ。
