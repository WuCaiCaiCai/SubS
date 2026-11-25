# TODO

按「先修 bug、再排功能」组织。每条都带定位，方便下次直接下手。

## 待修 bug

### B1 · 配置预览显示框（编排第 3 步）

定位：`web/src/pages/ProfileEditor.tsx:871-879`（`<CodeEditor readOnly height={420} …>`）、
组件 `web/src/components/CodeEditor.tsx:129-165`。

现象：预览框高度塌陷 / 内容被裁、着色层与文本错位。

怀疑的原因（按可能性排序）：

1. `CodeEditor` 内部两个子层（`<pre>` 和 `<textarea>`）都是 `absolute inset-0`，
   父容器 `.relative flex-1` 的高度由内容撑开，而绝对定位元素不参与撑高 → 父容器高度为 0（或等于外部固定高度）。
   非全屏时 `height` 只写在了绝对定位的 textarea 的 `style` 上，撑不到外层。
   修法：给外层滚动容器显式 `style={{ height: fullscreen ? undefined : height }}`，
   或让 `<pre>`/`<textarea>` 二选一做非绝对定位的尺寸基准。
2. `pre` 的 `overflow-hidden` + `wrap="off"`：横向滚动靠 `scrollLeft` 同步，若父容器宽度变化（超长 base64 行）
   会出现错位或末尾被截。可改 `overflow-auto` 并隐藏滚动条，或对超长行加软换行开关。
3. `leading-[1.7]` 与行号列 `py-3` 的基线：行号列与代码区若 padding 不一致会造成逐行累积偏移
   （行数越多越明显），验收时用 200 行模板对比首尾两行。
4. `readOnly` 时 `caret-accent` / `selection:` 仍生效，误以为可编辑；只读态应隐藏光标并换 `cursor-default`。

顺带：模板页 `web/src/pages/Templates.tsx:284-290` 复用同一组件但没传 `height`，两处表现不一致，一起验。

### B2 · 「一键导入」区域文字与缩放

定位：`web/src/components/SubscriptionOutput.tsx:179-199`。

现象：文字大小跳跃、窄屏挤压换行、与相邻区块视觉重量不一致。

具体问题：

1. 字号混用三套裸值：链接行 `text-[0.7812rem]`（≈14.1px）、Loon 提示行 `text-[0.7188rem]`（≈12.9px）、
   右侧 hint 用 `.meta`（12.4px）——同一列表里三种大小，读起来是"没对齐"而不是"有层级"。
   改为：主标签 `text-body`、hint `text-meta text-fg-subtle`。
2. hint 用 `ml-auto` 单行右对齐，来源名较长（如「Clash Verge Rev、FlClash」）时与标签抢宽度，
   窄屏被压成两行且图标错位。改为 `flex-wrap` + hint 独占次行（`sm:` 以上才并排）。
3. Loon 那行是 `<div>` 虚线框，和两条 `<a>` 实线框在圆角、内距、阴影上不一致，视觉上像"半成品"。
   统一为同一组件的 disabled 态。
4. 图标 15px 与文字 14px 的视觉重心偏上（lucide 线框基线），需要 `mt-px` 或用 `size={14}` 对齐。
5. 与上面「下载配置文件」卡片相比，这个区块没有 hover 反馈的差异说明（链接可点、按钮可点），
   两者 hover 边框色一致，容易误判。

## 待做功能

### F1 · 协议适配器重构 + 新增协议

- `internal/protocol/adapter.go`：`Adapter` 接口 + 注册表，把 hy2 / vless / tuic / anytls / mieru 各拆一个文件；
- `ToSingBox` / `ToLoon` 返回 `ErrUnsupported` 即跳过（已在 Mieru 上验证过这条路径）；
- 新增 `vmess` / `ss` / `trojan`（sing-box 与 Loon 都有原生支持，覆盖率高）；
- `GET /api/protocols` 驱动前端协议列表，去掉前端硬编码；
- 用真实机场订阅跑通「导入 → 三端渲染 → 校验」。

### F2 · 输出方式继续扩展

已有：订阅链接、二维码、三端单文件下载、整包 zip、`?target=` 固定格式地址、Clash/sing-box 一键深链。
可继续加：Shadowrocket 深链、`sub://` 通用 scheme、单节点导出（选中若干个节点单独生成一份配置）。

## 待补的安全项

- 只设了 `X-Content-Type-Options: nosniff`，缺 CSP / `Referrer-Policy` / HSTS；
- `server.go:337` 的 `secure := r.TLS != nil` 在 TLS 终止代理后失效，需认 `X-Forwarded-Proto`；
- 无 2FA，会话不与 IP 绑定。
