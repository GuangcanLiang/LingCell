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
