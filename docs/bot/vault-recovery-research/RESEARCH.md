---
type: reference
status: done
tags: [kinko, recovery, research]
owner: ishizakahiroshi
related: [README.md]
last_reviewed: 2026-10-12
---

# #P-20261012-003 kinko：復旧調査記録

取得日: 2026-10-12（UTC）。公開公式support/securityの本文をWeb取得して確認。検索snippetだけで確定していない。実1Passwordアカウントの操作・破壊試験はなし。
固定指示／成果parent: e0e76ca57a3e4ad6de540bcd68813a3dc4cb3848。挙動source: 503c7d9f10536a06ed1581e93f597fe8913bd94b。

## 核心

1Passwordの現行accountはPC故障時に再取得できるが、金庫全体の削除は公式に復元不可。項目削除は30日、account削除は30日内にSupportへ相談すると復元できる場合がある。この3つを同じ「削除」として扱わない。
kinkoは「正常な金庫copy」と「そのcopyを開く手段」の両方が必要。現状の手動暗号化exportを、事故前に別媒体へ残して開けることまで確認する必要がある。最小製品要件の提案までで、実装／cloud方式の決定は行わない。

## 対象・用語

- 主比較は現行サブスクリプションaccount（主に1Password 8／Web）。個人、Families、Teams、Businessの差は下記に明記。
- 1Password 7を使っていてもaccountに保存したデータとstandalone金庫は別。7以前のstandaloneは端末保存であり現行accountのserver backup、code、管理者復旧をそのまま適用しない。
- 同期: 現在の変更を他端末でも利用する仕組み。金庫削除もaccount・共有相手を含む端末へ反映する。
- server backup: 運営側の日次暗号化保存保護。利用者が全事故で任意時点へ戻せる権利・機能とは同じでない。
- 履歴／Recently Deleted: 残っている対象を旧版・削除前へ戻す機能。金庫全体・account全体の復旧には拡張しない。
- 独立backup: 元の削除／破損と切り離したcopy。本報告の設計用語であり、既存app cacheの名称ではない。
- Emergency Kit／recovery code／管理者復旧: 残存accountへアクセスするための備え。項目データの独立copyではない。

## 事故別比較

### 1. 個別項目を削除

- 1Password: 30日以内なら復元候補。Recently Deletedに残っていて、account／金庫へアクセスでき、必要な復元権限がある。
- 期限・限界: 30日経過、早期完全削除、金庫自体の削除後にはこの救済を使えない。
- kinko基準source: rmは保存時に項目を除く。組込ゴミ箱なし。削除前の別copy＋それを開く手段が必要。
- 根拠（全て2026-10-12取得）: [Archive and delete items](https://support.1password.com/archive-delete-items/)

### 2. 項目を完全削除

- 1Password: 通常の復元対象外。事前の独立した書出し等に項目が残っていれば、そのcopyに含まれる情報を救出する別経路。
- 期限・限界: Recently DeletedのDestroy permanentlyから元に戻す手順は案内されていない。日次server backupを理由に復元できるとはしない。
- kinko基準source: 通常rmにも履歴保持なし。OSゴミ箱／媒体復旧へ製品として依存できない。
- 根拠（全て2026-10-12取得）: [Archive and delete items](https://support.1password.com/archive-delete-items/) / [1Password backups](https://support.1password.com/backups/)

### 3. 値を誤って更新

- 1Password: 旧版を選んで戻す。desktop app／Webで項目の旧版を表示・復元。家族・TeamsはAllow Editing、BusinessはEdit ItemsまたはView Item History。
- 期限・限界: 取得した現行support本文に旧版の保持日数なし。365日／無制限の保証は置かない。元項目・金庫消失の救済に拡張しない。
- kinko基準source: Setは現在値を置換。旧値が残る別世代を事前に保存していなければ履歴から戻せない。
- 根拠（全て2026-10-12取得）: [View and restore previous versions of items](https://support.1password.com/item-history/) / [View and restore previous versions of items in your organization](https://support.1password.com/item-history-teams/) / [Teams／Businessの権限](https://support.1password.com/create-share-vaults-teams/)

### 4. 金庫全体を削除

- 1Password: 公式に「復元不可」。削除前に独立copyを作っていた場合だけ、その中の情報を別経路で取り戻す余地。これは削除金庫そのものの復元ではない。
- 期限・限界: 1Password accountの金庫削除は恒久的。共有先を含む端末にも反映。組込Personal／Private／Employee金庫は削除不可だが、追加金庫は対象。
- kinko基準source: 金庫fileがなくなると、別の正常copy＋対応する解除手段が必要。同じdisk上のcopyもdisk消失には耐えない。
- 根拠（全て2026-10-12取得）: [1Password backups](https://support.1password.com/backups/) / [Create and share vaults](https://support.1password.com/create-share-vaults/) / [Create, share, and manage vaults in your organization](https://support.1password.com/create-share-vaults-teams/)

### 5. アカウントを削除

- 1Password: 30日以内にSupportへ相談。個人、家族organizer、team ownerは削除後30日以内なら復元できる場合がある。本人／役割確認等はSupportの判断。
- 期限・限界: 削除直後からsign-in不可。30日後は情報が恒久削除。復元の確約ではなく、全体削除は全員に影響。通常のaccount recoveryとも別。
- kinko基準source: サービスaccountの概念なし。OS user削除でfileとproviderの両方を失う事故として確認が必要。
- 根拠（全て2026-10-12取得）: [How to delete your 1Password account](https://support.1password.com/delete-account/)

### 6. 端末のアプリを削除

- 1Password: cloud側に残れば再取得。accountに保存済みのデータとsign-in手段があれば、再導入・別端末からアクセス。事前にWeb／別端末で全項目を確認。
- 期限・限界: local dataを消す手順やmobile app削除では端末copyも失う。未同期の変更や未移行standaloneの唯一copyは別問題。
- kinko基準source: 実行fileだけの削除と金庫file削除を分ける。金庫と解除手段が残れば対応CLIで開けるというsource上の条件。
- 根拠（全て2026-10-12取得）: [How to uninstall 1Password](https://support.1password.com/uninstall-1password/) / [Sync your 1Password data](https://support.1password.com/sync/) / [Migrate standalone vaults to a 1Password account](https://support.1password.com/migrate-1password-account/)

### 7. PC紛失・ディスク故障

- 1Password: 別端末から再sign-in。1Password accountに保存済みなら端末故障と独立して残る。新端末ではpassword＋Secret Key等、2FA／SSOがあればその条件も必要。
- 期限・限界: 端末だけにあった未同期データはcloudから戻せると保証しない。データが残っても解除情報も同時に失えば次の救済が必要。
- kinko基準source: 別device／媒体にある暗号化backup＋そのpassword等。旧PCのOS providerが新PCでも使えると想定しない。
- 根拠（全て2026-10-12取得）: [1Password backups](https://support.1password.com/backups/) / [If your device was lost or stolen](https://support.1password.com/lost-device/) / [Get to know your Emergency Kit](https://support.1password.com/emergency-kit/) / [About 1Password Unlock with SSO security](https://support.1password.com/sso-security/)

### 8. password・Secret Keyを紛失

- 1Password: 残っている解除経路を確認。記入済Kit、既存端末、事前code＋email、または家族／組織の復旧担当。codeの対象は個人・家族のみ。
- 期限・限界: Kitにpasswordは自動記入されない。Secret Key単独では代用不可。codeもemailやserverを要し、2FAは維持。全経路喪失時は復旧を保証できない。
- kinko基準source: 忘れたpasswordだけで一律不可ではない。有効OS providerから開ければ別passwordのexportができるsource経路がある。実機受入は未完了。
- 根拠（全て2026-10-12取得）: [Get to know your Emergency Kit](https://support.1password.com/emergency-kit/) / [Find your Secret Key or Setup Code](https://support.1password.com/secret-key/) / [Generate and use recovery codes](https://support.1password.com/recovery-codes/) / [Recover accounts for family or team members](https://support.1password.com/recovery/) / [If you forgot your account password](https://support.1password.com/forgot-account-password/)

### 9. クラウド・回線が一時停止

- 1Password: 準備済み端末のlocal copy。通常password方式は同期済みlocal暗号文を解除して利用。SSOは管理者の生体許可、offline期限、端末条件を満たす場合。
- 期限・限界: 新端末への取得・serverを使うcode復旧は障害中に完了できると保証しない。SSO browser解除不可。同期前の最新変更は他端末にない。
- kinko基準source: 正常なlocal file＋解除手段があればnetwork serviceに依存しない。外部保存先からのdownloadが必要なら、その停止は影響する。
- 根拠（全て2026-10-12取得）: [Sync your 1Password data](https://support.1password.com/sync/) / [About 1Password Unlock with SSO security](https://support.1password.com/sso-security/) / [About the security of recovery codes](https://support.1password.com/recovery-code-security/)

### 10. サービスが長期停止・終了

- 1Password: サービス外で使えるcopyが備え。事前に取得し、保護して保存したexportと、読めるformat／移行先が残っていれば情報を救出できる。
- 期限・限界: 長期継続・終了時の復元保証は今回の公式資料では確認できない。Emergency Kit／codeだけではデータcopyにならない。desktop exportはpasskeys非対応。
- kinko基準source: 独立copyを対応CLIで開けることまで確認する。file format／利用可能なソフトを含めた復旧手順が必要。
- 根拠（全て2026-10-12取得）: [How to export your data from the 1Password apps](https://support.1password.com/export/) / [About the 1Password Unencrypted Export format](https://support.1password.com/1pux-format/) / [Get to know your Emergency Kit](https://support.1password.com/emergency-kit/) / [About the security of recovery codes](https://support.1password.com/recovery-code-security/)

## プランと解除救済の条件

- 個人: 他メンバーによる管理者復旧はない。事前生成code＋登録email、記入済Emergency Kit、残存端末の利用可能な解除などを用意する。
- Families: codeに加え、別のfamily organizerが復旧開始／完了できる。自分一人を復旧担当にしない。項目復元のAllow Editingと、金庫を消せる管理権限は別。
- Teams／Business: 現行recovery codeは非対応。administrator／owner／Recover Accounts権限groupによる他人のaccount復旧。本人emailで新credentialを設定し、担当者が完了するまで利用再開できない。2FAはreset。Business SSOでは新しいapp／browserのlinkをやり直す。
- recovery code: 事前に生成・安全に保存した有効codeが必要。email本人確認、server処理が必要で、2FAは有効のまま。生成後は最低1時間待つ。最近のsign-in、途中のsign-in、検証誤りなどによる中断では1日待機。固定の有効期限は本文に示されず、再利用可。ただし削除・置換、team/businessへの変更、family organizerによる復旧開始等で無効化される。
- Emergency Kit: passwordは記入欄なので未記入PDFだけでは足りない。Secret Keyを含むため安全に保管する。SSOまたは管理者設定で使えない場合あり。2FAの備えも別途必要。
- password／Secret Key方式の既存端末はlocal暗号文をoffline解除できる。SSOはbiometricsの管理者許可・認証cache・設定期限が条件。browser SSOはoffline解除不可。Windows非TPM／Linuxはapp再起動後internetが必要になる条件がある。これを長期サービス終了時の継続保証としない。

根拠: [Generate and use recovery codes](https://support.1password.com/recovery-codes/) / [About the security of recovery codes](https://support.1password.com/recovery-code-security/) / [Recover accounts for family or team members](https://support.1password.com/recovery/) / [Get to know your Emergency Kit](https://support.1password.com/emergency-kit/) / [About 1Password Unlock with SSO security](https://support.1password.com/sso-security/) / [View and restore previous versions of items](https://support.1password.com/item-history/) / [Create and share vaults](https://support.1password.com/create-share-vaults/) / [Create, share, and manage vaults in your organization](https://support.1password.com/create-share-vaults-teams/)

## 旧standaloneの具体例と限界

1Password 3 for Mac由来の.agilekeychain_zip backupには、正確にその拡張子であることを条件に展開して対応旧appで開く公式手順がある。1Password miniを使う旧Mac向け記事もApplication Support内のデータbackupから戻せると案内する。これは既存copyと対応appがある場合の経路であり、8への直接復元、他のbackup形式、忘れた解除情報の再発行を保証しない。旧自動backupの保持日数・世代数は確定しない。1Password 7のaccount連携は2027年初頭のretirement後に変更が反映されなくなる予定と現行export資料にあり、継続的なaccount backup用途として推奨しない。

根拠: https://support.1password.com/agilekeychain-zip/ / https://support.1password.com/extra-copies-found/ / https://support.1password.com/export/ （全て2026-10-12取得）

## 書出しと独立backup

1Password desktop 8の1PUX／CSVは暗号化されない。1PUXはZIP/JSONで、圧縮は暗号化ではない。CSVは項目種別とfieldが限定され、desktop exportにはpasskeysが含まれない。mobileのCredential Exchangeは別の移行経路。SSOではexportできず、組織では権限にも依存する。平文fileをそのままemail・online保存するのは公式の警告に反する。独立backupが必要なら、適切な暗号化保管・対応する解除手段・読出し確認を別途設計する。今回exportを実行していない。

exportは取った時点の情報の持出しであり、項目履歴・共有権限・account状態まで完全復元するimage backupと断定しない。Emergency Kitやcodeだけを保存しても、削除された項目群は入っていない。

根拠: [How to export your data from the 1Password apps](https://support.1password.com/export/) / [About the 1Password Unencrypted Export format](https://support.1password.com/1pux-format/) / [Create, share, and manage vaults in your organization](https://support.1password.com/create-share-vaults-teams/) / [Get to know your Emergency Kit](https://support.1password.com/emergency-kit/) / [About the security of recovery codes](https://support.1password.com/recovery-code-security/)

## kinko固定sourceの照合

挙動根拠は503c7d9f10536a06ed1581e93f597fe8913bd94b。指示FACTS.mdの internal/cli/export.go はこのSHAに存在しないため、実体の internal/cli/run.go を参照する。指示ファイル自体は変更しない。

- Createは既存destination拒否。Saveはlockとloaded ciphertext snapshotで協調writerの競合を拒否し、同directory tempへ暗号化書込、Sync/close、rename。新fileはexclusive hard linkで公開。[vault.go:77–101](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/vault/vault.go#L77-L101) / [251–332](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/vault/vault.go#L251-L332)。
- 保存保護は旧世代を残さない。directory fsync、自動世代backup、network/sync filesystem排他は保証外。[architecture:60–68](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/docs/reference_architecture.md#L60-L68)。
- Setはmap値を置換、Deleteはmapから除去。rmは直後にSaveする。ゴミ箱／履歴recordやrestore dispatchはない。[vault.go:169–187](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/vault/vault.go#L169-L187) / [unlock.go:183–213](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/cli/unlock.go#L183-L213) / [main.go:79–111](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/main.go#L79-L111)。
- ExportWithOptionsは共通解除serviceでsourceを開き、destination用passwordを別指定（8文字以上）できる。空欄では明示的にsession passwordを再利用。既存destination拒否。[run.go:269–326](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/cli/run.go#L269-L326)。
- Exportはvault_idと任意metadataを含むdocumentをcopyし、age scrypt recipientで暗号化保存。[persistence.go:102–115](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/vault/persistence.go#L102-L115) / [vault.go:269–286](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/vault/vault.go#L269-L286)。
- 新端末では入手済backupをKINKO_VAULTに指定し、そのpasswordで開いてproviderを設定し直す。exportはupload／sync／保持管理を行わない。[architecture:193–203](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/docs/reference_architecture.md#L193-L203)。
- OS providerはpasswordとvault IDを返し、serviceが復号・ID照合してSessionを作る。使用可能な登録済providerが残れば、記憶password紛失時でも新password exportへ進めるsource経路がある。[service.go:40–119](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/unlock/service.go#L40-L119)。
- provider lookupはpath由来。暗号文copyだけではOS credentialは移らない。同じvault_idでも、別password exportは元passwordで開けない。[provider.go:94–129](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/unlock/provider.go#L94-L129)。
- WindowsはHello承認とCredential Manager読出しのapplication-gated構成。macOSはdevice-only Keychain user-presence。Linuxはsession-protected Secret Serviceで毎回の本人確認保証なし。[windows_provider.go:94–124](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/unlock/windows_provider.go#L94-L124) / [macos_keychain.go:90–107](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/unlock/macos_keychain.go#L90-L107) / [linux_secret_service.go:25–34](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/unlock/linux_secret_service.go#L25-L34)。
- 全OSのnative acceptanceは未完了。source上の経路、合成testの存在、実機救済成功、disk故障／power loss耐久は別。[architecture:31–33](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/docs/reference_architecture.md#L31-L33)。
- 現行init／誤password時の「忘れると取り出す手段がありません」という警告は上記条件を省略している。改善提案は「全ての使用可能な解除手段を失った場合」と分けること。今回sourceは変更しない。[commands.go:44–45](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/internal/cli/commands.go#L44-L45) / [main.go:58–60](https://github.com/ishizakahiroshi/kinko/blob/503c7d9f10536a06ed1581e93f597fe8913bd94b/main.go#L58-L60)。

## 最小製品要件案（未実装・方式未決定）

1. **削除・上書きの直前へ戻れる**: 既知の正常世代を自動保持し、保持日数／世代数・削除条件を明示する。同期先の削除に巻き込まれないcopyを残す。

2. **PCごと失っても取り戻せる**: 少なくとも1copyを別device／別媒体に保管し、その世代の解除手段も別に確保する。cloud採用はここでは決定しない。

3. **戻せることをその場で確かめる**: backup日時、対象金庫、最終復元確認日時を利用者に示す。元fileを上書きせず別場所で開き、必要項目の存在を秘密を表示せず確認する。

4. **passwordを忘れたときに迷わない**: 残存provider、別passwordのbackup、全解除手段喪失を区別した案内。端末依存の解除が新PCへ移ると約束しない。

5. **復旧作業でも情報を漏らさない**: kinkoの暗号化exportを維持し、平文の一時file・ログ出力を避ける。復旧資料自体には実秘密を入れず、解除情報の保管責任と失効を示す。

6. **失敗しても最後の正常copyを壊さない**: 破損、誤password、権限不足、容量不足、競合、保存中断を安全に止める。復元先を開けるまで元copyと証拠を残す。

## 将来の事故受入案（今回未実行）

- 項目誤削除／誤更新: 一つ前の必要値を取り戻し、現在の金庫を勝手に巻き戻さない。
- 金庫file削除／破損: 正常世代を別場所に復元・解除。元の壊れたfileを上書きしない。
- PC・disk全損／新OS user: 旧providerなしで別媒体copy＋対応passwordから必要項目を回収。
- password失念・provider生存: 対象OS実機でprovider解除→新password export→再openを確認。
- password失念・providerなし／失効: 誤った成功を返さず、残るbackup／解除経路を案内。全経路なしは限界を明示。
- 別passwordのexport／古いprovider: vault_idが同じでも誤credentialを受け入れない。backup側の正しいpasswordで開く。
- 世代保持期限・同期削除・保存容量不足: どこまで戻せるか表示。最後の正常世代が消えない。最新成功時刻が偽られない。
- 保存中断・競合・復元再試行: 原本・正常backupの非破壊、失敗理由、再開手順を確認。power-loss耐久は実環境別に評価。
- network／外部保存先停止: 既存local copyを使える範囲と、未取得backupが使えない範囲を正しく表示。

## 公式根拠台帳

全URLを2026-10-12に実取得。公開日はページ表示値。引用の大量転載は避け、以下は短い要約。

### backup: 1Password backups

- URL: https://support.1password.com/backups/
- 取得日: 2026-10-12 / ページ公開日: 2025-09-10
- 対象: 現行1Password account全般
- 短い根拠・限界: サーバー上の金庫・項目を毎日backup。端末喪失は再sign-inでアクセス。金庫削除は恒久的で復元不可。backup保持世代・任意時点への利用者restore・復旧時間保証は記載なし。

### delete: Archive and delete items

- URL: https://support.1password.com/archive-delete-items/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-17
- 対象: 現行アプリ／Web、個人・家族・組織
- 短い根拠・限界: 削除項目は30日。Recently Deletedから早期完全削除も可能。Archiveは任意期間保持。家族は編集、Teamsは編集、Businessは操作別権限を確認。

### history: View and restore previous versions of items

- URL: https://support.1password.com/item-history/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-03
- 対象: 個人・家族の現行account
- 短い根拠・限界: 変更のたびに旧版を保存しdesktop/Webで復元。家族はAllow Editing。本文に旧版の保持日数なし。

### historyteam: View and restore previous versions of items in your organization

- URL: https://support.1password.com/item-history-teams/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-03
- 対象: Teams／Businessの現行account
- 短い根拠・限界: desktop/Webで旧版復元。TeamsはAllow Editing、BusinessはEdit ItemsまたはView Item History権限。本文に旧版の保持日数なし。

### vault: Create and share vaults

- URL: https://support.1password.com/create-share-vaults/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-03
- 対象: 個人・家族の現行account
- 短い根拠・限界: Delete Vaultはaccountと各端末から金庫・項目を恒久削除。共有先端末にも反映。組込Personal／Private／Employeeは削除不可。金庫管理権限と項目閲覧編集権限は別。

### vaultteam: Create, share, and manage vaults in your organization

- URL: https://support.1password.com/create-share-vaults-teams/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-07
- 対象: Teams／Business
- 短い根拠・限界: 金庫削除は共有先端末にも反映。TeamsはAllow Managing、BusinessはManage Vaultで金庫削除可能。Employee金庫は削除不可。BusinessのExport Itemsは平文exportの権限。

### account: How to delete your 1Password account

- URL: https://support.1password.com/delete-account/
- 取得日: 2026-10-12 / ページ公開日: 2026-09-03
- 対象: 個人／家族／組織accountの削除
- 短い根拠・限界: 削除開始後sign-in不可。情報の恒久削除まで30日。個人、家族organizer、team ownerは30日以内のSupport連絡で復元できる場合あり。保証ではない。全体削除は全員へ影響。Business削除はSupport案内あり。

### uninstall: How to uninstall 1Password

- URL: https://support.1password.com/uninstall-1password/
- 取得日: 2026-10-12 / ページ公開日: 2026-07-28
- 対象: 現行アプリ、旧版を含むOS別説明
- 短い根拠・限界: アプリ削除・local data除去の前に別端末／Webで全データへアクセスできるか確認。OSによってアプリとlocal data削除は別。local backupも除去される手順がある。

### lost: If your device was lost or stolen

- URL: https://support.1password.com/lost-device/
- 取得日: 2026-10-12 / ページ公開日: 2026-07-28
- 対象: 現行1Password account
- 短い根拠・限界: 端末を失ってもaccount上のデータは残り別端末でsign-inできる。盗難時のアクセス無効化などは可用性復旧とは別の対応。

### kit: Get to know your Emergency Kit

- URL: https://support.1password.com/emergency-kit/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-17
- 対象: 現行password／Secret Key方式、組織制限あり
- 短い根拠・限界: PDFにsign-in address、email、Secret Key、Setup Codeとpassword記入欄。項目データのbackupではない。SSO／管理者無効化時は保存不可。2FA使用時はその備えも必要。

### code: Generate and use recovery codes

- URL: https://support.1password.com/recovery-codes/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-17
- 対象: 個人・家族のみ。Teams／Businessは非対応
- 短い根拠・限界: 事前生成codeと登録email検証で新password・新Secret Keyを設定。2FAは維持。codeは再利用可だが置換・削除等で無効化。生成直後は最低1時間待つ。保護による中断後は1日待機。

### codesec: About the security of recovery codes

- URL: https://support.1password.com/recovery-code-security/
- 取得日: 2026-10-12 / ページ公開日: 2026-09-29
- 対象: 個人・家族のrecovery code設計
- 短い根拠・限界: codeは別経路で鍵へ到達する暗号要素。登録emailによる本人確認と1Password server処理も必要。データ複製ではない。

### admin: Recover accounts for family or team members

- URL: https://support.1password.com/recovery/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-03
- 対象: Families／Teams／Business
- 短い根拠・限界: 別の家族organizer、team administrator／owner、Recover Accounts権限groupが開始・完了。本人email手続と新password等が必要。2FAリセット。SSOは再link。復旧担当は2人以上を推奨。

### forgot: If you forgot your account password

- URL: https://support.1password.com/forgot-account-password/
- 取得日: 2026-10-12 / ページ公開日: 2026-05-14
- 対象: 現行account、OS別解除案内
- 短い根拠・限界: 記入済Kit、残存端末の生体／OS解除、code、家族／組織復旧を区別。生体／OS解除は失効するため永久の代替ではない。全経路を失ったらやり直しの案内。

### secret: Find your Secret Key or Setup Code

- URL: https://support.1password.com/secret-key/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-28
- 対象: 現行account、7／8の画面案内
- 短い根拠・限界: 既存sign-in端末／browser／Kitで確認。Secret Keyだけではpassword代用不可。既存Secret KeyをSupportが知っている／再発行できるという意味ではない。

### sync: Sync your 1Password data

- URL: https://support.1password.com/sync/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-25
- 対象: 現行membership
- 短い根拠・限界: 変更を他端末でも利用、同期済みデータは一時offline時も利用可。削除から隔離された世代backupと同じではない。

### sso: About 1Password Unlock with SSO security

- URL: https://support.1password.com/sso-security/
- 取得日: 2026-10-12 / ページ公開日: 2026-10-06
- 対象: 主にBusinessのUnlock with SSO。password方式比較も掲載
- 短い根拠・限界: 通常password方式はlocal暗号文をpasswordでoffline解除。SSOは管理者の生体許可・期限・端末条件でoffline能力が異なる。browser SSOはoffline解除不可。

### export: How to export your data from the 1Password apps

- URL: https://support.1password.com/export/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-28
- 対象: desktop版8の1PUX／CSV、7旧形式、mobile交換
- 短い根拠・限界: desktop書出しは平文。CSVはLogin／Passwordの限定field。desktopはpasskeys書出し不可。SSO利用者はexport不可。mobile交換とfile exportを区別。

### pux: About the 1Password Unencrypted Export format

- URL: https://support.1password.com/1pux-format/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-17
- 対象: 1PUX
- 短い根拠・限界: ZIP内のJSONと添付fileによる持出し形式。暗号化archiveではない。別appへの移行に使えるが全サービス状態の完全再現保証ではない。

### legacy: Migrate standalone vaults to a 1Password account

- URL: https://support.1password.com/migrate-1password-account/
- 取得日: 2026-10-12 / ページ公開日: 2026-08-17
- 対象: 1Password 7以前のstandaloneと8への移行
- 短い根拠・限界: standaloneはaccount外で端末に保存。8はmembership必須。移行済accountの救済と未移行standaloneを区別。

### legacyzip: How to open a .agilekeychain_zip backup

- URL: https://support.1password.com/agilekeychain-zip/
- 取得日: 2026-10-12 / ページ公開日: 2026-05-01
- 対象: 1Password 3 for Mac由来の旧backup限定
- 短い根拠・限界: 拡張子が正確に.agilekeychain_zipのfileを展開し、対応旧appで開く案内。8や他形式へ一般化しない。保持数・解除情報紛失救済を保証しない。

### legacymac: If you see extra copies of 1Password found

- URL: https://support.1password.com/extra-copies-found/
- 取得日: 2026-10-12 / ページ公開日: 2026-04-28
- 対象: 1Password miniを使う旧Mac appの文脈
- 短い根拠・限界: appとデータを分け、Application Support内データをbackupする案内。残るbackupからデータを戻せる。app削除補助ツールはデータごと消す危険あり。

### security: About the 1Password security model

- URL: https://support.1password.com/1password-security/
- 取得日: 2026-10-12 / ページ公開日: 2026-07-28
- 対象: 現行account
- 短い根拠・限界: end-to-end暗号化。通常はaccount passwordとSecret Keyで保護。暗号の機密性は、削除や媒体喪失から戻せる保証とは別。

## 未確定点と検証の境界

1. 現行item-history記事は旧版の保存日数を明記しないため、365日／無制限とは断定していない。
2. server日次backupの世代数・保持期間・顧客別point-in-time restore・災害時RPO/RTOは取得本文から確認できない。金庫削除の「復元不可」は明示されており、ここを未確認とは扱わない。
3. account削除後30日内のSupport救済は可能性であり、個別の本人確認・復旧成功・所要時間は未確認。team memberの削除／suspensionと組織全体削除を同一条件としない。
4. legacy standaloneの現行公式ページは移行案内が中心。旧backupのOS別詳細・保持数・対応versionは取得資料だけで確定しない。既存backupと対応旧app、元の解除情報を確認する必要がある。現行membershipの救済を遡って保証しない。
5. サービス長期停止／終了での無期限利用、SSO障害中の復旧、未同期データの回収は保証未確認。通常のoffline利用とは分ける。
6. この調査では1Password実account、kinko実vault、OS credentialを操作していない。製品test／buildも未実行。
7. HTML実描画は既知のChromium socket EPERM制約のため未実施。静的構造・外部依存・hash確認と、手元の幅1280/500表示検収を分ける。
