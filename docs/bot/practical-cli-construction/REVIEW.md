# #P-20261011-012 kinko：独立レビュー

実装者と別担当が実 code SHA を全差分で読む。固定指示公開 commit→提出 SHA の今回差分と、基準 `9d463955b06e5b25bc5c80492eb1ded4f5761fe9`→提出 SHA を両方確認する。mainとの差分には既存端末解除も含まれる。

1. concurrent init/create が既存 vault を壊さず、stale add/rm/save が成功に見えて更新を消さない。競合検出・lock scope・解除・異常終了・path alias・cross-platform の根拠を読む。
2. 復号の EOF と認証、truncation/tamper、JSON schema/version、旧 format 1、unknown metadata、vault_id と export の binding を確認する。
3. 誤入力・誤 password・置換失敗・export destination 既存時に元内容を保持し、暗号化 temp 以外の平文コピーを作らない。
4. argument validation を操作前に行い、non-TTY input と run の子 input を先読みで壊さない。get の末尾改行・値以外の出力・writer error、list の非値表示、run の env と exit code を subprocess で確認する。
5. OS provider の fallback boundary、cancel/timeout/stale、native memory cleanup を既存仕様より弱めていない。
6. fake provider、source、cross-platform CI、実機 user-presence を別 proof として報告する。skip を success に数えない。
7. reference/README/examples が actual source と一致し、公開資料に real data/private path/conversation locator がない。secrets-scan の最新実行を確認する。

提出: review対象 SHA、担当の独立性、重大度・対象行・利用者への影響・合成再現・修正案、command/exit code、未確認範囲。指摘ゼロでも確認範囲を書く。修正後は新 SHA を再レビューする。手元 Codex の最終レビュー前に全体完了・release-ready としない。
