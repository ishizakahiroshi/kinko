# P-20261011-012 kinko 独立レビュー 660417bf

確認日: 2026-10-11 UTC

## 結論

**静的review上blockingなし。660417のGo検査、CI、手元検収待ち。**

対象は commit object `660417bfef87c9d615fe0c935c9a74c93f22f875` である。既報のP2 3件について、修正コードと合成回帰testの対応を再確認し、静的レビュー指摘として閉じる。ただし、このSHAのtest実行成功を確認した意味ではない。branch更新、同SHAの実行検査、手元の最終レビューが残るため、merge承認、全体完了、release-readyとは判定しない。

報告作成時点のPR headは `ffcf35c9271490636370e58224c4d4215cd4e6c5`。対象commit objectの存在とbranchへの反映は別であり、660417のPR-triggered workflow runsは0件だった。

- 対象: [660417bf](https://github.com/ishizakahiroshi/kinko/commit/660417bfef87c9d615fe0c935c9a74c93f22f875)
- Draft PR: [#2](https://github.com/ishizakahiroshi/kinko/pull/2)
- 管理Issue: [#1](https://github.com/ishizakahiroshi/kinko/issues/1)

## 独立性とレビュー方法

レビュー担当は実装を担当しておらず、source編集、Git書込み、Issue・PR投稿、実credential操作を行っていない。本報告だけを別途許可された成果物として新規作成した。

最初に固定指示commitの `docs/bot/practical-cli-construction/README.md`、`FACTS.md`、`REVIEW.md`、`PROGRESS.md` と `AGENTS.md`、`CLAUDE.md` を確認した。GitHub connectorで対象refを固定し、ローカル素材化用Git SHAを提出SHAとして使っていない。

ffcf35cのmain差分56 filesを固定refで取得してsourceを確認した。660417については親commitがffcf35c、treeが `16e1b645da44eabe2586bcf336d2eaf898acf81a` であること、変更13 filesが再レビューした素材とblob一致すること、残る43 filesもffcf35cの取得blobと一致することを確認した。

## 比較した全差分

| 比較基準 | 対象SHAまでの差分 | 判定対象 |
|---|---:|---|
| 固定指示 `6e81e57a0f2a5f9e4e6cebb6b3d3a412f6bc5b9e` | 31 files、9 commits ahead、0 behind | 今回のCLI・保存・review修正と、取り込んだmainの変更 |
| 挙動基準 `9d463955b06e5b25bc5c80492eb1ded4f5761fe9` | 35 files、11 commits ahead、0 behind | 上記に案件指示4文書を加えた差分 |
| main `ad842a4f56427871ccb4386d1ff125a99ab9850a` | 56 files、12 commits ahead、0 behind | 既存端末解除・監査修正を含むPR全体 |
| 統合候補 `ffcf35c9271490636370e58224c4d4215cd4e6c5` | 13 files、1 commit ahead、0 behind | P2修正、文書整合、native/CI修正 |

各比較のmerge-baseは指定した基準commit自身である。mainとの全差分に含まれる既存providerや既存依存を、今回の新規実装・新規依存と混同していない。

mainの暗号文fingerprintをlock内で比較する保護と、未読込Vaultによる既存pathへのSaveを `ErrConflict` とする契約は維持されている。main由来の回帰testも残る。portfolioはmainとの変更差分に含まれず、scannerはNUL区切りのファイル列挙と、既存のmasked text出力・短needle境界処理を保持する。

## P2指摘と修正確認

### R1 個別secretのnull受理と重複名

旧対象ffcf35cの `internal/vault/persistence.go:148` は `map[string]string` へ直接Unmarshalしていた。個別値のnullが空文字として成功し、重複secret名が後勝ちになり得た。利用者への影響は、不正schemaを正常なvaultとして読み、getを空出力成功にしたり、runへ意図しない空値を渡したり、後続Saveでその解釈を固定することである。ageの認証を破る問題ではない。

nullの挙動は[Go公式encoding/json文書](https://pkg.go.dev/encoding/json#Unmarshal)と一致する。手元検収担当からは、ffcf35cでage認証済みJSONのnull・duplicate-key受理を実行再現したとの報告を受けた。これは手元担当の実行証跡であり、独立レビュー環境でのGo実行ではない。

修正箇所は対象SHAの `internal/vault/persistence.go:126–142,181–213`。object内のkeyを一意に検証し、各secret値についてJSON string型を明示的に要求する。null・数値・重複名を拒否し、unknown metadataのRawMessage保持を維持する。

`TestOpenRejectsInvalidJSONVersionAndUnauthenticatedEOF` にnull値・数値・重複secretの合成caseが追加されている。各caseは暗号化fixtureを作り、Open拒否と、拒否前後のciphertext byte一致を確認する。source上の修正と回帰の対応を確認し、静的指摘を閉じる。同SHAでの実行結果は未確認。

### R2 Linux Secret Serviceの正常なno-prompt応答を失敗扱い

旧対象ffcf35cの `internal/unlock/linux_secret_service.go:93,197` はpromptを空文字と比較していた。[CreateItem](https://specifications.freedesktop.org/secret-service/latest/org.freedesktop.Secret.Collection.html)と[Item.Delete](https://specifications.freedesktop.org/secret-service/latest/org.freedesktop.Secret.Item.html)のno-prompt sentinelは `/` である。このため、保存・削除が既に成功してもCLIがunavailableを返す可能性があった。挙動基準にも存在する既存不具合で、今回の保存機構による新規導入ではない。

修正箇所は対象SHAの `internal/unlock/linux_secret_service.go:93–98,200–202,329–341`。`/` は成功、実promptは起動せず失敗、不正pathは安全な分類で拒否する。session、item、collection、SearchItemsの返却pathも検証し、root sentinelを実objectとして使わない。

`TestLinuxSecretServiceNoPromptSentinel` と `TestLinuxSecretServiceObjectPathsRejectEmptySentinels` が正常sentinel・実prompt・空／不正path・有効pathを検査する。source上の修正と合成回帰を確認し、静的指摘を閉じる。実Secret Serviceでの保存・削除成功は未確認。

### R3 Windows本人確認後のcredential読出し失敗でfallback

旧対象ffcf35cの `internal/unlock/windows_provider.go:101–107` は、Hello本人確認後のstore.read errorを共通serviceへそのまま返していた。ErrorUnavailable等はpassword fallback可能な分類であり、本人確認request後の同一invocationで追加promptを出さない契約と食い違った。これも挙動基準からの既存問題である。

修正箇所は対象SHAの `internal/unlock/windows_provider.go:105–114`。本人確認後のcontextを確認し、store.read失敗は `credential-read-failed` へ統一する。この分類はfallback対象に含まれない。未登録・削除済み・一時障害・未知errorも同じinvocationでは停止する。明示的な `--master-password` による別invocationで復旧する仕様を文書化した。未登録確認のためにcredential blob読出しを本人確認前へ移していない。

`TestWindowsPostVerificationReadFailureNeverFallsBack` は4種類のstore error、password reader未呼出し、呼出し回数、診断へのprivate detail非混入を検査する。`TestWindowsCanceledVerificationNeverReadsCredential` はcancel後にstoreを読まないことを検査する。source上の修正と回帰の対応を確認し、静的指摘を閉じる。Windows native user-presenceは未検証。

## 正当データと既存保護の回帰範囲

以下は対象SHAにtestが存在し、そのassertionとsourceを確認した範囲である。660417でのtest成功を示す表ではない。

| 保持すべき性質 | 回帰と静的確認 |
|---|---|
| 正当な空文字値 | `Test保管庫は書いた内容をそのまま読み戻せる` にempty値を含むround-tripが残る。新parserもJSON stringの空文字を許可する |
| ID未設定の旧format 1 | `Test既存format1はVaultIDなしでも開けて次回保存で付与される` が旧データ読込、値保持、次回保存でのID付与、再読込を検査する |
| unknown metadata | `TestUnknownMetadataSurvivesSaveAndExport` が旧形式sourceのSave、Export、双方の再読込とmetadata・論理vault ID保持を検査する |
| 不正schema拒否時の原本 | null・数値・重複名を含むOpen拒否caseがciphertext byte一致を検査する |
| 既存export先の非破壊 | 既存destination拒否とciphertext保持のassertionが残る。公開失敗後に他writerのdestinationを削除しない |
| stale writerと競合 | 別Open／別processのstale Save、stale削除、concurrent create、main由来ErrConflict契約の回帰が残る |
| 通常失敗とcrash | 置換失敗で元暗号文保持・lock解除・暗号化temp cleanup、crash lockを奪わない回帰が残る |
| age認証EOF | 複数chunkを跨ぐ末尾空白、truncation、tamper、append、余分なJSON、versionを扱う回帰が残る |

旧format 1 sourceのexportでは、先にsourceを保存してIDを付与する。その後のbackup公開が失敗しても、成功したID付与は残り得る。この例外はreferenceに明記されており、sourceを常にbyte不変とする保証ではない。

CLIについては引数の入力前検証、non-TTYの非先読み、getの改行なしstdoutとwrite error、listの値非表示、runのstdin/stdout/stderrとexit、missing reference時の未起動、KINKO_PASSWORDの大小文字を無視した非継承をsourceと合成subprocess testで確認した。helperはmainのdispatch/exit写像を模倣しており、実CLI executableのend-to-end検収とは分ける。

## CI修正と文書整合

[ffcf35cのCI run 38150357418](https://github.com/ishizakahiroshi/kinko/actions/runs/38150357418)のjob・step状態と失敗ログを読み、以下を区別した。

- Ubuntu: vet、test、raceがsuccess。
- Windows: vetはsuccess、testはmacOS source-contract assertionでfailure。sourceの改行依存を除くためCRLFをLFに正規化する修正を確認した。これは診断とsourceからの原因判定であり、660417の再実行による解消確認ではない。
- macOS: explicit_bzero未宣言でvetがfailure、後続testはskipped。660417は最初のinclude前で必要macroを定義し、memset_sへ変更している。[Apple公開libc仕様](https://github.com/apple-oss-distributions/Libc/blob/main/string/NetBSD/memset_s.3)を参照し、消去前のfree／CFReleaseへ順序を変えていないことを確認した。
- govulncheck: [GO-2026-4971](https://pkg.go.dev/vuln/GO-2026-4971)を検出してfailure。660417はCI toolchainをGo 1.27.2へ変更する。[公式配布情報](https://go.dev/dl/?mode=json)の該当versionを確認した。go.mod、go.sum、module dependenciesは変更していない。

3 OS test/vet、Ubuntu race、govulncheckは削除・skip・弱化していない。Windows/macOSでUbuntu専用raceがskippedになることをrace成功とは扱わない。source-contractの正規化も改行だけで、zero-before-releaseのassertionを維持する。

stdin連続入力の旧説明、Windows setupの利用可否確認とunlockの本人確認の混同、CLAUDEの「lock未実装」記述は修正されている。README/reference/examplesにおける実機未検証の区別も確認した。

## 独立レビュー担当の実行結果

対象commitとblob照合済みのsourceで実行した。

| Command／確認 | 結果 | 限界 |
|---|---|---|
| `git diff --cached --check` | exit 0 | format全般やGo構文の証明ではない。Gitへの書込みなし |
| `env -u KB_ROOT -u FAMILY_ROOT node scripts/secrets-scan.mjs --all-tracked --block` | exit 0、58 files、0 needles＋4 structural patterns | source-only。私有watchlistの検査ではない。本報告は後から新規生成したため、このtracked scanには含まれない |
| 固定refのblob照合 | 修正13 files、未変更43 filesが一致 | ローカルGit SHAを提出SHAとして使用していない |
| 660417のPR-triggered workflow取得 | 0件 | 新SHAのCIは未実施。ffcf35cの成功stepを流用しない |
| `gofmt` | 未実行 | レビュー環境にGo toolchainなし |
| `go test -count=1 ./...` | 未実行 | 同上 |
| `go vet ./...` | 未実行 | 同上 |
| `go test -race -count=1 ./...` | 未実行 | 同上 |
| OS credential保存・読出し・削除 | 未実施 | 実データと実credentialを使わない境界を維持 |

依存・toolchainの新規導入、配布artifactのbuild、実vaultの利用は行っていない。手元Windowsや旧SHA CIの結果は補助証拠であり、独立レビュー環境での実行結果へ置き換えない。

## 保証の限界

- 排他は同じprotocolを使う協調writerと、mkdir／hard link／renameを提供するローカルfilesystemが前提。同じVault値の同時goroutine利用は対象外。
- 新規作成とexportはhard linkを使う。FAT／exFAT等の非対応filesystemでは安全に失敗し、危険な上書きへfallbackしない。未検証filesystemを対応済みとは数えない。
- 既存symlinkと親directoryのsymlinkは正規化する。hard-link別名、network／sync filesystem、第三者によるdirectory・symlink・lock差替え、旧／非協調writerは保証外。
- crash lockを時間やPIDだけで自動回収しない。全writerの終了と正規化対象を確認してから、その空lock directoryだけを手動回収する。
- directory metadataのfsyncと世代backupは未実装で、完全な電源断耐性は保証しない。公開後のlock解除失敗は保存済みの可能性がある。残るtempは暗号文だけだが、cleanup失敗はあり得る。
- Windows mode bitはACL保証ではない。共有directoryのACL構成は利用者側の責任。
- fake provider、source-contract、cross-platform compile／CIはWindows Hello、macOS user-presence、Linux desktop credentialの実機acceptanceを代替しない。

## 残る提出ゲート

1. 対象commitを案件branchへ反映し、PR headの一致を再確認する。本報告時点では未反映。
2. 同じSHAでgofmt、test、vet、source/content scanを実行し、必要なら修正後の新SHAを再レビューする。
3. 同じSHAの3 OS CI、Ubuntu race、govulncheckを確認する。失敗・skipped・未実行を成功へ数えない。
4. 手元の実CLIによる合成一連操作と最終レビューを完了する。
5. native user-presenceの実機acceptanceは独立した未完了ゲートとして維持する。

本レビューの有効範囲は記載したcommit objectまでである。後続SHA、branch反映、CI成功、実機acceptanceを先取りして承認しない。
