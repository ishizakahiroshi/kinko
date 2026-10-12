---
type: reference
status: done
tags: [kinko, recovery, research, review]
owner: ishizakahiroshi
related: [README.md, REVIEW.md, RESEARCH.md]
last_reviewed: 2026-10-12
---

# #P-20261012-003 kinko：独立レビュー記録

## 対象と担当分離

- 初回独立レビュー対象SHA: ec82ceda54bb004a48edd48efea16b2e59a861a2
- 指示／成果parent: e0e76ca57a3e4ad6de540bcd68813a3dc4cb3848
- 挙動source: 503c7d9f10536a06ed1581e93f597fe8913bd94b
- 日付: 2026-10-12（UTC）
- 調査・HTML作成と独立レビューは別担当・別context。独立担当は資料作成に関与せず、GitHub上の固定commitと8文書、固定sourceの12ファイルを新規取得して確認した。
- 本書を含む最終成果commitのSHAと、そのSHAに対する独立再照合結果は [Issue 4](https://github.com/ishizakahiroshi/kinko/issues/4) の最終報告を正本とする。本書に自分自身のcommit SHAは埋め込まない。初回の判定を修正後へ自動で引き継がない。

## 初回判定と修正

初回判定: 中1件の修正が必要。他の重大・中指摘なし。

### F-01 / 中 / TeamsとBusinessの権限名の一括表記

- 対象: HTML事故3、RESEARCH.md事故3、historyteam根拠台帳。
- 指摘: 「家族はAllow Editing、組織はEdit ItemsまたはView Item History」では、TeamsとBusinessの権限名の違いが読めない。
- 根拠: https://support.1password.com/create-share-vaults-teams/ はTeamsをAllow Editing、BusinessをEdit Items／View Item History等に分ける。 https://support.1password.com/item-history-teams/ の権限設定説明はBusiness accountが前提。 https://support.1password.com/archive-delete-items/ もTeamsとBusinessを区別する。
- 採用した修正: 「家族・TeamsはAllow Editing、BusinessはEdit ItemsまたはView Item History」と分離。HTMLは各権限の日本語説明を添え、両成果の事故3近傍に権限説明の公式URLを追加。
- 状態: 修正文を本成果へ反映済み。最終固定SHAの新規取得で閉鎖確認する。

## 手元の初回検収からの修正

初回候補ec82cedaは手元担当が幅1280／500で描画済み。以下の追加修正は独立担当F-01と区別して記録する。

1. 冒頭のPC故障からの再取得に「アカウントへ保存済みのデータ」と「再ログイン手段」の条件を同じカード内で明示。
2. HTML本文のcopy、password、account、file、device、disk、backup、provider、sign-in、cloud、email等を文脈に沿って日本語化。製品名、コマンド名、公式英語記事名、URLは維持。役職は家族の管理担当者／同じ組織の別の管理者・所有者と説明し、SSO初出に「組織の統一ログイン」を添えた。
3. 途中で欠けた書式指示を推測せず、後続の明示補足を受けて反映。HTMLはDOCTYPE直後に空白なしの `<!--docsweep-meta`。fieldsはtype: design、status: done、tags: [kinko, recovery]、owner: ishizakahiroshi、related: []、last_reviewed: 2026-10-12、docsweep_policy: archive_with_release。title末尾は「 — 2026-10-12」。この最新指定がREADMEの当初metadata指定を置き換える。
4. hero／kinko囲みの装飾border-leftを削除。クリーム／オレンジと全周枠・背景は維持。この範囲だけSTYLEの当初見た目を上書きする。
5. FACTS内のexport.go誤記の訂正はRESEARCHだけに残し、HTMLの内輪の誤記説明は削除。HTMLの実装出典は正しいinternal/cli/run.goを維持。

10事故、公式の復元条件・期限・権限、未確認事項、実装と実機確認の境界は維持した。実装変更はない。

## 独立確認済みの内容

- 項目削除30日と早期完全削除、金庫削除の恒久復元不可、アカウント削除後30日内のSupport救済は可能性という3区分。
- 項目旧版の保存日数を未確認の365日／無期限として保証していない。
- 個人・家族のcodeと組織の担当者復旧、codeでは2FA維持、担当者復旧では2FA reset。
- 同期と独立backup、Kit／codeが項目データのcopyではないこと。
- 平文1PUX／CSV、passkeys／SSO／権限の制限、旧standaloneの限定条件。
- SSOのoffline利用に必要な管理者許可、期限、端末条件。
- kinkoの暗号化export、別password、vault_id引継、非上書き、temp Sync/close置換、履歴なし、path由来provider識別。
- 使用可能なproviderからの救出はsource上の経路であり、全OSのnative acceptanceは未完了。master password紛失だけで一律不可としない。
- 初回commitの唯一parent、ahead_by=1／behind_by=0、指定folderのPROGRESS／RESEARCH／REVIEW_RESULTS／HTMLのみ。製品source/test/deps/workflow変更なし。

独立source取得: internal/vault/vault.go、internal/vault/persistence.go、internal/cli/run.go、internal/cli/unlock.go、internal/cli/commands.go、internal/unlock/service.go、internal/unlock/provider.go、internal/unlock/windows_provider.go、internal/unlock/macos_keychain.go、internal/unlock/linux_secret_service.go、main.go、docs/reference_architecture.md。固定SHAのGit blobとローカルcopyが全12ファイルで一致。

## HTML・公開前検査と未検証範囲

- 初回HTML: 38,437 bytes / SHA256 83283ec20b8327ee8c4ae623ad40b50b59d7c1e7850225d021f965de9e090ac9。
- 初回独立静的検査: DOCTYPE／meta／日付／viewport／日本語lang、タグの対応・内部リンク・重複ID、外部asset・JS・イベント属性なしを確認。全10事故に近接出典。650px以下の1列化、URL／SHA折返しあり。
- 修正後も同じ静的項目を再実行し、最新7項目meta、title末尾、左線なし、意味の保持を確認する。最終bytes／SHA256はIssue 4の提出manifestに記録。
- repoの固定source版scripts/secrets-scan.mjsを、公開資料4ファイルだけの隔離した検査用indexに対して実行。構造4patternはPASS。KB_ROOT／FAMILY_ROOT未設定のため個人watchlistは未実施。全repoのscan／CI実行を意味しない。
- 作成担当／独立担当側のブラウザ実描画は未実施。既知のChromium socket EPERM制約があり、無駄な再試行をしていない。
- 手元担当は初回候補を1280／500で描画済み。修正後の最終SHAは手元の全内容・根拠・表示再検収が必要。初回画像で最終表示PASSとはしない。
- 製品test／build、実金庫・OS credential、実1Passwordアカウント操作や破壊試験は一切行っていない。

## 独立取得した公式本文

以下23URLを全て2026-10-12（UTC）に新規取得。検索snippetだけで判定していない。RESEARCH.mdの根拠台帳全23URLと一致。

1. https://support.1password.com/backups/
2. https://support.1password.com/archive-delete-items/
3. https://support.1password.com/item-history/
4. https://support.1password.com/item-history-teams/
5. https://support.1password.com/create-share-vaults/
6. https://support.1password.com/create-share-vaults-teams/
7. https://support.1password.com/delete-account/
8. https://support.1password.com/uninstall-1password/
9. https://support.1password.com/lost-device/
10. https://support.1password.com/emergency-kit/
11. https://support.1password.com/recovery-codes/
12. https://support.1password.com/recovery-code-security/
13. https://support.1password.com/recovery/
14. https://support.1password.com/forgot-account-password/
15. https://support.1password.com/secret-key/
16. https://support.1password.com/sync/
17. https://support.1password.com/sso-security/
18. https://support.1password.com/export/
19. https://support.1password.com/1pux-format/
20. https://support.1password.com/migrate-1password-account/
21. https://support.1password.com/agilekeychain-zip/
22. https://support.1password.com/extra-copies-found/
23. https://support.1password.com/1password-security/
