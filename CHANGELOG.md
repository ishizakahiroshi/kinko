# Changelog

この形式は [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) に従い、版は
[Semantic Versioning](https://semver.org/lang/ja/) で付けます。

## [Unreleased]

最初の版（0.1.0）に向けた内容です。公開時にこの節を `## [0.1.0] - <日付>` へ改めます。

### Added

- 保管庫の作成と、名前を指定した秘密の追加・取得・一覧・削除（`init` / `add` / `get` / `list` / `rm`）
- `${kinko:名前}` を含む env テンプレートを展開して子プロセスだけへ渡す `run`
- 別の暗号化済み保管庫へ書き出す `export`
- 端末解除（`unlock setup` / `unlock status` / `unlock disable`）と、OS の解除を使わない
  `--master-password`。Windows は Windows Hello と Credential Manager、Linux は desktop の
  Secret Service を使う。macOS の Keychain は C コンパイラ付きで作った場合だけ含まれる
- 保存時の競合検出（開いた後に他の書き込みがあれば上書きせず止める）と書き込み lock
- GitHub Releases への配布（Windows / macOS / Linux、amd64 と arm64）と `SHA256SUMS`
- 復旧の備え: `init` / `add` / `rm` のたびに変更前後の保管庫を自動で世代として残す（最新 32 件と 30 日以内、
  上限 512 MiB）。`kinko backup status / list / add / remove / push / verify / restore`。登録した PC 外の保存先へは
  変更のたびに追記だけで写し（最大 2 秒待ち）、削除・上書きはしない。復元は新しいファイルへ書き、今の保管庫は変えない
- 環境変数 `KINKO_GENERATIONS_DIR`（手元の世代の置き場）

### 確認の状態

- マスターパスワード方式は Windows / macOS / Linux の CI で test 済み
- 端末解除の実機確認: Windows は確認待ち、macOS と Linux は未検証
