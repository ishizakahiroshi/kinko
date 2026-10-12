---
type: reference
status: stable
tags: [style, html]
owner: ishizakahiroshi
related: [README.md]
last_reviewed: 2026-10-12
---

# HTML下地CSS

これは説明HTMLにインラインで埋め込む下地。外部フォントを取得せず、未導入フォントはsystem-uiへfallbackする。追加CSSでレスポンシブな比較図を作る。

```html
<style>
  :root {
    --bg:#f7f3ea; --ink:#241f1a; --ink-soft:#837a6c; --line:rgba(36,31,26,0.12);
    --accent:#ff7a3d; --card:#fffdf8; --ok:#2e7d4f; --warn:#b8860b; --bad:#b3261e;
    --display:'Baloo 2','Zen Maru Gothic',system-ui,cursive;
    --body:'Zen Maru Gothic',system-ui,sans-serif;
  }
  *{box-sizing:border-box}
  html,body{margin:0;padding:0}
  body{font-family:var(--body);color:var(--ink);background:var(--bg);line-height:1.7;
       text-wrap:pretty;font-size:16px}
  .wrap{max-width:min(1080px,92vw);margin:0 auto;padding:48px 0 80px}
  h1{font-family:var(--display);font-weight:800;font-size:2rem;margin:0 0 12px;letter-spacing:-0.01em}
  h2{font-family:var(--display);font-weight:700;font-size:1.4rem;margin:48px 0 16px;
     padding-bottom:8px;border-bottom:2px solid var(--ink)}
  h3{font-family:var(--display);font-weight:700;font-size:1.1rem;margin:28px 0 10px}
  p,li{font-size:1rem}
  a{color:var(--accent);text-decoration:underline;text-decoration-thickness:1.5px;text-underline-offset:3px}
  .card{background:var(--card);border:2px solid var(--ink);border-radius:18px;
        padding:22px 24px;box-shadow:4px 5px 0 var(--line);margin:16px 0}
  table{width:100%;border-collapse:collapse;margin:12px 0;background:#fff;
        border:2px solid var(--ink);border-radius:12px;overflow:hidden}
  th,td{padding:10px 14px;text-align:left;border-bottom:1px solid var(--line);font-size:0.94rem}
  th{background:#fffdf8;font-family:var(--display);font-weight:700}
  tr:last-child td{border-bottom:0}
  .tag{display:inline-block;padding:2px 10px;border-radius:999px;
       font-family:var(--display);font-weight:700;font-size:0.78rem;
       border:2px solid var(--ink);background:#fff;color:var(--ink)}
  .tag-ok{background:var(--ok);color:#fff} .tag-warn{background:var(--warn);color:#fff}
  .tag-bad{background:var(--bad);color:#fff} .tag-accent{background:var(--accent);color:#fff}
  .system-badge{display:inline-flex;align-items:center;gap:8px;padding:6px 14px;
                border-radius:999px;border:2px solid var(--ink);background:var(--card);
                font-family:var(--display);font-weight:700;font-size:0.86rem}
  .system-badge .dot{width:8px;height:8px;border-radius:50%;background:var(--accent)}
  .meta{color:var(--ink-soft);font-size:0.88rem}
  @media print{ body{background:#fff} .card{box-shadow:none} }
</style>
```