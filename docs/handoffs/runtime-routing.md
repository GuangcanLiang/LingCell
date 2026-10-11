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
- 独立报告：待验收。
