# #P-20261011-012 kinko：確認済みの根拠

確認日: 2026-10-11。公開可能な source と判定結果のみ。

| 根拠 | 確認した事実 |
|---|---|
| `CLAUDE.md`、`README.md`、`main.go` | age によるローカル Go CLI。CRUD/run/export/OS adapter が存在し、prerelease。GUI/cloud/team ACL は対象外 |
| `internal/vault/vault.go` | Create は Stat→Save、Save は同一 directory の暗号化 temp を sync/close 後に Rename。writer coordination は未実装。復号は JSON decoder 1回で、終端検証の再現確認が必要 |
| `internal/cli/config.go` | non-TTY reader は byte 単位で先読みを避ける既存監査修正がある |
| `internal/cli/unlock.go`、`internal/cli/commands.go` | init/list は余分な引数を明示拒否しない。get は fmt.Print の error を処理していない。source 上の観察であり、影響は再現して確定する |
| `internal/cli/run.go`、`run_test.go` | missing reference の fail closed、master password 非継承、stdout/exit code 契約と既存 test がある |
| 基準 `9d463955b06e5b25bc5c80492eb1ded4f5761fe9` | 既存端末解除と監査修正を含む手元の source。今回の実装はこの基準からの差分で評価する |
| main `ad842a4f56427871ccb4386d1ff125a99ab9850a` | 依頼前の公開 main。手元の基準とは異なる。main の実装済み度をそのまま current としない |
| `go test -count=1 ./...` | 手元 Windows で exit 0。cli/unlock/vault の package test 成功。実機credentialの保存や Hello 認証の証明ではない |
| `go vet ./...` | 手元 Windows で exit 0 |
| `node scripts/secrets-scan.mjs --all-tracked --block` | 基準の47 tracked files、126 needles + 4 structural patterns、exit 0 |
| `.github/workflows/ci.yml` | Windows/macOS/Linux の test/vet と Ubuntu govulncheck がある。race 常設 step は現状ない |

古いローカル計画の状態は current の証明に使わない。OS の credential 保存・利用・cancel・disable は今回未検証。実値と実機の対話操作を外部 dots へ渡さない。
