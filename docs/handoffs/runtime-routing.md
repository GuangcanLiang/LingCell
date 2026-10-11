# 运行上下文与输入路由：交接

- 需求：每任务提供有限结构化运行上下文；未知首词的明确自然语言请求含 shell 符号仍进入 Agent，保留命令/脚本/赋值/复合结构和强制指令。
- 职责：开发 Agent 实现和修复；独立测试 Agent 审查及验收，不改代码、提交、合并、配置或真实服务。
- 根目录：`/home/liang_guang_can/LingCell`。
- 开发：`.worktrees/runtime-routing`，`feat/runtime-routing`。
- 基线：main `4876fba`（完整 SHA 与固定候选由派生消息提供）。候选不在自身提交中预写。
- 测试：固定候选 SHA 的 detached `.worktrees/verify-runtime-routing-<sha>`。
- 范围：internal/agent 运行上下文、internal/router 本地优先级、回归矩阵和 README/AGENT.md 契约。
- 验收：核对 Linux 内核、架构、root/non_root、Bash cwd、固定 root、全部工具；环境哨兵不上传；已知命令直通；未知自然语言前缀的 `$`、分号、换行和未闭合输入进入 Agent；脚本/赋值/复合语法兼容，模糊混合输入走分类、命令仍确认；强制指令兼容。
- 命令：make check、make lint、make build、make terminal、git diff --check；临时可写缓存及本地 HTTP 监听权限。
- 未验证：真实模型、远程 SSH、个人 bashrc、人工日常试用；不配置凭据或发出供应商请求。
- 独立报告：PASS，完整记录见下。

# 运行上下文与输入路由：独立验收报告

- 状态：**PASS**
- 测试 Agent：`/root/accept_runtime_routing`，干净上下文与固定 detached worktree。
- 基线完整 SHA：`4876fba7caec990905434f7c2ae53ea8a7bbad9f`
- 候选完整 SHA：`03ea0b500bb5e48f8a138a98e002fda64f25a276`
- 实际 HEAD（开始/结束）：均为候选完整 SHA。
- 工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-runtime-routing-03ea0b5`。下述命令均在该目录执行。
- 工作树状态（开始）：`## HEAD (no branch)`，无文件修改。
- 工作树状态（结束）：`## HEAD (no branch)`，无文件修改；仅忽略生成物 `bin/`、`scripts/__pycache__/`（额外 Python 验收导入既有 fixture 产生）。未改产品源码、提交、refs、配置或 hooks。
- 环境：Linux `6.6.87.2-microsoft-standard-WSL2` / x86_64；Go `go1.27.2 linux/amd64`；Staticcheck `2026.2.1 (0.8.1)`；有效 UID 1000，对应 `non_root`。不读取真实凭据或调用真实服务。

以下 make 命令的完整前缀为：

```text
env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/lingcell-runtime-routing-go-cache STATICCHECK_CACHE=/tmp/lingcell-runtime-routing-staticcheck-cache
```

| 验收条件/检查 | 命令或步骤（make 行使用上述完整前缀） | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 独立审查 | 读取 AGENTS.md、AGENT.md、README.md、Makefile、开发日志、交接与报告模板；`git diff 4876fba7caec990905434f7c2ae53ea8a7bbad9f 03ea0b500bb5e48f8a138a98e002fda64f25a276 -- internal/agent internal/router` | 差异为 9 文件，新增运行上下文、明确请求优先级、回归矩阵及说明。上下文从 uname/GOARCH/Geteuid/cwd/root/Tools 生成，未读取环境、账户详情、进程或配置内容。已知命令先于请求前缀，路径/赋值保留 shell 优先级。 | PASS |
| 格式/模块/vet/竞态 | `make check` | 首次沙箱内退出 2：模型 httptest 本地监听 `operation not permitted`；按 require_escalated 重跑退出 0。fmt、tidy -diff、vet、全部 7 内部包竞态测试通过。 | PASS |
| 实际静态分析 | `make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/lingcell-runtime-routing-staticcheck-measurements.txt'` | 退出 0，无诊断；独立临时缓存，测量文件 5,456 行，包含全部 8 实际包及测试变体。不是零包分析。 | PASS |
| 构建 | `make build` | 退出 0，生成忽略的 bin/lingcell。 | PASS |
| 终端完整流程 | `make terminal` | 首次沙箱内退出 2：Python 本地 socket `Operation not permitted`；按 require_escalated 重跑退出 0。三独立 CLI 会话、状态/续行/作业/尺寸/Agent 工具/确认拒绝/错误/取消/历史/终端恢复通过；vim、top、ssh -V 通过。 | PASS |
| 上下文/新增路由/强制指令/分类确认 | `python3 -u /tmp/lingcell-runtime-routing-extra.py`（require_escalated，本地 HTTP + 真实 CLI PTY） | 最终退出 0，见下方覆盖证据；全部独立断言通过。 | PASS |
| 文档本地链接 | 同一额外脚本检查 AGENT.md、AGENTS.md、README.md、docs/development-log.md、docs/handoffs/runtime-routing.md 的相对 Markdown 链接 | 5 文档 / 24 链接存在，退出 0。外部链接未联网验证。 | PASS |
| 差异空白 | `git diff --check 4876fba7caec990905434f7c2ae53ea8a7bbad9f 03ea0b500bb5e48f8a138a98e002fda64f25a276` | 退出 0，无输出。 | PASS |
| 结束身份 | `git rev-parse HEAD`；`git status --short --branch`；`git status --short --ignored` | 均退出 0；HEAD 未变、源码干净、仅上述忽略生成物。 | PASS |

额外 CLI 验收覆盖：

- 已知 echo、别名 personal、函数 help、`./please-script $PATH`、赋值及 if 结构均直接执行，期间模型请求计数不增加。已知 help 保留优先级，即使输入匹配 `help me` 请求前缀。
- `帮我解释 \`$PATH\``、`请解释 a; touch REQUEST_MUST_NOT_RUN`、`please explain echo $(`、`为什么 echo '`、带管道与换行的 `can you explain a | b\nand c` 都只发起带 tools 的 Agent 请求，原文保留，无分类、无续行要求、无 marker 文件副作用；多行使用真实 bracketed paste。
- 对每个 Agent 首次请求检查仅有 runtime_context/request，运行上下文字段恰为 os/kernel/architecture/privilege/cwd/project_root/tools。值分别比对实际 uname、架构、euid、Bash cwd；工具名与请求 schema 一致，包含全部三工具。
- `:sh printf FORCED_SHELL_OK` 直接运行；`:ai echo SHOULD_NOT_EXECUTE` 直接进入 Agent；随后 cd 到启动项目根外目录，再发任务，cwd 更新而 project_root 保持原启动根。synthetic 环境变量名/值与 `.env` 哨兵均不出现在上传 JSON。
- `unknown $PATH` 发起不含工具的分类请求；分类为 shell 后展示确认。空回答默认拒绝，任务结束且不发诊断请求；明确 y 批准后运行原始 `unknown $PATH`，exit 127 及原命令作为 tool 数据进入诊断，没有自动重放。
- CLI :exit 返回 0，终端属性恢复。

## 缺陷与复现

本次已执行检查未发现产品缺陷，无阻止合入的验收阻塞。

测试环境/fixture 中间失败记录：首次 check、terminal、额外脚本在沙箱内被本地 socket 权限阻止，均按权限流程升级后完成。额外脚本首次模拟回答遗漏 finish_reason、下一次误用拒绝文本 `execution declined`，两次退出 1；这是验收 fixture 的协议/断言错误，已在 /tmp 修正并完整重跑退出 0，不改候选。Staticcheck 测量文件首次误按 JSON 读取退出 1，随后按实际 TSV 格式解析，确认 5,456 条和全部实际包。

## 未运行项与限制

- 真实供应商/真实凭据、成功远程 SSH、个人 bashrc、人工日常试用未获本任务授权，未运行。make terminal 明确 SKIP remote SSH 和 personal bashrc。ssh -V 不作为成功远程会话证据。
- 最低 Go 1.26 未单独重跑，本次固定 Go 1.27.2，未升级依赖。
- 当前实际权限 non_root；未以真实 root 身份启动。root/non_root 分支经源码审查，non_root 输出经真实 CLI 与 euid 精确比对。
- 文档外部 URL 未联网验证。本次验证本地链接及所列命令。

## 返回开发者的动作

无需修复。协调者核对 main 基线、候选、工作树及 AGENT.md 合入条件；仅可追加验收证据文档。报告不授予额外服务调用或发布权限。
