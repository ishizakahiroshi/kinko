---
type: reference
status: stable
tags: [architecture, security, age]
last_reviewed: 2026-10-11
---

# kinko architecture reference

この文書は、`kinko` の現在の設計、安全性の境界、暗号化保管庫の扱いを説明する。
現行仕様の正本は Go ソースとテストであり、この文書と差がある場合はコードを優先する。

## 背景と現在地

設計の出発点は、平文の設定ファイルを検索したとき、目的とは関係のない秘密まで同じ
画面や AI CLI の会話ログへ流れたことだった。注意力やプロンプトだけに頼らず、ファイル
全体を読んでも実値へ到達できない配置へ変えることを狙っている。

背景調査と当初構想は、次の記事にまとめている。

- [secret manager を自作するために、1Password と age と SOPS と OS 標準の秘密保管を白書ベースで読み解いた](https://qiita.com/ishizakahiroshi/items/b999eff900b43a7a8b52)

記事は 2026 年 7 月時点の構想を含む。現在の実装との差は次のとおり。

| 項目 | 現在の状態 |
|---|---|
| Go と `filippo.io/age` を使う | 実装済み |
| `init`, `add`, `get`, `list`, `run`, `export` | 実装済み |
| 秘密の削除 | `rm` として実装済み |
| マスターパスワード | 実装済み |
| Windows Hello + Credential Manager 連携 | build 22000以上のadapter実装済み。Windows 11実機acceptance未完了 |
| macOS Keychain user presence | `darwin && cgo` adapter実装済み。macOS実機acceptance未完了 |
| Linux Secret Service 連携 | `session-protected` adapter実装済み。対象desktop acceptance未完了 |
| GitHub Releases の配布成果物 | 未提供 |

未提供項目と実機acceptance待ちは、提供時期を約束するロードマップではない。

## 設計原則

### 1. 一度に公開する秘密を最小化する

`get` は名前を指定して 1 件だけ返す。`list` は名前だけを返し、値を混ぜない。
取り違えや不用意な画面出力が起きても、保管庫全件が同時に露出しにくい形にする。

### 2. ディスクへ平文の env ファイルを作らない

`run` は参照名だけのテンプレートをメモリ上で展開し、子プロセスの環境へ渡す。
実値を埋めた一時 `.env` は作らない。

### 3. 暗号は自作しない

パスフレーズ暗号化と復号は `filippo.io/age` の scrypt recipient / identity に委ねる。
`kinko` が実装するのは、JSON の格納形式、CLI、名前単位の操作、ファイル置換である。

age の仕様と実装は以下を参照する。

- https://age-encryption.org/
- https://github.com/FiloSottile/age

### 4. 既存の保管庫を直接書きつぶさない

保存時は同じディレクトリに一時ファイルを書き、flush と close を終えてから対象パスへ
rename する。暗号文の書き込み途中で失敗したとき、既存ファイルを半端な内容へ変えない
ための設計である。

これは完全なクラッシュ耐性を意味しない。ディレクトリ metadata の fsync、複数世代の
自動バックアップは実装していない。現在は writer lock と暗号文 snapshot 比較で協調 writer の
競合を拒否する。network/sync filesystem や非協調 writer まで排他できるものではない。

### 5. 強い資格情報を子プロセスへ渡さない

`KINKO_PASSWORD` は保管庫全体を開ける。`run` が渡す個別 secret より強いため、親環境に
存在しても子環境から除外する。Windows の環境変数名は大文字小文字を区別しないため、
除外とテンプレートの予約名判定は全 OS で case-insensitive に行う。

## データフロー

```text
vault.age                 .env.tmpl
(age 暗号文)              (参照名のみ)
    |                         |
    | 復号                    | 読み込み
    +-----------+-------------+
                |
          kinko run のメモリ
          ${kinko:name} を展開
                |
                | 個別値だけを環境変数へ追加
                | KINKO_PASSWORD は除外
                v
             子プロセス
```

子プロセスが起動した後は、そのプロセスと子孫が渡された個別値を読める。プロセス終了時に
OS が環境を破棄するが、`kinko` は Go runtime 内の文字列を明示的にゼロ化しない。

## 端末解除のデータフロー

通常のTTY commandは `internal/unlock` の同じserviceを使う。空でない `KINKO_PASSWORD` は
OS UIより先に使い、non-TTYまたは `--master-password` ではproviderを呼ばない。TTYでは
providerを1 invocationにつき最大1回呼ぶ。

```text
TTY command
    |
    +-- KINKO_PASSWORDあり ----------------------> age vault open
    |
    +-- OS provider unlock ----------------------> password + vault_id
              |                                      |
              | provider failure before UI           +-- vault.Open + ID照合
              |                                      |       |
              +-- master password fallback           |       +-- success: same session
                                                     |       +-- mismatch/open error: stop
                                                     v
                                              add/get/list/rm/run/export
```

OS credentialにはvault pathそのものではなく、正規化絶対pathをhashしたroute keyを使う。
credential payloadの`vault_id`と暗号化JSON内のIDを照合するため、同じpathへ別vaultを置いた
場合の取り違えを黙ってpassword promptへ流さない。WindowsはRtlGetVersionでbuild 22000以上かを先に
確認し、旧buildではHello interopとCredential Managerを呼ばない。対応buildではCheckAvailabilityAsyncで利用可能性と
active application HWNDを確認し、UserConsentVerifierの成功後に
Credential Managerを読む`application-gated`、macOSはKeychain access controlに任せる
`credential-bound`、Linux Secret Serviceはlogin keyringの状態を保証しないため
`session-protected`と表示する。

## 保管庫の形式

暗号化前の論理形式は、version と名前から値への map を持つ JSON である。

```json
{
  "version": 1,
  "vault_id": "16 bytes as lowercase hex, optional for legacy format 1",
  "secrets": {
    "example-service/api-token": "<secret value>"
  }
}
```

この JSON 全体を age で暗号化する。値だけでなく名前も暗号文の中に入るため、保管庫
ファイルを読んだだけでは、利用サービスや用途の名前も分からない。

現在の format version は `1`。version は整数の `1` だけを受理し、未指定・0・負数・null は拒否する。
future version は更新を促す。`secrets` は object が必須で、未指定・null は拒否する。
age 復号後の JSON object は1個だけとし、二度目の Decode が認証済み EOF に到達することを要求する。
これにより複数 age chunk にまたがる末尾空白の先の切断・改変も拒否する。JSON 末尾の別 object / garbage、
重複 top-level field も拒否する。未知の任意 metadata は次回保存・export でも保持する。
JSON / stream error の詳細に平文が混ざらないよう、診断は固定文面にする。2026-08-13に承認した方式では`vault_id`を旧format 1に無い場合も読める任意metadataとして扱い、
新規作成または次回の成功した保存で付与する。旧binaryが未知fieldを落として保存するとIDが消えるため、
端末解除はstale credentialとして停止するが、master passwordによる復旧は残る。別々に作成したvaultには
異なるIDを付け、同じpasswordでも取り違えを検出する。export backupはsourceのIDを引き継ぎ、同じ論理vault
として扱う。必須構造を変える場合はversionを上げ、既存形式の読み込み互換を維持するformat 2 child planへ分離する。

## 保存手順

`Create` / `Open` は絶対パス化し、対象および親 directory の symlink を解決する。
対象は通常 file に限り、dangling symlink は拒否する。`Open` は暗号文の SHA-256 fingerprint を
memory に保持する。これは更新検出用で、暗号認証の代替ではない。

`Vault.Save` は次の順序で処理する。

1. `<正規化対象>.lock` を排他的 mkdir で作る。存在すれば待たずに `ErrLocked`
2. lock 内で現在の暗号文 fingerprint と読込時 snapshot を比較。不一致・削除は `ErrConflict`
3. 任意の vault ID が無ければ付与し、JSON 全体を age の暗号文へ変換
4. 同じ directory に暗号化 temp を作り、Unix 系では mode `0600` にする
5. 暗号文を write、file sync、close
6. 更新は temp を対象へ rename。新規作成 / export は hard link で完成済み temp を公開し、
   既存の名前があれば原子的に拒否する。Stat→Rename だけで新規を作らない
7. 成功後に snapshot を更新。自分の temp を best-effort 削除し、lock を解除

公開前の保存・置換失敗では元の暗号文を保持する。競合した snapshot は自動再試行しない。
`export` は空の destination を先に作らず、公開失敗後に destination を削除しない。
旧 format 1 の source に ID が無い場合は source を先に保存し、同じ ID を copy する。
backup の公開が失敗しても、この互換 metadata の付与は残り得る。

lock は異常終了後も残る。時間や PID で奪うと生存 writer を誤判定できるため、自動回収しない。
すべての writer が終了したことと正規化対象を利用者が確認してから、空の lock だけを取り除く。
通常エラーでは defer で解放し、lock 解放の失敗も報告する。公開後の lock 解放失敗では保存自体は
完了している可能性があるため、状態を開き直して確認する。異常終了や OS cleanup エラーで残った
`.kinko-*.tmp` には暗号文だけが入る。

排他の前提は同じ実装を使う協調 writer と、mkdir / hard link / rename を提供するローカル FS。
既存 symlink alias は同じ lock へ解決するが、hard-link 別名、network/sync FS、第三者による
親 directory・symlink・lock の差替え、旧 writer / 非協調 writer は保証外。
新規作成は hard link 非対応 FS では失敗し、危険な上書き方式へ fallback しない。
1個の `Vault` の同時 goroutine 利用は非対応で、各操作で別の `Open` を使う。
provider の path-based credential route は既存互換のため変えていない。

Windows では mode bit より ACL が優先される。既定保存先はユーザー設定ディレクトリだが、
`KINKO_VAULT` で共有ディレクトリを指定した場合の ACL までは `kinko` が構成しない。

## backupからの復元と新端末

`kinko export` はGoogle Drive等の同期・retention・cloud account認証を行わず、別のage暗号化
fileを作るだけである。新端末では同期済みbackupへ`KINKO_VAULT`を向け、master passwordで
`get`または`unlock setup --dry-run`を実行してから、その端末のproviderをsetupする。古い端末の
OS credentialが無くてもmaster passwordがあれば復旧できる。sourceに`vault_id`がある場合は
export先へ引き継ぐため、同じ論理vaultとして扱える。

password違いまたはage暗号文の破損は、ageの制約上 `ErrWrongPassword` として同じ扱いになる。
future formatは更新を要求し、provider credentialのvault ID不一致はstale credentialとして
停止する。これらを越える自動復旧、Google Drive API、cloud同期の監視はkinkoの責務ではない。

## 脅威モデル

### 減らせるリスク

- リポジトリや設定ディレクトリを `cat`, `grep`, diff したときの実値露出
- 1 回の不用意な取得で、保管庫の全値をまとめて表示する事故
- 保管庫ファイルだけを持ち出された場合の平文露出
- 値だけでなく、秘密の名前から利用サービスを推測されること
- 保存途中の失敗で既存保管庫を途中まで上書きすること
- `KINKO_PASSWORD` を `run` の子プロセスへそのまま継承すること
- 対応OSのcredentialを毎回通常password promptへ露出させること（実機acceptance済みの環境に限る）

### 保護しないもの

- 正しいパスワードを知る利用者、同じ権限で動く悪意あるコード
- 侵害された OS、管理者権限、キーロガー、デバッガ、メモリダンプ
- `get` の標準出力をログ、履歴、ファイル、AI の会話へ流す操作
- 子プロセスが個別 secret を表示、保存、外部送信すること
- 子孫プロセスへの環境変数継承
- swap、hibernation、crash dump に平文が残る可能性
- 弱いマスターパスワードに対するオフライン推測
- 破損、誤削除、パスワード忘れからの自動復旧
- lock を守らない writer、hard-link 別名、network/sync FS による同時変更

## 運用上の不変条件

- 保管庫、実値入り `.env`、パスワードを Git へ入れない
- `add` の値は原則として引数ではなく非表示入力で渡す
- `KINKO_PASSWORD` は非対話処理に限定し、手作業の常設環境変数にしない
- `run` で起動するコマンドを信頼境界として扱う
- `export` のバックアップを本体とは別の安全な場所へ置く
- `export` はsource vaultの`vault_id`を引き継ぎ、backupから復元したときのprovider照合を保つ
- パスワード変更は、別パスワードへの `export` と切替で行う
- 競合エラーを無視せず最新の vault を開き直し、旧 writer と混在させない

## 非目標

現在の `kinko` は、1Password、SOPS、Vault、OS keychain の代替を完成させるものではない。
次の機能は現在のスコープ外である。

- チームのユーザー管理と secret ごとの ACL
- ネットワークサービス、クラウド同期、Web UI
- 監査ログ、利用承認、secret の自動ローテーション
- Git で暗号文の構造差分をレビューする部分暗号化
- vaultのdata keyを分離するformat 2、hardware-backed key、Secure Enclave、TPM
- OS adapterの実機acceptanceが済んでいない環境での端末解除保証

必要な機能がこの範囲に入る場合は、既存の secret manager を選ぶ方が適切である。

## 検証対応表

| 保証したいこと | 主な検証 |
|---|---|
| 暗号化後に同じ値を復元できる | `Test保管庫は書いた内容をそのまま読み戻せる` |
| 誤パスワードを拒否する | `Test違うパスワードでは開けない` |
| 名前と値が暗号文へ平文で残らない | `Test保管庫のファイルに平文が残らない` |
| 既存保管庫を `Create` で上書きしない | `Test既存の保管庫を上書きしない`, `TestConcurrentInitProcessesPublishExactlyOnce` |
| stale snapshot を拒否する | `TestStaleSnapshotCannotLoseAnotherUpdate`, `TestStaleProcessCannotOverwriteAnotherProcess` |
| symlink alias / crash lock | `TestSymlinkAliasesShareLockAndSnapshot`, `TestCrashedWriterLockIsNeverStolen` |
| 置換失敗・temp cleanup | `TestReplacementFailurePreservesOriginalAndReleasesLock` |
| age EOF / JSON / version を厳格に読む | `TestOpenRejectsInvalidJSONVersionAndUnauthenticatedEOF` |
| 未知の任意 metadata を保持 | `TestUnknownMetadataSurvivesSaveAndExport` |
| 保存前の失敗で既存内容を壊さない | `Test保存に失敗しても既存の保管庫を壊さない` |
| format 1 legacy / vault ID互換 | `Test既存format1はVaultIDなしでも開けて次回保存で付与される` |
| provider credentialとvaultの取り違えを止める | `TestOpenVaultProviderMismatchStopsWithoutPasswordFallback` |
| providerの期限切れでsetup/status/disableを無期限待ちしない | `TestProviderLifecycleDeadlineStopsBlockingFake` |
| request開始後の認証失敗で二重promptしない | `TestOpenVaultProviderPromptFailuresStopWithoutPasswordFallback`, Windows consent result tests |
| Windowsのunsafe HWNDを採用しない | `TestSelectActiveApplicationWindowRejectsUnsafeCandidates`, `TestNoActiveApplicationWindowDoesNotInvokeConsentRequest` |
| source / backupのvault ID継承 | `TestExportPreservesVaultID` |
| 子へマスターパスワードを渡さない | `TestChildEnvironmentDoesNotInheritVaultPassword` |
| テンプレートで予約変数を再注入させない | `TestExpandRejectsReservedPasswordVariable` |

`TestDailyWorkflowInSubprocesses` は Go test executable から production command 関数を呼び、合成データの
`init -> add -> get -> list -> run -> export -> backup reopen -> rm`、子の input/output/exit と
case-insensitive password 非継承を検査する。`TestGetClosedStdoutFailsInSubprocess` は get 出力失敗、
`TestInvalidArgumentsDoNotConsumeInputOrCreateVault` は入力前の拒否を検査する。
この helper は main の dispatch/exit 写像を模倣するので、実 CLI executable の end-to-end 検収とは区別する。
各 SHA で何が実行できたか、skip と実機未確認は案件 report に記録し、test の存在だけで成功としない。
release 前には対象 OS の実 CLI と native user-presence を別々に再確認する必要がある。

FAT / exFAT など hard link 非対応 FS での新規作成・export は非対応であり、安全に失敗する。
未検証の FS を対応済みとは扱わない。
