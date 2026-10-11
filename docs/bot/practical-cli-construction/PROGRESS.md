# #P-20261011-012 kinko：構築の入口

更新: 2026-10-11。対象/保存先: `ishizakahiroshi/kinko`（公開）。
branch: `dots/practical-cli-construction-P-20261011-012`。Draft PR base: `main`。
挙動基準: `9d463955b06e5b25bc5c80492eb1ded4f5761fe9`。
管理 Issue: https://github.com/ishizakahiroshi/kinko/issues/1 。状態の正本は管理 Issue コメント。

| 工程 | 担当 | 証跡と残るゲート |
|---|---|---|
| 指示公開・送信 | 手元 Codex | README/FACTS/REVIEW の固定 commit を Issue と送信で示す |
| 保存と CLI 構築 | dots 実装担当 | Issue の受付・提出 comment と code SHA をリンクする |
| 独立レビュー | dots 別担当 | Issue に review SHA/finding/再レビューを記録する |
| 手元検収・HTML | 手元 Codex | actual diff、Windows test/vet/scan、合成一連操作。実機 Hello は別ゲート |

指示 SHA、code SHA、review SHA、看板 commit は別々に管理する。看板自身の commit は履歴/後続 receipt で読む。実値と私有 locator を載せない。

## 2026-10-11 修正候補と独立レビューの提出

- Draft PR: https://github.com/ishizakahiroshi/kinko/pull/2 。remote head は `ffcf35c9271490636370e58224c4d4215cd4e6c5`（15:24 UTC 確認）。main `ad842a4f56427871ccb4386d1ff125a99ab9850a` の保存競合・scanner・portfolio 改善を統合済み。
- code commit object / 独立 review 対象: `660417bfef87c9d615fe0c935c9a74c93f22f875`。P2 3件、Darwin zeroing、CRLF contract、CI toolchain を修正。ref 更新は取り消され、再試行せず公開保存の次指示待ち。
- 独立担当は固定指示から 31 files、挙動基準から 35 files、main から 56 files の全差分を確認。P2 3件の修正と回帰を照合し、追加 blocking finding なし。source review であり新候補の動作検査成功ではない。
- ffcf35c の最初の Go CI run 38150357418: Ubuntu vet/test/race 成功。Windows test は source contract の CRLF 差、macOS vet は C 宣言、govulncheck は旧 Go の GO-2026-4971 で failure。660417bf の CI は未起動。
- Linux 実装環境は Go toolchain/gofmt/cache なし。660417bf source の diff check と secrets-scan は exit 0（58 files、private watchlist なし、4 structural patterns）。手元 Windows の成功報告は ffcf35c まで。新候補の test/vet/fmt と再検収は未実施。
- `results/report.html`、`REVIEW_RESULTS.md`、`results/recovery-files-660417bf.json` はローカル提出資料。文書自身は code SHA に含まれず、公開前に資料込みの最終 scan が必要。
- 実機 user-presence、hard-link 非対応 FS / network FS、crash lock 回収は別ゲート。構築完了・release-ready とはしていない。

### 手元からの後続検査報告

15:27 UTC 受付: Windows 660417bf の vet / 変更 Go format / scan（58 files）は成功。CLI / unlock test は成功、vault test は実行中。COM の read-only 確認は device-not-present、symlink alias は skip であり、成功に数えない。対話 Hello は未実施。この報告は独立担当自身の Go 実行ではない。文書のみの追加 commit object を660417bfの子として作る許可を受けたが、remote ref 更新は禁止のまま。最終 CI と HTML の判定は、手元担当の確定結果を照合して更新する。

文書6 files は同じ scanner の file 列挙のみをメモリ上で固定して read-only 検査し、exit 0（0 private needles + 4 structural patterns）。公開前の private watchlist 込み scan は手元担当で確認する。
