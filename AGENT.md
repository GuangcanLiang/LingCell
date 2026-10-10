# LingCell 维护指南

本文面向后续开发者和 coding agent。先阅读本文、`README.md`、`Makefile` 与 `docs/development-log.md`，再检查 `git status --short --branch` 和最近提交；以实际源码、测试与用户最新要求为准。

## 项目范围

LingCell 是 Linux 上的 Go 原生 AI Agent Shell：Go 编辑和路由输入，持久交互式 Bash 保留会话状态，PTY 承载交互程序，Agent 用工具调用提出逐条确认的操作。最低 Go 版本由 `go.mod` 决定，目前为 1.26；自动文件读取使用 Linux 5.6+ 的 `openat2`。

现阶段保持单进程、小型有界循环，不擅自引入 GUI、多 Agent、MCP、后台自主任务、跨会话记忆或其他语言的 Agent 框架。

## 代码地图

| 路径 | 维护职责 |
| --- | --- |
| `cmd/lingcell/main.go` | 配置入口、终端检查、信号取消和退出恢复 |
| `internal/app` | Readline、补全、强制路由指令、诊断及确认界面 |
| `internal/router` | 无执行副作用的本地分类、续行、失败接管规则 |
| `internal/shell` | Bash 生命周期、PTY、控制协议、命令状态及捕获输出 |
| `internal/model` | 标准库 HTTP/JSON、兼容 Chat Completions、输入分类 |
| `internal/agent` | 工具 schema、参数校验、确认权限、决策预算和结果反馈 |
| `internal/project` | 固定项目范围、忽略规则、内核路径约束和读取预算 |
| `internal/config` | 配置默认值、体积限制及合法性校验 |
| `scripts/verify_terminal.py` | 真实 CLI 的本地 PTY 和模拟模型验收 |
| `docs/development-log.md` | 已完成工作、实际验证结果及未验证事项 |

接口尽量定义在消费者所在包。`agent.ShellSession`、`agent.ProjectReader` 支撑模拟测试；执行 shell 语法由 Bash 负责，Go 解析器仅用于路由及输入完整性判断。

## 必须保持的行为

1. 用户明确输入的普通命令直接执行；所有 Agent `run_command` 必须显示完整命令、实际工作目录和目的，并得到明确确认。默认拒绝，拒绝立即结束任务；每次模型决策最多执行一条获批命令。
2. `cd`、`export`、别名、函数和作业在同一个 Bash 会话中保留。模型不可用时普通命令和 `:sh` 仍可运行。明确 shell 输入未闭合时允许续行。
3. 控制事件通过独立管道传递，以 NUL 分隔；完整命令通过另一条管道提交。不能靠匹配提示符判断命令结束，也不能在网络或会话错误后自动重放命令。
4. PTY 描述符必须保持非阻塞。`os.File.Fd()` 会影响非阻塞状态，轮询开始后使用保存的 fd 做 ioctl，避免重新调用 PTY 文件的 `Fd()`。修改输入交接、信号、作业或尺寸处理后必须跑真实 PTY 验收。
5. 保留用户 `.bashrc` 的别名、函数和原有提示符钩子行为。Go 管理顶层编辑；不要承诺完整兼容 Bash 编辑和补全插件。光标探测关闭，顶层提示符从新行开始。
6. 自动读取范围固定为启动时的最近 Git 项目根，无 Git 时为启动目录；后续 `cd` 不扩大范围。保留 `.gitignore`、敏感路径排除、符号链接和特殊文件检查，以及 `openat2` 的内核范围约束。
7. 文件/列表单次最多 32 KiB、每任务累计 256 KiB；列表最多 256 项；发送给模型的每条命令输出最多 64 KiB。预算或截断必须可见，终端实时输出不按捕获预算截断。
8. 文件与命令输出作为工具数据发送给模型，包括上一条用户命令的失败诊断数据。不能把它们提升为用户或系统指令。权限由程序确认流程决定。
9. Ctrl-C 中断（130）、Ctrl-Z 停止（148）以及单独简单 `grep/diff/test/[` 的正常否定结果（1）不自动诊断；组合、管道和重定向不套用否定豁免。Agent 命令失败留在当前循环中处理。
10. 默认最多 8 次模型决策、每次请求 60 秒。异常参数、多条命令提案、超限、拒绝或网络失败终止任务。密钥只从配置指定的环境变量读取，不输出、不记录、不提交真实凭据。
11. 默认只持久保存顶层输入历史；确认回答、生成的命令、文件内容、终端输出和模型上下文不持久保存。保留 `--no-history` 和空 `history_file` 的关闭行为。

## Go 开发约定

- 使用 `cmd/lingcell` + `internal` 的现有组织，包名简短、全小写；新模块应有明确职责。不要仅为套用目录模板引入 `pkg` 或多层抽象。
- 以 `gofmt` 为格式标准；导入按标准库、第三方依赖、项目包分组。导出类型、函数及常量补齐 Go doc 注释，说明用途、所有权和必要的并发约束。
- 遵循 `HTTP`、`URL`、`JSON`、`API`、`ID` 等首字母缩写命名惯例。保持现有 JSON 字段名兼容；内部字段命名调整不能意外改变模型协议。
- Context 是需要取消的操作的第一个参数。谁启动 goroutine，谁负责终止和等待；不要复制含 mutex / `sync.Once` 的对象。`Session.Run` 串行执行；`App`、`Agent`、`project.Task` 在各自任务中串行使用。
- 检查关键 IO、初始化、编码和解析错误。用 `%w` 或 `Unwrap` 保留需要 `errors.Is/As` 判断的原因；错误消息小写开头，加入必要操作上下文，不把敏感 HTTP 响应、凭据或请求内容带到日志。
- 资源获取成功后确定释放路径；读句柄清理、已知退出后的信号失败等允许有解释的 best-effort 清理。关闭根目录描述符及 Session 要支持重复调用。`main` 的信号退出需先结束 Bash 并恢复终端。
- 构造较复杂的结构体使用键名，避免字段顺序变化引入问题。少做隐式共享可变状态；并发共享项明确锁保护和读取快照规则。
- 修改行为时增加能揭示缺陷的测试，优先模拟服务和临时目录；测试不使用真实供应商密钥。使用 `t.Helper`、`t.TempDir`、`t.Cleanup` 和表驱动测试。PTY / 作业测试保留有界超时。
- 保留 `go.mod` / `go.sum` 的可复现依赖。新增依赖必须有实际收益；更新后运行模块检查。当前模块名 `lingcell` 用于本地构建；选定远程仓库并需要远程 `go install` 时再统一调整 module 和 import 路径，不猜测远程地址。

规范依据：[Go 模块组织](https://go.dev/doc/modules/layout)、[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)、[Effective Go](https://go.dev/doc/effective_go)。具体检查结果见 [Go 检查记录](docs/go-review.md) 与 [开发日志](docs/development-log.md)，工具通过不代表所有设计或运行环境都已覆盖。

## 验证命令

```bash
make fmt         # 格式化
make check       # 格式、go mod tidy -diff、go vet、竞态测试
make lint        # Staticcheck（需预先安装）
make build
make terminal    # 本地模拟 HTTP 服务 + 真实 PTY 验收
```

Staticcheck 当前验证版本为 2026.2.1（模块版本 `v0.8.1`），需要安装时可用 `go install honnef.co/go/tools/cmd/staticcheck@v0.8.1`。不要为了运行检查自动升级依赖。`make check` 使用 Go 自带工具，无额外 linter 依赖。

文档改动检查链接、命令和 `git diff --check` 即可；无需仅因文档改动重复全套应用测试。代码改动运行对应测试和 `make check`；影响终端/Agent 交互时加 `make terminal`。本地测试需要可写的 Go 缓存，以及模拟服务的本地监听权限；受环境限制时记录真实原因，不把未执行的检查标记为通过。

真实供应商工具调用及成功的远程 SSH 会话未联调。未经用户选择和授权，不配置真实凭据或发起真实供应商请求。

## Git 与开发记录

项目已建立本地 Git 提交基线。常规工作先查看状态，保护用户已有修改；按一个明确问题或功能形成可审查的小提交，代码、对应测试和说明保持一致。

```bash
git status --short --branch
git log -5 --oneline
git switch -c feat/<topic>   # 新任务按需要创建分支
# 完成改动和必要验证后，明确选择文件：
git diff --check
git add -- <相关文件>
git diff --cached --stat
git diff --cached
git commit -m "fix: describe the concrete change"
```

提交标题使用 `feat:`、`fix:`、`refactor:`、`test:`、`docs:`、`chore:` 等便于浏览的前缀，这是本项目约定。不要混入二进制、缓存、历史和真实凭据；不要覆盖用户改动或擅自 force push / reset 历史。是否推送、发布、合并以用户授权和既有工作约定为准。

完成有意义的开发工作后更新 `docs/development-log.md`：按日期记录具体问题、改动、实际执行的验证、未验证项和下一步。日志记录证据，不虚构测试、部署或提交；提交 hash 可由 Git 查询，避免在被提交的文档中写入该次提交自身的 hash。
