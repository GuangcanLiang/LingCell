# 开发日志

记录已完成的工作与实际验证，按日期及主题倒序排列。日期使用 Asia/Shanghai 时区；架构与日常约定见 [AGENT.md](../AGENT.md)。

## 2026-10-10: 开发/测试 Agent 分离与 Git 工作树流程

### 目标与改动

根据用户要求，建立后续 Agent 必须执行的独立测试、交接、提交及分支/worktree 规范。

- 保留原有历史，将默认分支 master 重命名为 main；本任务在 docs/agent-workflow 分支和独立 .worktrees/agent-workflow 开发。
- AGENT.md 作为规范源，要求正式验收派生无开发对话继承的测试 Agent，固定候选提交、独立 detached 工作树、最小交接、复现反馈和改动后重验；明确这是维护分工，产品仍保持单 Agent MVP。
- 规定有意义的代码、测试、配置和文档变动必须提交，使用短期任务分支和 no-ff 整合，禁止主分支直接开发；补充 WIP、基线变化、回滚、工作树清理及远程保护边界。
- 增加 docs/workflow.md 操作手册、双向交接模板、CONTRIBUTING.md 入口及本地提交 hooks；README、AGENTS.md 链接到强制流程，忽略 .worktrees。
- 实际开发交接见 [agent-workflow 报告](handoffs/agent-workflow.md)，基线为先前文档提交 948a7a0；候选在提交后交给独立测试 Agent。

### 验证状态

首个候选 5e1757a 已由干净上下文测试 Agent 在 detached 工作树验收。hooks 正反行为、链接和身份检查通过，但回滚说明导致 FAIL：限制提交的 hooks 不覆盖直接 revert 的所有自动提交途径，且反向改动需检查暂存区。

修订 AGENT.md 与操作手册，统一回滚的 --no-commit / 暂存差异审查 / revert: 提交流程，补充本地 hooks 的真实边界。修订候选将另派干净上下文测试 Agent，结果再追加。当前只改变维护文档、Git hooks 与忽略规则，没有修改 Go 产品代码。

### 未验证与后续事项

远程仓库、服务器分支保护、PR 状态检查尚未配置；本次只落地本地开发流程。本仓库已设置 core.hooksPath=.githooks，主工作树更新 hooks 文件后生效；实际整合与工作树清理在验收通过后执行，并记录结果。

## 2026-10-10: Go 规范检查与维护流程

### 目标与改动

检查现有 Go 应用的开发惯例，修正维护问题，并让后续开发有文档、统一验证命令与可审查的 Git 提交。

- 补充导出接口和类型注释，说明终端会话、项目读取句柄、Agent 的使用和并发约束；统一格式及导入分组。
- `internal/model` 用可解包的请求错误保留取消/超时身份，终端仍显示简洁且不含敏感详情的消息。
- `internal/config` 显式限制配置体积为 64 KiB，拒绝 `null` 等非对象以及被限制流截断的超大配置。
- `internal/project` 让根目录句柄关闭支持重复调用；`internal/shell` 检查非阻塞设置，初始化失败后清理进程与句柄。
- `internal/agent` 检查文本输出、工具参数解码和结果编码错误；无法显示模型提案时停止，不进入命令确认/执行。
- `internal/app` 检查历史文件初始化关闭错误。新增相关回归测试，覆盖配置边界、超时原因、重复关闭和输出失败。
- 扩展 `Makefile`，增加 `fmt`、`fmt-check`、`mod-check`、`test-race`、`vet`、`lint`；基础检查统一为 `make check`，终端检查为 `make terminal`。
- 新建 [AGENT.md](../AGENT.md) 维护指南与标准发现入口 [AGENTS.md](../AGENTS.md)、[Go 检查记录](go-review.md) 及本日志；README 添加入口和最新验证命令。
- Git 以 MVP 提交 `bb4df55` 为基线，代码维护与文档分别形成提交；增加 Python 缓存忽略项，不提交二进制或真实凭据。

### 验证

在 Linux/amd64、Go 1.27.2、Staticcheck 2026.2.1 下执行：

- `make fmt`：完成格式化。
- `make check`：通过格式与 `go mod tidy -diff` 检查、`go vet ./...` 和 `go test -race ./...`，7 个实现包测试通过。
- `make lint`：通过，无报告。
- `make terminal`：CLI 构建与本地模拟服务/真实 PTY 验收通过，包含状态、输入、信号、作业、窗口、Agent 确认/拒绝、诊断、取消、历史及恢复流程。
- `git diff --check`：通过；检查 5 份 Markdown 文档的相对链接与空白格式，没有发现问题。

以上应用检查针对代码改动执行；后续文档修改检查链接和差异格式。依赖未升级，最低 Go 1.26.0 工具链未单独验证。

### 未验证与后续事项

1. 用户选定供应商后，验证真实分类、工具调用和结果反馈；当前配置仅为模板，没有真实凭据或供应商请求。
2. 在用户选择的测试主机上联调成功的 SSH 交互；本地脚本仅检查 `ssh -V`。
3. 确定远程仓库及远程安装需求后调整 module/import 路径，并按需求配置 Linux CI。当前没有远程仓库推送、发布或部署。

## 2026-10-10: Linux Agent Shell MVP 基线

### 已实现

本地 Git 基线：`bb4df55 feat: implement Linux agent shell MVP`。

CLI 使用持久 Bash + PTY；Go 负责编辑、补全和路由，命令及完成事件使用独立管道。普通 shell 命令保留工作目录、环境、别名、函数和作业；自然语言任务使用兼容 Chat Completions 的有界工具循环，所有模型提案的命令逐条确认。

项目读取范围固定到启动时最近 Git 根，使用 Linux `openat2` 保证打开范围，遵循敏感路径排除与 `.gitignore`，限制读取及输出预算。失败命令的诊断结果作为工具数据传入，拒绝后不执行修复；输入历史与模型上下文分离。

### 主要文件职责

| 文件或目录 | 职责 |
| --- | --- |
| `cmd/lingcell/main.go` | CLI 配置、信号取消和终端恢复。 |
| `internal/app`、`internal/router` | 输入编辑与补全、路由、续行、失败接管及确认。 |
| `internal/shell` | Bash 生命周期、PTY、状态管道、捕获输出及前台任务。 |
| `internal/model`、`internal/agent` | HTTP 模型协议、工具参数、决策循环和执行确认。 |
| `internal/project`、`internal/config` | 项目读取边界/预算及配置默认值/合法性校验。 |
| `scripts/verify_terminal.py` | 临时 HOME、本地模拟模型与真实 CLI 交互验收。 |
| `README.md`、`config.example.json` | 构建运行说明与无凭据配置模板。 |

本日志建立时重新阅读了实现、测试、脚本及 Git 差异。维护修改后的统一测试结果见上方条目；真实供应商、远程 SSH 和远程发布仍属于未验证事项。
