# 交互与离线诊断交接

- 任务：第一批静态阶段、确认展示失败停止、七类模型错误和离线 `--doctor`。
- 职责：开发 Agent 实现/提交；独立测试 Agent 审查并运行正式验收，不改产品代码。
- 项目根：`/home/liang_guang_can/LingCell`。
- 开发：`.worktrees/interaction` / `feat/interaction-doctor`。
- 基线：`ec411712d610ec3de24420649b3826fbf8a49b02`（main）。
- 候选 SHA 与 detached 测试目录：提交后在测试 Agent 最小交接中指定，正式报告填写完整值。
- 范围：cmd/lingcell、internal/app/agent/model 及其回归、PTY 脚本、README。
- 风险：提示的 IO 失败、取消交接、诊断输出泄露私密配置、错误恢复。

## 验收条件

1. 命令/目录/目的展示的错误与短写必须在确认输入前返回，绝不执行。
2. 阶段覆盖分类、等待模型（轮次/Ctrl-C）、读取、确认及执行；回调失败停止对应操作，PTY 保留原始输出。
3. 配置/认证/限流/服务/超时/取消/协议错误可判别、提示含下一步，无正文或密钥泄露、无自动重试；错误后普通命令可运行。
4. `--doctor` 无终端、无联网/写入，检查依赖、配置、模型、变量及项目读取；无密钥/配置值/文件内容输出；缺模型 FAIL，缺凭据 WARN。
5. `make check`、有效的 `make lint`、`make build`、`make terminal` 通过；文档链接和 `git diff --check` 通过。

## 测试约束

读取 AGENT.md、README、Makefile 和差异；创建候选完整 SHA 的 detached `.worktrees/verify-interaction-<short-sha>`。开始/结束检查 HEAD、状态。允许本地模拟 HTTP/PTY，遇沙箱限制按权限流程运行；Staticcheck 必须实际分析包，不能将零包视为 PASS。不可真实服务请求、读取密钥、修改产品/refs/hooks/config、提交或合并。

真实供应商、远程 SSH、Go 1.26 和个人 bashrc 留给后续批次，不要求第一批执行。报告 PASS/FAIL/BLOCKED，记录精确 SHA、环境、命令/退出码、复现及未运行项；报告写到 `/tmp/lingcell-interaction-report.md`，由开发者追加证据。

## 独立验收报告

# 交互与离线诊断独立验收报告

- 状态：**PASS**
- 测试 Agent：`/root/interaction_acceptance`，干净上下文、最小交接，独立于开发 Agent。
- 基线完整 SHA：`ec411712d610ec3de24420649b3826fbf8a49b02`
- 候选完整 SHA：`8fb386f2f893731c7bcc91a57787007bf852100e`
- 测试目录：`/home/liang_guang_can/LingCell/.worktrees/verify-interaction-8fb386f`，以完整候选 SHA 创建 detached worktree。
- 实际 HEAD（开始/结束）：均为 `8fb386f2f893731c7bcc91a57787007bf852100e`。
- 工作树状态：开始 `## HEAD (no branch)`，干净；结束同样干净，`git status --short --branch --ignored` 仅多出忽略的 `!! bin/` 构建产物。`git diff --exit-code` 退出 0。
- 环境：Linux/amd64，WSL2 内核 `6.6.87.2-microsoft-standard-WSL2`；Go `go1.27.2`；Staticcheck `2026.2.1 (0.8.1)`；Python `3.13.5`；Bash `5.2.21(1)-release`；GNU stty/coreutils `9.4`。结束证据时间：`2026-10-11T02:38:21Z`。
- 已读取：AGENT.md、README.md、Makefile、开发日志、交接文档、测试报告模板；独立审查基线到候选全部产品、回归和 PTY 脚本差异。

下列命令除 worktree 创建外均在上述测试目录执行。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 候选隔离和身份 | `git worktree add --detach /home/liang_guang_can/LingCell/.worktrees/verify-interaction-8fb386f 8fb386f2f893731c7bcc91a57787007bf852100e`；开始/结束 `git rev-parse HEAD`、`git status --short --branch` | 全部 0；完整 SHA 一致；无产品改动 | PASS |
| 必需基础检查 | `make check` | 0；gofmt、`go mod tidy -diff`、`go vet ./...`、`go test -race ./...` 全部完成，7 个内部包竞态测试通过，cmd 包无测试 | PASS |
| 有效 Staticcheck | `go list ./...`；`STATICCHECK_CACHE=/tmp/lingcell-interaction-staticcheck-fresh make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/lingcell-interaction-staticcheck-fresh-measurements.txt'`，通过工具权限流程重跑 | 0；包列表为 8 个实际包；5,456 条 analyzer 记录，8 个实际包各 160 条，另有测试变体和依赖记录；无诊断 | PASS |
| CLI 构建 | `make build` | 0；构建 `bin/lingcell` | PASS |
| 真实本地 PTY | `make terminal` | 0；脚本输出 PASS，真实 CLI/PTY、本地模拟 HTTP；状态持久、多行、信号/作业、resize、vim/top、审批/拒绝、诊断、分类、取消、历史和终端恢复通过 | PASS |
| 展示失败先于确认输入/执行 | `go test -v ./internal/app ./internal/agent ./internal/model`；审查 app.confirm、agent.execute | 0；`TestConfirmationDisplayFailure` 的错误/短写和 4 个部分展示边界均通过；未初始化 confirmation 的情况下返回，无输入读取；Agent 输出/确认失败测试无命令执行 | PASS |
| 静态阶段与失败停止 | 同上；审查 app.Run、app.execute、app.progress、agent.Run；PTY 脚本阶段检查 | 0；等待模型包含轮次和 Ctrl-C；读取/确认/执行通知先于对应操作；`TestProgressFailureStopsOperations` 覆盖各 Agent 阶段失败与计数；分类阶段先于模型调用；raw PTY 路径未修改，真实交互程序通过 | PASS |
| 七类错误、脱敏、下一步、无重试、shell 恢复 | 同上及 `make terminal`；审查 model/errors.go 和 client.go | 0；配置/认证/限流/服务/超时/取消/协议类型可判别；401/403/429/500/503/408/504/404、非法 JSON/finish reason 和取消/超时身份回归通过；错误消息固定且含下一步；PTY 对 401/429/503/504/协议失败检查一次请求、无私密正文、错误后普通命令成功，另覆盖缺配置和等待取消后的 shell 可用 | PASS |
| doctor 非终端、脱敏、无文件修改、无 endpoint 请求 | `python3 /tmp/lingcell-interaction-doctor-acceptance.py bin/lingcell`（本地监听受限后按工具权限流程重跑）；审查 main doctor 分支、Doctor 及依赖调用链 | 0；stdin 为 PIPE、空输入，无 TTY；七种场景通过，stdout 只含允许的固定标签、stderr 空；故意设置私密 endpoint/model/env 名/凭据/history/file 名与内容均未出现；目录树路径、权限、大小、mtime、内容哈希前后相同；模拟 endpoint 请求计数始终 0；`.bashrc` 未执行，history/cache 未创建；doctor 在终端检查和 shell 初始化前返回，调用链无 HTTP 客户端或写文件操作 | PASS |
| doctor 正确状态码 | 同上 | 完整配置但缺凭据：WARN/退出 0；具备凭据：PASS/0；空凭据变量：SKIP/0；缺配置：FAIL model/1；缺模型：FAIL model/1；未知配置字段：FAIL configuration/1；空 PATH：Bash/stty FAIL/1 | PASS |
| 文档与空白 | Python 校验 3 份变更 Markdown 中 16 个本地链接；`git diff --check ec411712d610ec3de24420649b3826fbf8a49b02 8fb386f2f893731c7bcc91a57787007bf852100e` | 两项退出 0；目标存在、差异无空白问题 | PASS |

## 缺陷与复现

本次已执行检查未发现产品缺陷，无未解决的验收阻塞。

环境与验收工具的异常均已处理，不作为产品 FAIL：

- 初始 `make lint` 和使用 analyzer 参数的缓存运行退出 0；fresh cache 的沙箱运行仍退出 0，但提示 `warning: "./..." matched no packages`，该结果没有作为通过证据。按权限流程重跑同一 fresh-cache 命令后产生真实分析记录，并逐个检查 `go list` 的 8 个实际包存在 analyzer 记录。
- 初次独立 doctor 脚本创建 localhost socket 时 `PermissionError: [Errno 1] Operation not permitted`，退出 1。按权限流程重跑同一脚本，七种场景全部通过，脚本退出 0。
- 分析记录统计脚本首版误把测试变体计入“应恰好 8 包”的断言，退出 1；调整为逐个确认 `go list` 的 8 个实际包均有正数记录后退出 0。这是验收脚本断言错误，未改产品。
- 一次辅助源码读取尝试 `internal/project/reader.go`（不存在）退出 1；随后通过 `rg --files internal/project` 找到并读取 `reader_linux.go`。所有实际检查均针对正确源码执行。

## 未运行项与限制

- 真实供应商、真实凭据、成功远程 SSH 会话未授权且不在本批范围；没有调用真实模型服务或连接远程 SSH。PTY 脚本只调用 `ssh -V`。
- 最低 Go 1.26 工具链和用户个人 `.bashrc` 属于后续批次。本轮为 Go 1.27.2，使用临时 HOME 中模拟 `.bashrc`。
- 未提供系统调用级网络/写入 trace：本环境无 `strace`（`command -v strace` 退出 1）。doctor 的无联网结论由本地 endpoint 零请求与完整调用链审查共同支持，无文件修改由完整临时目录内容/元数据快照与调用链审查支持；不把未执行的 syscall trace 写作通过。
- 没有修改产品代码、测试仓库文件、refs、Git 配置或 hooks；未提交、未合并、未删除工作树。独立辅助脚本与报告只存于 `/tmp`。

## 返回开发者的动作

候选 `8fb386f2f893731c7bcc91a57787007bf852100e` 在本批交接范围内独立验收 PASS。开发者可追加本报告及开发日志等证据文档；任何产品、测试、脚本、依赖或操作规范变化需重新独立验收。协调者仍须按 AGENT.md 核对 main 基线、候选身份、后续差异仅为证据与工作树干净状态；本报告本身不授予合并权限。
