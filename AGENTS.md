# AGENTS.md — AI 施工总入口

本仓库由 AI 会话自主开发，人类只负责部分 UI 手动测试与产品决策。任何新会话开工前**必须按顺序通读**：

1. `docs/设计说明.md` — 产品逻辑与边界（做什么、不做什么）
2. `docs/前端设计规范.md` — 视觉与动效约束
3. `docs/技术架构与选型.md` — 技术栈与目录结构（已锁定，不得更换）
4. `docs/数据模型与API规范.md` — 数据库与接口契约
5. `docs/协议解析与配置生成规范.md` — 各协议字段与输出映射
6. `docs/测试与验收清单.md` — 自动化与手动测试范围
7. `docs/部署与CLI命令规范.md` — 部署与命令行行为
8. `DECISIONS.md` — 已拍板决策日志（有冲突时以最新决策为准）

## 已锁定的技术决策（不可由 AI 会话单方面推翻）

- 后端：Go（单一静态二进制，前端资源 embed）
- 前端：Vite + React + TypeScript + Tailwind + shadcn/ui + lucide-react，Motion 用于动效
- 数据库：SQLite（单文件，WAL 模式）
- 目标客户端：Mihomo（主力）、sing-box、iOS **Loon**（已确认，见 DECISIONS.md）
- 部署：GitHub Release + 一条 `curl | bash` 安装命令 + systemd

## 代码规范

- Go：标准 `gofmt`/`golangci-lint`；错误必须处理，不吞错；敏感字段类型用专用 `Secret string` 包装，其 `String()` 输出遮蔽值
- TS：严格模式；组件用函数式；shadcn/ui 组件放 `src/components/ui/`
- 注释写"为什么"，不写"是什么"
- 禁止：任何形式的遥测/上报、把秘密写入日志、添加未在架构文档中声明的依赖

## 目录结构（约定）

```
/                     # Go 模块根（go.mod）
/cmd/subs/            # main 入口
/internal/            # server、store、airport、node、profile、render、backup、destroy
/web/                 # 前端（Vite 工程）
/web/dist 构建产物 embed 进二进制
/install.sh           # 安装脚本
/docs/*.md            # 上述设计文档
/README.md、TODO.md、DECISIONS.md、AGENTS.md  # 仓库根只保留导航与决策类文件
```

## 验证要求（每次改动后必须执行，全绿才能算完成）

```bash
cd web && npm run build          # 前端零报错
go build ./... && go vet ./...
go test ./...                    # 单测全过
```

涉及 UI 改动时，通知人类按 `docs/测试与验收清单.md` 的手动部分验证。

## 提交纪律

- 小步提交，一次提交一个逻辑单元
- commit message 用中文，格式：`类型: 描述`（feat / fix / docs / refactor / test / chore）
- 破坏性改动（改 schema、改 API 契约）必须同步更新对应文档并在同一次提交内完成迁移逻辑
