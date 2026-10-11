# 终端验收设施与双工具链交接

- 任务范围：第三批本地验收设施；三独立 CLI 自动会话、准确可选能力结果、SSH/个人配置显式入口、Go 1.26.0 与当前 Go 检查。不代表个人日常试用和真实远程验收完成。
- 开发/测试职责：开发提交脚本和说明；fresh-context 测试 Agent 在固定候选 detached worktree 独立验证，不修改产品或提交/合并。
- 仓库：`/home/liang_guang_can/LingCell`。
- 开发 worktree / 分支：`.worktrees/terminal-daily` / `test/terminal-daily`。
- 基线完整 SHA：`6ad5b17ba8d93d98b5b74f8bcbcc64d97a6b734d`。
- 候选完整 SHA / detached 测试目录：提交后最小交接提供，报告填写。
- 改动：scripts/verify_terminal.py、README、证据文档；无 Go 产品/依赖变更。

## 验收条件

1. `make terminal` 完成三个独立 CLI 进程，普通命令/多行/状态/作业/取消/恢复无未解决失败；默认不请求真实模型或 SSH，不加载用户个人配置。
2. 可选工具存在时实际执行并 PASS，缺失时 SKIP；ssh 本地版本与远程验证区分；任何必要检查异常打印 FAIL 且非零退出，不能最终 PASS。
3. 可选 SSH 分支的命令引用、窗口/中断/退出行为需用本地模拟 ssh 函数或等价 fixture 检查（不得当作真实 SSH）。`--bashrc` 用合成 alias/function/PROMPT_COMMAND 及私密输出 fixture 检查加载、错误时不泄露私密片段。没有真实目标或个人配置授权则明确未运行。
4. Go 1.26.0 与当前工具链均 `make check`、有效 `make lint`、`make build`、`make terminal`；使用 `GOTOOLCHAIN=local` 且 PATH 指向相应工具链，防止自动升级。最低工具链 `/tmp/lingcell-go126/go/bin/go`，来源/校验记录在日志；可自行复核元数据与归档 SHA256。
5. 文档链接、Python 无缓存编译、差异空白、开始/结束固定 SHA 与工作树状态检查。

## 约束与报告

创建 `.worktrees/verify-terminal-daily-<short>` detached 候选 worktree；阅读 AGENT.md 与差异。允许临时本地 HTTP/PTY 和合成 bashrc/ssh fixture；禁止真实服务/SSH、读取用户密钥或私人 bashrc、修改代码/refs/config/hooks、提交/合并。可临时脚本验证缺工具/失败输出，不改候选脚本。若发现缺陷返回复现，不自行修产品。Staticcheck 防零包，必要时按权限流程重跑；可用临时 STATICCHECK_CACHE。报告 `/tmp/lingcell-terminal-daily-report.md`，明确本地设施 PASS/FAIL/BLOCKED 与完整第三批尚未验证项。

真实供应商/成功远程 SSH/个人 bashrc/用户人工三会话日常试用均待用户配置与现场测试；不会因为这些不在当前设施验收范围而宣称它们 PASS。
