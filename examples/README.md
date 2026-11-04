# 合成データだけで日常操作を試す

Go 1.25 は source の最低言語版です。日常利用には security patch を含む現在サポートされる安定版を推奨し、CI は Go 1.27.2 で検査します（2026-10-11 確認）。
既存 module の利用可能な環境で、repo root から実行する。
新規作成・export は hard link 対応のローカル FS が必要です。FAT / exFAT は非対応です。
実 vault、実 password、実 token は使わない。端末解除も使わない。

まず新しい一時 directory を選び、保管庫の場所を指定する。

POSIX shell:

```sh
demo=$(mktemp -d)
export KINKO_VAULT="$demo/vault.age"
unset KINKO_PASSWORD
```

PowerShell:

```powershell
$demo = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory $demo | Out-Null
$env:KINKO_VAULT = Join-Path $demo 'vault.age'
Remove-Item Env:KINKO_PASSWORD -ErrorAction SilentlyContinue
```

両環境で次を実行する。入力する password は8文字以上の使い捨て合成値にする。
`add` の値にも架空の文字列を入力する。値は引数、template、会話へ貼らない。

```sh
go run . --master-password init
go run . --master-password add example-service/token
go run . --master-password list
go run . --master-password get example-service/token
go run . --master-password run --env-file examples/development.env.tmpl -- go run ./examples/check-env
go run . --master-password export "$demo/backup.age"
```

`get` だけは合成値そのものを表示し、末尾改行を付けない。子 command は設定の存在だけを表示する。
export の2番目の password 入力を空にすれば source と同じ password を使う。既存 backup 名は拒否される。

`KINKO_VAULT` を同じ一時 directory の `backup.age` に切り替えて `get example-service/token` を実行すると、
再度開けることを確認できる。その後 `vault.age` に戻し、次で削除と名前一覧を確認する。

```sh
go run . --master-password rm example-service/token
go run . --master-password list
```

終了後は指定した一時 directory と環境変数だけを片付ける。実 vault の場所へこの手順を流用しない。
lock が残った場合は、全 writer が終了したことを確認してから正規化対象の空 `.lock` directory だけを除く。

non-TTY の値省略 `add` は「値1行、password 1行」。`init` は「password、確認」の2行、
`export` は「source password、destination password」の2行。`run` は password の1行を読んだ後の
stdin を子 command に渡す。実用では process list / shell history / pipeline のログにも注意する。

この例の実行成功は Windows Hello、macOS user-presence、Linux desktop credential の実機検証ではない。
