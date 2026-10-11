---
cover:
  path: portfolio/overview-2026-09-28.jpg
  alt: {ja: "kinko の紹介動画", en: "kinko overview video"}
video:
  provider: youtube
  id: "ZGPQsUVrGfo"
  durationSeconds: 20
schemaVersion: 1
color: "#3f7766"
initials: "ki"
cat: {ja: "秘密管理 CLI", en: "Secret management CLI"}
tagline: {ja: "秘密は暗号化して、必要なひとつだけ。", en: "Encrypted secrets, retrieved one at a time."}
short: {ja: "秘密を age で暗号化してローカルに保管する Go CLI。名前を指定して取り出し、参照テンプレートから子プロセスの環境変数へ渡せます。", en: "A Go CLI that stores secrets locally with age encryption, retrieves them by name, and resolves reference templates into child-process environment variables."}
tech: ["Go", "age", "CLI"]
store: null
live: null
guide: null
featured: false
---
## ja

平文の設定ファイルを丸ごと読む機会を減らすための、ローカル専用の秘密管理 CLI です。保管庫の名前と値を age で暗号化し、必要な項目を名前で取り出します。run は参照だけを書いたテンプレートを読み、実値を平文ファイルへ保存せずに子プロセスへ渡します。侵害済みの OS や悪意ある子プロセスから秘密を守る仕組みではありません。紹介素材には実在の秘密を使いません。

## en

A local secret-management CLI that reduces the need to read entire plaintext configuration files. Names and values are encrypted with age, and individual entries are retrieved by name. The run command resolves references into child-process environment variables without writing the resolved values to a plaintext file. It does not protect against a compromised OS or a malicious child process. Promotional material uses no real secrets.
