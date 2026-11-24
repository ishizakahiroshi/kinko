# エージェントごとの保管庫を合成データで試す

複数の AI エージェントやジョブへ渡す秘密を分ける運用を、`KINKO_VAULT` と
別々のマスターパスワードだけで確認する手順。追加の機能は使わない。
実 vault、実 password、実 token は使わない。端末解除は使わない。

この分離がエージェント間の境界として機能するのは、エージェントごとに
**別の OS ユーザー**で動かす場合だけ。同じ OS ユーザーの別プロセスは
`/proc/<pid>/environ` などから互いの `KINKO_VAULT`・`KINKO_PASSWORD` を
読み取れるため、同一ユーザー内でのこの手順は機能確認であり、
エージェント間の隠蔽の証明ではない。

POSIX shell で repo root から実行する。新規作成は hard link 対応の
ローカル FS が必要。

まずエージェント2つ分の一時 directory を用意する。

```sh
demo=$(mktemp -d)
mkdir -p "$demo/agent-a" "$demo/agent-b"
unset KINKO_PASSWORD
```

agent-a 側の保管庫を作る。password は8文字以上の使い捨て合成値、
`add` の値にも架空の文字列を入力する。

```sh
export KINKO_VAULT="$demo/agent-a/vault.age"
go run . --master-password init
go run . --master-password add example-service/agent-a-token
```

agent-b 側は `KINKO_VAULT` を `"$demo/agent-b/vault.age"` へ切り替え、
別の合成 password で同じ手順を実行し、`example-service/agent-b-token` を入れる。

確認できること:

- 各 `list` は自分の保管庫の名前だけを返す
- agent-b の password で agent-a の保管庫は開けず、
  「パスワードが違うか、保管庫が壊れています」で失敗する
- 自分の保管庫に無い名前の `get` は
  「指定された名前の秘密がありません」で失敗し、値の有無も返さない
- `strings "$demo/agent-a/vault.age"` に名前・値の平文は出ない

`run` の境界は、他方の名前を参照する実値なし template で確認できる。

```dotenv
OTHER_TOKEN=${kinko:example-service/agent-b-token}
```

この template を agent-a 側で `run` すると「保管庫に無い参照があります」で
子 command を起動せず失敗する。同じ名前を agent-a の保管庫にも `add` すれば
自分の値で展開されて成功する。つまり保管庫の中身そのものが、
そのエージェントが template 経由で受け取れる秘密の上限になる。

終了後は一時 directory ごと削除する。`KINKO_VAULT` と `KINKO_PASSWORD` は
検証用 shell のみに設定し、実運用の保管庫パスへ流用しない。
non-TTY の入力契約は [README](README.md) の値省略 `add` の説明と同じ。
