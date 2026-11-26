# TODO

按「先修 bug、再排功能」组织。每条都带定位，方便下次直接下手。

## 已修复 UI 问题

### B1 · 配置预览显示框（编排第 3 步）

已完成。外框显式承载高度，行号、着色层和输入层统一字号及行高，内部 code 继承字号，着色层同步输入区宽度，只读态允许 Tab 移出且隐藏光标。已用 200 行及 2000 字符长行验证双向滚动、全屏和 Escape，320px、390px、1280px 深浅主题均无页面横向溢出。以下保留原问题记录。

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

已完成。导入条目统一主标签与说明字号、图标和边框，窄屏操作移至下一行，Loon 提供复制订阅地址操作，下载区域同步采用语义字号。已验证 320px、390px、1280px 深浅主题布局。客户端实际唤起和设备端导入仍需手动验收。以下保留原问题记录。

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

## 已修复重大问题（订阅不可用）

- 根因：`renderMihomo` 只在 `# subs:nodes` 标记处注入节点名，而 WebUI 新建模板的默认内容既无标记也无 `include-all: true`，注入只有 proxies 定义、代理组里仅剩 DIRECT，`MATCH,PROXY` 规则全部直连——「导入成功但不可用」。
- 修复：`internal/render/mihomo.go` 新增回退逻辑——当没有任何组通过标记拿到节点名时，对 `proxy-groups` 下 `proxies` 列表存在且不含 `include-all: true` 的组自动追加节点名（保留 DIRECT 等原有成员）；带标记与 `include-all` 的模板行为不变。
- 覆盖测试：`TestRenderMihomoDefaultTemplateGroupsGetNodes`、`TestRenderMihomoAutoFixSkipsIncludeAllAndMarkers`。

## 已移除：Caddy 集成

- 删除 `internal/server/caddy.go` / `caddy_test.go` / `configure-caddy.sh` 及相关路由、备份快照、schema 列（旧库遗留列不读）。
- 替代：实例新增 `https_only` 开关（「设置 → 仅支持 HTTPS 访问」），开启后非加密请求返回 426；反代与证书由用户自行部署，实例只信直连 TLS 或回环反代的 `X-Forwarded-Proto: https`。旧版备份里的 `caddy/Caddyfile` 恢复时忽略。

## 已新增：订阅名（slug）

- `profile.slug`：小写字母/数字/连字符，最长 32，唯一（部分索引）；空 = 旧式 `/s/<token>` 链接。
- 路由同时支持 `/s/<slug>/<token>` 与 `/s/<token>`；下载文件名、bundle.zip、客户端导入名优先用 slug。
- 创建与编辑表单均提供输入与校验；冲突返回 409。

## 备注：mieru 分享链接

`mierus://` scheme 变体与 query 里带 `port=` 的形式解析已支持；IPv6 主机、TCP transport 默认值均已覆盖测试。

## 已修复：订阅渲染与真实配置的字段对齐（v0.2.3）

- 以用户提供的真实 Mihomo 订阅文件为基准（脱敏副本 `internal/render/testdata/reference.yaml`），新增逐字段对比测试 `internal/render/reference_test.go`：解析基准内节点 → 整份文件作模板渲染 → 与基准逐节点逐字段断言，并校验 proxy-groups / rules / 顶层键完整性。
- 修复解析端（`internal/protocol/clash.go`）：vless/vmess SNI 读 `servername`（mihomo 字段名）并回退 `sni`；全协议读 `udp`，QUIC 系（hy2/tuic）缺省时默认开；tuic 读 `version`（默认 5）；hy2 读 `alpn`（默认 h3）。
- 修复渲染端（`internal/render/mihomo.go`）：全协议输出 `udp`；tuic 输出 `tls: true` 与 `version`；hy2 输出 `alpn: [h3]`。
- 层级组保护：自动补名跳过成员里引用了其他组的层级组（如 `[Stable, DIRECT]`），避免污染用户构造的选择层级。
- 已知豁免：hysteria2 的 `auth` 是 sing-box 系别名，mihomo 用 `password`，不补。
- 发布流程：release.yml 不再打包已删除的 configure-caddy.sh，校验和只含 tar.gz 与 install.sh。

## v0.2.3 发布清单

- 变更：订阅渲染修真（字段对齐 + 层级组保护）、移除 Caddy 集成、HTTPS-only 开关、订阅名 slug、mieru 链接变体。
- 流程：提交 → 打 tag `v0.2.3` → CI 构建发布 → 服务器 `curl | bash` 重装。
- 安装命令不变：`curl -fsSL https://github.com/WuCaiCaiCai/SubS/releases/latest/download/install.sh | sudo bash`；想装指定版本加 `SUBS_VERSION=v0.2.3`。
- 重装为同数据目录覆盖安装时数据保留；彻底重来可 `SUBS_DATA_DIR` 指新目录或先清 `/var/lib/subs`。
