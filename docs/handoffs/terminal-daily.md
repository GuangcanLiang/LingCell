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

## 独立本地设施验收报告

# 第三批本地终端验收设施独立测试报告

- 状态：**PASS（本次授权的本地设施验收范围）**。完整第三批真实环境目标仍 **BLOCKED/未完成**，不能将本报告解释为个人日常试用、真实 SSH 或个人 bashrc 已通过。
- 测试 Agent：`/root/terminal_daily_acceptance`，fresh context 最小交接，独立 detached worktree。
- 基线完整 SHA：`6ad5b17ba8d93d98b5b74f8bcbcc64d97a6b734d`。
- 候选完整 SHA：`e960692de1382a1c1d460bfae448fab4a2359afe`。
- 实际 HEAD（开始/结束）：均为 `e960692de1382a1c1d460bfae448fab4a2359afe`。
- 工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-terminal-daily-e960692`。
- 工作树状态（开始/结束）：开始 `## HEAD (no branch)`，无修改；结束 `## HEAD (no branch)`、`!! bin/`，仅忽略构建产物。测试 harness 导入一度产生忽略的 Python 缓存，已删除自己的生成物，后续禁止写字节码。没有改产品源码、候选脚本、refs/config/hooks，没有提交或合并。
- 环境：Linux `6.6.87.2-microsoft-standard-WSL2`、amd64；Go 1.27.2（GOROOT `/usr/local/go`）与 Go 1.26.0（GOROOT `/tmp/lingcell-go126/go`）；Staticcheck `2026.2.1 (0.8.1)`；make 环境 Python 3.12.3，补充 harness Python 3.13.5。vim/top/ssh 均位于 `/usr/bin`，实际启动交互工具及本地版本命令。

所有 make 命令在上述工作目录运行。为完整记录实际环境，以下 `E127` 与 `E126` 代表准确的命令前缀：

```text
E127 = env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/lingcell-terminal-acceptance-go127-cache
E126 = env GOTOOLCHAIN=local PATH=/tmp/lingcell-go126/go/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/lingcell-terminal-acceptance-go126-cache
```

两种环境的 `go env GOVERSION GOTOOLCHAIN GOROOT` 均退出 0，分别确认 `go1.27.2/local//usr/local/go` 和 `go1.26.0/local//tmp/lingcell-go126/go`，没有允许自动切换工具链。Makefile 的 gofmt 也由相应 PATH 选择。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 独立固定候选 | `git worktree add --detach .worktrees/verify-terminal-daily-e960692 e960692de1382a1c1d460bfae448fab4a2359afe`；读取 AGENT.md、README、Makefile、日志、交接、报告模板及基线差异 | 0；4 个文件修改，仅 Python 验收脚本和文档，Go 产品/依赖无变化 | PASS |
| 当前工具链检查 | `E127 make check` | 首次沙箱退出 2：httptest 本地监听 `operation not permitted`。按权限流程沙箱外同命令重跑退出 0；格式、模块 diff、vet、全部竞态测试通过 | PASS |
| 最低工具链检查 | `E126 make check GO=/tmp/lingcell-go126/go/bin/go` | 首次沙箱退出 2：同样本地监听受限。按权限流程同命令重跑退出 0；全部检查通过 | PASS |
| 当前工具链有效 lint | `E127 STATICCHECK_CACHE=/tmp/lingcell-terminal-acceptance-staticcheck127 make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/lingcell-terminal-acceptance-staticcheck127-measurements.txt'` | 0；fresh cache 5,456 条分析计时记录，全部 8 实际包及 7 测试包可见，无诊断 | PASS |
| 最低工具链有效 lint | `E126 STATICCHECK_CACHE=/tmp/lingcell-terminal-acceptance-staticcheck126 make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/lingcell-terminal-acceptance-staticcheck126-measurements.txt'` | 0；fresh cache 5,360 条计时记录，同样全部 8 实际包及 7 测试包可见，无诊断。没有接受零包结果 | PASS |
| 当前构建 | `E127 make build` | 0 | PASS |
| 最低构建 | `E126 make build GO=/tmp/lingcell-go126/go/bin/go` | 0 | PASS |
| 当前三会话 PTY | `E127 make terminal` | 首次沙箱退出 2，打印 FAIL，因为 Python 本地监听被拒；按权限流程沙箱外同命令重跑退出 0。3 独立 CLI 会话全部 PASS，vim/top/ssh -V 实际 PASS，真实远程 SSH/个人 bashrc 明确 SKIP | PASS |
| 最低三会话 PTY | `E126 make terminal GO=/tmp/lingcell-go126/go/bin/go`（允许本地监听的环境） | 0；同样 3 会话、交互 vim/top/ssh -V 实际 PASS，真实环境明确 SKIP | PASS |
| 缺可选工具报告 | `python3 /tmp/lingcell-terminal-acceptance-extra.py skip` | 0；只将 shutil.which(vim/top/ssh) 替换为 None，不改候选。3 会话全部完成，分别打印 `SKIP ... executable unavailable`，没有虚假可选 PASS | PASS |
| 合成 bashrc / SSH 分支 | `python3 /tmp/lingcell-terminal-acceptance-extra.py fixture` | 0；本地 Bash ssh 函数，无网络。3 独立 CLI PID 各验证 alias/function 和保存的原钩子；原 PROMPT_COMMAND 48 次实际执行，来自 3 个独立 Bash PID。核验 SSH argv 10 个参数、固定选项、`--` 和完整目标；含单引号及 `$()` 的目标未执行插值或创建注入哨兵。实际 40×100→36×110 窗口缩放、Ctrl-C trap、退出和本地输入恢复通过 | PASS（本地 fixture） |
| 精确 CLI 参数入口 | `python3 /tmp/lingcell-terminal-acceptance-extra.py cli-fixture`，内部执行候选脚本 `--binary <绝对bin路径> --bashrc /tmp/lingcell-terminal-acceptance-fixture.bashrc --ssh-target <带引号/$()的合成目标>`，argv 数组传参 | 0；候选 CLI 解析两种显式参数后完成全部 3 会话及 SSH fixture 分支，无目标插值 | PASS（本地 fixture） |
| bashrc 必要失败与脱敏 | `python3 /tmp/lingcell-terminal-acceptance-extra.py private-failure` | 外层断言 harness 0；候选脚本子进程 1。合成 bashrc 打印私密哨兵后退出；候选打印 FAIL，未打印哨兵或最终总 PASS | PASS |
| SSH 必要失败与脱敏 | `python3 /tmp/lingcell-terminal-acceptance-extra.py private-ssh-failure` | 外层断言 harness 0；候选脚本子进程 1。本地 ssh 函数版本成功、目标调用打印私密哨兵并返回 42；候选最终 FAIL/非零，错误原始终端片段被省略，未打印哨兵或总 PASS | PASS |
| 最低工具链来源校验 | `sha256sum /tmp/lingcell-go1.26.0.linux-amd64.tar.gz`，与 `/tmp/lingcell-go126-asset.json` 比较 | 0；归档 `aac1b08a0fb0c4e0a7c1555beb7b59180b05dfc5a3d62e40e9de90cd42f88235` 与官方元数据记录一致；未重下载、未替换系统 Go | PASS |
| 文档、Python、空白 | Python 检查 3 个改动 Markdown 文件的 17 个本地链接；`compile(source, filename, 'exec')` 内存编译；`git diff --check 6ad5b17ba8d93d98b5b74f8bcbcc64d97a6b734d e960692de1382a1c1d460bfae448fab4a2359afe` | 全部 0，无缓存编译成功，无链接或空白缺陷 | PASS |
| 身份与状态复核 | 开始/结束 `git rev-parse HEAD`、`git status --short --branch --ignored` | 全部 0，固定 SHA、detached，最终仅忽略 bin/ | PASS |

两套 Staticcheck 的实际包证据为 `lingcell/cmd/lingcell` 和 `lingcell/internal/{agent,app,config,model,project,router,shell}`，计时文件保留在 `/tmp`。

## 缺陷与复现

本次已执行检查未发现缺陷。没有根据工具通过推断所有环境正确。

补充 harness 初次在初始化提示符之前强行等待自定义 PROMPT_COMMAND 终端 marker，退出 1；这一观测假设没有成为候选失败结论。随后改用保存钩子检查和独立 PID 追加日志，实际观察到 48 次钩子执行并成功完成全流程。其他初次命令失败均为沙箱禁止本地 socket，已按权限流程重跑通过，未把失败/未执行结果标为成功。

## 未运行项与限制

- **真实供应商分类、工具调用及真实凭据：未授权且没有选定服务，未运行。** 全部模型请求仅到临时本地 MockModel。
- **成功真实远程 SSH：未提供已授权目标/认证/known_hosts，未运行。** ssh -V 只证明本地版本命令；函数 fixture 只验证本地分支，不能证明 SSH 网络、认证、raw 模式或真实远端 PTY 转发。
- **用户个人 `.bashrc`：未授权，未读取或运行。** 指定文件均为我们创建的合成 fixture；临时 HOME 环境也不代表用户原 HOME 插件兼容。
- **人工三次日常 CLI 会话：尚需用户真实现场操作，未运行。** 自动三进程验收不是个人日常试用记录。
- 本次仅复核已下载发行元数据与归档哈希，没有重新请求发行站点或引入依赖。
- 缺工具分支通过检测 fixture 覆盖，不代表当前系统卸载工具；当前系统上实际存在的工具另有真实执行证据。
- 完整第三批真实环境目标仍受服务、目标、个人配置及人工操作缺口阻塞；本地设施 PASS 不关闭这些缺口。

## 返回开发者的动作

本地设施没有需要修复的阻止项。保留上述真实环境未运行项；不得将本地 fixture 的脚本 PASS 输出转载为真实 SSH/个人配置通过。可把本报告作为固定候选本地设施的独立验收证据，协调 Agent 仍需按 AGENT.md 核对基线、候选、主工作树以及仅证据追加的约束。本报告不授予额外合并、远程连接、凭据或个人配置权限。
