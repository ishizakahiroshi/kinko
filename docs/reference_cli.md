---
type: reference
status: stable
tags: [cli, reference]
last_reviewed: 2026-08-13
---

# kinko CLI reference

この文書は現在の CLI 契約を、コマンド、入出力、環境変数、env テンプレートの順に
まとめる。背景と安全性の境界は [Architecture reference](reference_architecture.md) を参照。

## コマンド一覧

### `kinko init`

新しい空の保管庫を作る。

```text
kinko init
```

- 8 文字以上のパスワードと確認入力を求める
- 保管庫が既に存在する場合は上書きせず失敗する
- 成功時は保管庫のパスと、復旧手段がない旨を標準出力へ出す

`KINKO_PASSWORD` が設定されている非対話処理では、その値をパスワードと確認入力の
両方に使う。

### `kinko add <名前> [値]`

秘密を追加する。同じ名前があれば更新する。別名は `set`。

```text
kinko add example-service/api-token
kinko add example-service/api-token <値>
```

- 値を省略すると、端末では画面に表示しない入力を使う
- 非 TTY の標準入力では 1 行を値として読む
- 値を引数で渡した場合は、process list と shell history に残る警告を標準エラーへ出す
- 名前の前後の空白は除去し、空の名前は拒否する
- 空文字の値は保存できる
- 成功メッセージは標準エラーへ出す

引数で渡した複数語は、半角 space で結合して 1 つの値にする。この形式は shell の quoting
によって元の空白表現が変わるため、対話入力を推奨する。

### `kinko get <名前>`

秘密を 1 件だけ標準出力へ出す。

```text
kinko get example-service/api-token
```

- 出力末尾に改行を付けない
- 値以外を標準出力へ混ぜない
- 名前が無い場合は失敗する
- パスワード prompt は TTY の標準エラーへ出す

標準出力は秘密そのものである。画面、ログ、command substitution、redirect の扱いは
呼び出し側の責任になる。

### `kinko list [接頭辞]`

秘密の名前を辞書順で一覧する。別名は `ls`。

```text
kinko list
kinko list example-service/
```

- 一致した名前を 1 行 1 件で標準出力へ出す
- 値は出さない
- 接頭辞を指定した場合は `strings.HasPrefix` の case-sensitive 一致で絞る
- 件数を標準エラーへ出す

### `kinko rm <名前>`

秘密を削除する。別名は `remove`, `delete`。

```text
kinko rm example-service/api-token
```

- 名前が無い場合は失敗し、保管庫を書き換えない
- 成功メッセージは標準エラーへ出す

### `kinko run --env-file <テンプレート> -- <コマンド> [引数...]`

テンプレートの kinko 参照を実値へ展開し、その環境を持つ子プロセスを起動する。

```text
kinko run --env-file .env.tmpl -- your-command --flag
kinko run --env-file=.env.tmpl -- your-command --flag
```

- `--env-file <path>` と `--env-file=<path>` を受け付ける
- command の option と区別するため `--` separator の使用を推奨する
- 標準入力、標準出力、標準エラーを子プロセスへ接続する
- 子が非 0 で終了した場合、その exit code で `kinko` も終了する
- command を起動できない場合は通常の `kinko` error として終了する
- テンプレートに kinko 参照が 1 件もなければ、保管庫を開かず password も求めない
- 参照が 1 件でも見つからない場合、子プロセスを起動せず失敗する

子環境は親環境を基にし、テンプレートで展開した項目を追加する。ただし
`KINKO_PASSWORD` は、大小文字の違いを含めて親環境から除外する。

### `kinko export <出力先>`

現在の全 secret を、新しい暗号化保管庫へ書き出す。

```text
kinko export backups/kinko-vault.age
```

- 出力先が存在する場合は上書きせず失敗する
- source vaultを通常の解除優先順位で開く（TTYではproviderを一度試す）
- 出力先のパスワードを求め、空なら元と同じパスワードを使う
- 別パスワードを指定する場合は 8 文字以上を要求する
- 成功時は件数と出力先を標準エラーへ出す

`KINKO_PASSWORD` を設定した場合、sourceと出力先のすべての password prompt が同じ環境変数値を返す。
そのため、非対話の `export` では別の出力先パスワードを指定できず、元と同じ値になる。

export 先は通常の vault と同じ形式である。`KINKO_VAULT` へそのパスを指定すれば開ける。
専用の import command は現在ない。

### `kinko unlock setup [--dry-run] [--yes] [--replace]`

master passwordでsource vaultを開き、この端末のOS providerへcredentialを登録する。

- `--dry-run`はvaultを検証してprovider、security level、verification binding、availability、
  configured、availableだけを表示し、vaultもOS credentialも変更しない
- 実保存はTTYでだけ実行でき、`--yes`を指定しない場合は確認を求める
- 既存credentialを更新するには`--replace`を指定する。旧credentialを先に削除する方式へは
  黙って切り替えない
- legacy format 1 vaultの実setupでは、provider credentialとの照合用`vault_id`を暗号化JSONへ
  追加保存する。provider保存失敗後もmaster passwordで開ける
- Windowsは`RtlGetVersion`で実ビルド番号を確認し、build 22000未満では`unsupported`として
  Hello interopとCredential Managerを呼ばない。対応buildでは`CheckAvailabilityAsync`でHello/PINの
  利用可否を確認してからWindows Hello確認後のCredential Managerを使う。`DeviceNotPresent`、
  `NotConfiguredForUser`、`DisabledByPolicy`、`DeviceBusy`、active application HWND不在では保存しない。
  macOSはKeychain user presence、Linuxは
  desktop Secret Serviceを使う。未対応OS、headless、provider UI表示前の利用不能は保存せず失敗する

### `kinko unlock status`

現在のprovider状態を標準出力へ表示する。vault path、account内部ID、credential値、master passwordは
表示しない。

```text
provider: <provider name>
security-level: <user-verified|session-protected|password-only>
verification-binding: <credential-bound|application-gated|session-protected|none>
availability: <available|device-not-present|not-configured|disabled-by-policy|device-busy|...>
configured: <true|false>
available: <true|false>
```

statusはvaultを開かず、vault fileやbackupを変更しない。

### `kinko unlock disable [--dry-run] [--yes]`

この端末のprovider credentialだけを削除する。

- `--dry-run`はstatusを表示するだけで削除しない
- 実削除はTTYの確認、または明示的な`--yes`が必要
- vault、backup、format、master passwordは変更しない
- credentialが未設定またはproviderが未対応なら安全なerrorを返し、passwordを表示しない

### `kinko --master-password <command> ...`

`add`、`get`、`list`、`rm`、`run`、`export`でOS providerを使わず、既存のpassword readerを
選ぶboolean option。password値をargumentへ置かない。空でない`KINKO_PASSWORD`は既存のautomation
契約としてreaderから優先される。

通常のTTY解除はproviderを1 invocationにつき最大1回試す。provider呼び出しには既定10秒のdeadlineを
適用する。未設定・未対応・UI表示前の利用不能だけはmaster passwordへfallbackし、OS認証request開始後
のcancel、retry exhaustion、timeout、DeviceBusy、async failure、credentialとvaultの不一致は同じ
invocationで追加promptせず停止する。OS側APIがcontext cancellationを受け取らない場合もCLIはdeadline
で戻るが、OS dialog自体が残る可能性があるため、明示的に閉じてから再実行する。

### Version と help

```text
kinko version
kinko --version
kinko -v

kinko help
kinko --help
kinko -h
```

source tree の既定 version は `dev`。release build では linker flag
`-X main.version=<version>` で埋め込む設計だが、release workflow はまだ提供していない。

## 環境変数

### `KINKO_VAULT`

保管庫ファイルのパスを上書きする。未設定時は、`os.UserConfigDir()` が返す
ユーザー設定ディレクトリ配下の `kinko/vault.age` を使う。

Windows の一般的な環境では `%AppData%\kinko\vault.age` になる。実際の値は OS と
ユーザー設定に依存する。

`KINKO_VAULT` は子プロセスから自動除外しない。パス自体を子へ見せたくない場合は、
起動前に環境を分ける必要がある。

### `KINKO_PASSWORD`

password prompt を非対話化する。

入力の優先順位は次のとおり。

1. 空でない `KINKO_PASSWORD`
2. TTY なら画面に表示しない端末入力
3. 非 TTY なら標準入力の 1 行

複数の password prompt を持つ command に、複数行を 1 本の pipe でまとめて渡すことは
automation の契約に含めない。非対話の `init` は `KINKO_PASSWORD` を使う。非対話の
`add` で値を標準入力から渡す場合も、password は `KINKO_PASSWORD` で分離する。

環境変数は子プロセス、診断情報、同じユーザーの別プロセスから読める場合がある。
CI や限定した自動処理以外では使用しない。

`kinko run` はこの変数を子環境から case-insensitive に除外する。env テンプレートで
同名を設定することも拒否する。

## env テンプレート

### 基本形

```dotenv
# 空行と、この形の comment は無視される
API_TOKEN=${kinko:example-service/api-token}
APP_MODE=development
COMBINED=prefix-${kinko:example-service/id}-suffix
```

各有効行は、最初の `=` で `KEY` と `VALUE` に分割する。

- 行全体の前後の空白を除去する
- 空行を無視する
- 空白除去後に `#` で始まる行を comment として無視する
- key の前後の空白を除去する
- 空の key を拒否する
- value 内の `${kinko:<名前>}` をすべて置換する
- 1 行に複数の参照を書ける
- secret の空文字と、存在しない secret は区別する
- 存在しない参照が 1 つでもあれば、空値で起動せず全体を失敗させる

### dotenv や shell との違い

テンプレートは完全な dotenv parser や shell parser ではない。

- `export KEY=value` 構文は扱わない
- quote を構文として解釈せず、`"` や `'` も value の文字として渡す
- inline comment を解釈しない
- variable expansion、command substitution、escape sequence を解釈しない
- multiline value を扱わない
- `${kinko:...}` を literal として escape する構文はない

テンプレートには 1 行 1 環境変数の単純な `KEY=VALUE` だけを書く。

### 予約名

`KINKO_PASSWORD` は大文字小文字を問わず予約されている。

```dotenv
# いずれも error
KINKO_PASSWORD=value
kinko_password=value
```

保管庫のマスターパスワードは、個別 secret を渡すテンプレートより強い資格情報である。
テンプレートからの再注入を許可しない。

## Exit code

| 状況 | Exit code |
|---|---|
| kinko command 成功 | `0` |
| 引数、保管庫、password、template などの error | `1` |
| `kinko run` の子プロセスが非 0 | 子プロセスと同じ code |

誤パスワードまたは破損した保管庫では、age が両者を区別できないため同じ error として
扱う。CLI は password の確認を促すが、破損の可能性も残る。

## Aliases

| 正式名 | Alias |
|---|---|
| `add` | `set` |
| `list` | `ls` |
| `rm` | `remove`, `delete` |
| `version` | `--version`, `-v` |
| `help` | `--help`, `-h` |

新しい automation は正式名を使うことを推奨する。
