# Agent runtime 开发 → 独立验收

- 任务/需求：按用户计划实现连续会话、SQLite 有界持久化、显式恢复、请求字节预算与摘要、自动来源记忆和按需检索。恢复不重跑命令、不复用确认、不恢复 Bash 状态；输入分类不入历史。
- 职责：开发 Agent 实现并提交；测试 Agent 全新上下文自行审查差异和验收，不改产品代码、不提交/合并，不修改共享 refs/config/hooks。报告写 `/tmp/lingcell-runtime-acceptance-report.md`，由开发 Agent 保存证据。
- 仓库根：`/home/liang_guang_can/LingCell`。
- 开发 worktree / 分支：`.worktrees/agent-runtime` / `feat/agent-runtime`。
- 测试 worktree：协调者按最终候选创建 `.worktrees/verify-agent-runtime-<short-sha>`，detached；确切绝对路径在交接消息。
- 基线：`main` / `e83f13e52e4029e819dfae194edf55024f23d807`。
- 候选完整 SHA：由交接消息提供（此文件不能写其所在提交自身 SHA）。前三批提交通过 Git 历史可查：storage / sessions-context / sourced-memory；第四批含真实 CLI 回归和开发反馈修正。
- 变化/风险：新 `internal/runtime`、SQLite 依赖、agent/app/main/config/model 及 CLI/PTY 脚本；旧的“不持久保存模型上下文/无跨会话记忆”边界已按用户要求变更，详见 `AGENT.md` 和 `docs/runtime.md`。

## 验收条件

1. 第二轮理解第一轮；正常启动隔离历史，显式重启恢复可追问；根不匹配和并发恢复拒绝。Bash 状态不被历史恢复/切换重放。
2. 用户请求/模型响应/执行开始/结果保存门禁；拒绝、中断、未知结果不重跑；历史确认不复用；合法历史工具配对和 ID 唯一。
3. byte budget 使用实际序列化请求；旧闭合轮次摘要保留最近两轮、约束与原档；摘要失败不失档；当前大工具数据截断并可引用，不能静默删除约束；旧历史最多 32 KiB，可 fragment 分页。
4. 成功保存 Agent 轮次最多一次有界提取；失败/中断不提取；伪造来源、模型推测/提案、工具伪偏好/指令、凭据/授权/瞬态状态拒绝。每轮最多 5 条、内容最多 2 KiB。
5. 新会话记忆按需工具加载；user/project/machine 范围过滤、FTS trigram 与短字面查询、8项/8 KiB；新偏好更新版本、事实冲突保留、过期与重新观察证据失效可见；管理修正/删除更新索引，旧归档不重新提取。
6. SQLite 事务迁移/未知版本/损坏/权限/独占锁、只读与 max_page_count 磁盘满；存储故障停止 Agent，普通 shell 可用；默认路径、0700/0600。敏感模式过滤不夸大保证、不修改实际执行参数。
7. `--ephemeral` 不开数据库、不读写记忆或持久输入历史、不创建 state/cache 历史文件；进程内仍可连续对话。`--no-history` 只控制输入历史。
8. 现有路由/读取边界/普通 Bash/PTY/确认/取消/通用任务场景回归无退化；README/配置示例/维护规范同步且说明准确。

## 建议命令与环境

按 `AGENT.md` 执行 `make check`、`make lint`、`make build`、`make terminal`（三组原 PTY + task fixtures + runtime fixtures）；另外固定最低 Go 1.26 验证新依赖与构建，推荐 `make check GO=/tmp/lingcell-go126/go/bin/go`。运行 `go mod verify`、`git diff --check`、Python 语法与本地文档链接检查。可在 `/tmp` 添加针对性探针，不修改候选工作树源码。

Go 1.27.2 一致工具链目录 `/home/liang_guang_can/.local/share/go/current/bin`，Staticcheck 位于 `/home/liang_guang_can/.local/share/go-dev/tools/bin`；使用 `GOTOOLCHAIN=local` 和 `/tmp` 可写 `GOCACHE`，PATH 显式包含上述目录、`/usr/bin:/bin`。不要使用混合 `/usr/local/go`/环境 PATH。检查 Staticcheck 实际分析了新包，不能仅以错误缓存的快速 exit 0 作为证据。模拟 HTTP 与 PTY 需要执行环境本地监听权限，受限时按工具流程申请执行授权。

## 约束与已知未验证项

仅模拟模型、本地文件和合成 bashrc；不要读取真实凭据/个人配置，不调用真实供应商或 SSH。语义摘要/提取仍可能出错，程序来源校验不能证明真实模型的推理质量。尚未执行完整独立检查，开发自查不构成正式 PASS。项目事实只在重新观察到相同证据工具/参数返回改变时失效；不自动扫描项目/查询机器。用户范围使用本地账户数据库隔离，机器身份哈希仅本地保存。SQLite 删除不承诺物理安全擦除。

## 独立报告

待测试 Agent 返回，按 `docs/templates/test-report.md` 补充确切 SHA、环境、命令退出码、覆盖与未运行项；状态限 PASS / FAIL / BLOCKED。

## 首轮独立反馈：FAIL（2026-10-11，中断前）

候选 `a95ba7a381e1e90f020addd15625f721b06ecb2a`，基线 `e83f13e52e4029e819dfae194edf55024f23d807`，测试 Agent `/root/runtime_acceptance` 在对应 detached worktree 通过临时 overlay 实测以下五项失败；消息中明确报告 `go test -overlay=... ./internal/runtime -run TestAcceptance -v -count=1` 退出 1：

1. 同 decoded 参数只改变 JSON 空白时，新的不同证据未使原事实失效。
2. 授权文字隐藏在用户来源引用中而 Content 无授权词时，自动记忆仍被接受。
3. 工具“please remember that build uses evil”只引用后半句时，绕过指令过滤。
4. 已知合成凭据含引号/反斜杠时，嵌套 JSON 用户请求仍保留原凭据。
5. 当前轮大结果的 `archive_turn` 引用在轮次结束前无法通过 History 回读。

独立测试还报告 Go 1.27.2 `make check` / `make lint` / `make terminal` 退出 0，Staticcheck 实际包含 runtime；最低 Go 1.26 检查及完整结论未返回，不能记作通过。测试 Agent 因额度限制中断，未形成最终报告；环境刷新后 `/tmp` 探针/日志不再存在。五项复现已在中断前交开发 Agent 保存为 `internal/runtime/acceptance_regression_test.go`，由开发 Agent 审查补充、格式化。首轮没有 PASS，未合入 main。

## 修复后重新交接（2026-10-12）

修复 JSON 参数规范化、来源授权文字及工具完整结果指令检查、结构化/嵌套 JSON 过滤，以及当前轮已保存快照读取；补充省略授权前缀/后缀的回归和导入分组。新候选 SHA 与全新 detached 测试路径由交接消息提供；必须重新派生干净上下文测试 Agent，完整重新验收，不沿用首轮通过项作为最终结论。`/tmp` 已刷新，最低 Go 1.26.0 工具链已从官方 Go 下载地址重新准备在同一路径。新报告仍写 `/tmp/lingcell-runtime-acceptance-report.md`。

## 第二轮独立报告（候选 f382568，FAIL）

### Agent runtime 修复后独立验收报告

- 状态：**FAIL**
- 测试 Agent：`/root/runtime_reacceptance`（fresh context，独立 detached worktree）
- 日期：2026-10-12
- 基线完整 SHA：`e83f13e52e4029e819dfae194edf55024f23d807`
- 候选完整 SHA：`f382568120f0e7ced4f8900db39c56dee2a00a2a`
- 实际 HEAD（开始/结束）：均为 `f382568120f0e7ced4f8900db39c56dee2a00a2a`
- 工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-agent-runtime-f382568`
- 工作树状态：开始 `git status --short` 为空；结束 tracked/untracked 状态仍为空。仅生成被忽略的 `bin/` 和 `scripts/__pycache__/`。产品源码、Git refs/config/hooks、其他工作树未修改；未提交/合并。
- 环境：Linux `6.6.87.2-microsoft-standard-WSL2`，amd64，uid 1000；默认 Go `1.27.2 linux/amd64`；最低工具链 `/tmp/lingcell-go126/go/bin/go` 实测 `go1.26.0 linux/amd64`；Staticcheck `2026.2.1 (0.8.1)`，由 Go1.27.2 编译。
- 默认环境前缀 E127：`env PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOTOOLCHAIN=local GOCACHE=/tmp/lingcell-acceptance-go127`。
- 最低工具链环境前缀 E126：`env PATH=/tmp/lingcell-go126/go/bin:/usr/bin:/bin GOTOOLCHAIN=local GOCACHE=/tmp/lingcell-acceptance-go126`。
- 下表命令均在上述测试目录执行；结论全部依据本轮实际运行，不沿用首轮结果。独立 Go 探针通过 `-overlay` 注入 `/tmp` 测试文件，不修改候选工作树。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 候选身份与独立审查 | `git rev-parse HEAD`、`git status --short --branch`、`git diff --stat`，审查基线到候选的 runtime/agent/app/main/config/model/PTY 脚本差异，阅读 AGENTS/AGENT/README/runtime/交接/开发日志 | 指定 detached SHA；规范和文档与主要实现相符；发现下述 list_files 失效遗漏 | FAIL |
| 默认工具链格式、模块、vet、竞态测试 | E127 `make check` | 0；fmt-check、`go mod tidy -diff`、`go vet ./...`、`go test -race ./...`，8个实现包全部通过（runtime 7.723s），CLI包无测试 | PASS |
| Staticcheck 全仓库 | E127 `make lint` | 0；无诊断 | PASS |
| Staticcheck 实际分析新 runtime | E127 加 `STATICCHECK_CACHE=/tmp/lingcell-staticcheck-fresh`，`staticcheck -debug.measure-analyzers /tmp/lingcell-acceptance-staticcheck-measure.txt ./internal/runtime` | 0；2360条测量；`lingcell/internal/runtime`、runtime测试变体、runtime.test各160条，证明实际分析 runtime，不凭快速缓存退出判定 | PASS |
| 默认构建 | E127 `make build` | 0；构建 CLI | PASS |
| 最低 Go1.26.0 全检查 | E126 `make check GO=/tmp/lingcell-go126/go/bin/go` | 0；fmt/tidy/vet/race全通过，8个实现包通过（runtime 7.447s） | PASS |
| 最低 Go1.26.0 构建与模块校验 | E126 `make build GO=/tmp/lingcell-go126/go/bin/go`；E126 `/tmp/lingcell-go126/go/bin/go mod verify` | 均0；CLI构建；all modules verified | PASS |
| 默认工具链模块校验 | E127 `go mod verify` | 0；all modules verified | PASS |
| 现有 Bash/路由/读取/PTY/确认/取消回归 | E127 `make terminal` → `verify_terminal.py` | 0；三个独立真实PTY会话、vim、top、ssh本地版本、Bash状态、续行、作业、缩放、确认拒绝、模型取消、输入历史、终端恢复通过 | PASS |
| 通用任务闭环 | 同一 `make terminal` → `verify_tasks.py` | 0；route/path/var、understand、diagnose、machine、modify、failed、boundary、cancel十场景通过；修改场景6决策/3确认，边界场景5决策/0确认 | PASS |
| 连续会话/恢复/Bash隔离/并发根校验 | 同一 `make terminal` → `verify_runtime.py` | 0；第二轮和显式重启追问、普通新会话隔离、fresh Bash、恢复锁、项目根匹配、历史配对通过 | PASS |
| 拒绝/未知结果不重放、存储故障保留 shell | 同一 runtime PTY 脚本 | 0；拒绝未执行；kill后恢复 unknown，once文件仅1行；失败/中断不提取；损坏DB Agent停止，普通shell仍执行 | PASS |
| 临时模式与输入历史分离 | 同一 runtime PTY 脚本 + 源码核对 | 0；ephemeral进程内连续、禁用持久记忆与提取、无state/cache文件、已有记忆不可检索；no-history仍生成会话DB、不生成输入历史；Init(noHistory)不打开输入历史 | PASS |
| 首轮五项修复的独立变体 | E127 `go test -overlay=/tmp/lingcell-independent-overlay.json ./internal/runtime -run 'TestIndependent(Source\|Evidence\|Redaction\|Summary\|Memory\|Storage\|Extraction\|Archive)' -v -count=1` | 0；重新排列/空白参数失效；完整用户授权前/后缀省略拒绝；工具前/后指令子串拒绝；带引号/反斜杠/换行的嵌套凭据过滤；当前轮已保存大Unicode结果分页完整重建且截断有archive_turn引用 | PASS |
| 来源、敏感/授权/瞬态过滤 | 同一独立 runtime 探针 | 0；伪造session/turn/message、失败step/命令、工具error、项目外cwd、历史/记忆作为来源、工具伪偏好、>2KiB候选、敏感/瞬态内容拒绝；合法用户偏好/证据事实接受 | PASS |
| 请求字节、摘要、原档、分页预算 | 同一独立 runtime 探针 + `make check`中的context测试 | 0；真实EncodeRequest字节=LastBytes、预算内；摘要请求预算内、最近两轮与当前约束保留；超限不删除约束；开发回归中的模型摘要失败保留；Unicode当前轮fragment拼接逐字节等于已保存原档，每页<=32KiB | PASS |
| 记忆范围/索引/版本/冲突/过期/检索预算 | 同一独立 runtime 探针 + `make check` memory测试 | 0；项目/机器过滤；FTS长查询/短字面查询；<=8项/8KiB；修正后可检索、删除移除索引和版本；冲突与过期显式；正常读取证据变化失效通过 | PASS |
| list_files 再观察证据变化 | E127 `go test -overlay=/tmp/lingcell-independent-overlay.json ./internal/runtime -run TestIndependentListEvidenceInvalidation -v -count=1` | **1**；同工具/decoded参数的新列表结果变化，已有事实仍valid | **FAIL** |
| SQLite权限/迁移/只读/磁盘满/恢复/归档界限 | 独立 runtime 探针 | 0；0700目录、0600DB/WAL/SHM/锁；无权限目录拒绝；未知schema/损坏/部分schema拒绝，迁移回滚无turns表；query_only故障锁存；max_page_count写失败保留原记录；根/并发恢复拒绝、unknown配对无重放；>2MiB轮次和>64MiB会话拒绝且不部分写入 | PASS |
| 四个保存门禁和历史权限 | E127 `go test -overlay=/tmp/lingcell-independent-overlay.json ./internal/agent -run TestIndependent -v -count=1` | 0；在request/response/start/result分别关闭数据库，模型/确认/执行次数严格符合门禁；结果失败后无下一决策；下一任务也停止；历史批准后再次提案仍确认并可拒绝 | PASS |
| 自动提取界限与失败语义 | 独立 runtime/PTY探针 + 源码核对 | 0；6候选整体拒绝且不保存、1候选合法提取；每次一次请求且<=上下文/64KiB；失败轮次、ephemeral不提取；App仅在已保存成功Agent轮次提取一次，普通命令不自动提取 | PASS |
| 文档/脚本/差异 | `git diff --check <base> <candidate>`、`git diff --check`；Python `ast.parse` 三个脚本；本地Markdown链接扫描 | 均0；3脚本语法通过，19份Markdown/42本地链接无缺失 | PASS |

## 缺陷与复现

### F1：list_files 来源事实不会在相同证据变化后失效（阻止合入，中等严重）

- 位置：`internal/runtime/memory.go:525` 的 `ObserveEvidence` 证据白名单不含 `list_files`，但 `validateMemory` 的已执行证据工具白名单允许 `list_files` 形成项目事实。
- 最小探针：`/tmp/lingcell-independent-runtime_test.go` 中 `TestIndependentListEvidenceInvalidation`（完整独立helpers也在同文件），overlay：`/tmp/lingcell-independent-overlay.json`。
- 操作：新建临时DB/runtime；保存成功的list_files轮次，参数 `{"path":"."}`，结果 `{"path":".","content":"Makefile\n","truncated":false}`；引用该成功工具的 `Makefile` 字面证据保存 `project/fact/build-files` 内容 `Project includes Makefile`（成功）；下一轮成功list_files使用等价参数 `{ "path" : "." }`，结果变为 `{"path":".","content":"go.mod\n","truncated":false}`；调用ObserveEvidence和ListMemory。
- 期望：来源工具和解码参数一致而结果改变，旧事实为 `pending_verification`。
- 实际：旧事实仍为 `valid`。
- 必要输出：`changed same list_files evidence leaves fact status=valid; expected pending_verification`；命令退出1，日志 `/tmp/lingcell-independent-list-failure.log`。
- 影响：删除或变化后的项目文件列表仍作为有效长期记忆返回，违反交接条件5和runtime说明的重新观察失效规则。已给开发Agent发送最小复现，候选测试目录未修改。
- 本轮已执行的其他检查没有发现产品缺陷；不能由此推断全部行为正确。

## 证据文件

- `/tmp/lingcell-acceptance-check127.log`
- `/tmp/lingcell-acceptance-check126.log`
- `/tmp/lingcell-acceptance-lint.log`
- `/tmp/lingcell-acceptance-runtime-lint.log`
- `/tmp/lingcell-acceptance-staticcheck-measure.txt`
- `/tmp/lingcell-acceptance-terminal.log`
- `/tmp/lingcell-acceptance-build126.log`
- `/tmp/lingcell-acceptance-mod126.log`
- `/tmp/lingcell-independent-probes.log`
- `/tmp/lingcell-independent-agent-probes.log`
- `/tmp/lingcell-independent-list-failure.log`
- `/tmp/lingcell-independent-runtime_test.go`
- `/tmp/lingcell-independent-agent_test.go`
- `/tmp/lingcell-independent-overlay.json`

测试工具修正：初次额外Staticcheck用了不存在的 `-debug.measure`（退出2），改为本机版本支持的 `-debug.measure-analyzers` 后实际分析成功；初始两个探针的预算和查询夹具不适合预期（保留两轮本身超预算，build1亦匹配build10等），只修改/tmp夹具并重跑，通过后才记录上表。这些不作为产品缺陷，也未计入PASS证据。F1使用真实list_files Data形状再次运行，仍失败。

## 未运行项与限制

- 未授权也未运行真实供应商、远程SSH连接、个人bashrc；PTY脚本明确SKIP后两项，仅ssh本地版本不代表远程已验证。
- 语义摘要/提取质量只使用mock验证程序契约，不能证明真实模型理解或自动语义可靠性；任意未知秘密的识别、SQLite物理安全擦除不在实现承诺内。
- 未做真实磁盘耗尽或断电硬件测试；用SQLite max_page_count和query_only实测写入失败/事务门禁。
- 没有尚未完成的本轮必需工具检查；总体FAIL来自确定产品缺陷F1，而不是验证权限或工具阻塞。

## 返回开发者的动作

修复F1，统一可产生事实与可观察失效的证据工具集合，并补回归；在任务分支产生新的完整候选SHA，再派生fresh-context独立测试Agent和新的detached worktree重新验收。此报告只适用于f382568，不对新修复提交授予PASS，也不授予合并权限。

## 目录证据修复后交接

开发已将事实来源校验与失效观察统一为 `evidenceTool`，使 list_files 与其他成功证据工具遵守同一失效契约；新增真实 Data 形状的目录变化回归。新候选 SHA/测试路径由交接消息提供；第三轮 fresh-context Agent 必须独立重新检查新候选，完整报告不沿用第二轮 PASS 项。首轮五项及第二轮 F1 都要复测。

## 最终独立报告（候选 0f60a41，PASS）

### Agent runtime 最终独立验收报告

- 状态：**PASS**
- 测试 Agent：`/root/runtime_final_acceptance`（fresh context，独立 detached worktree）
- 日期：2026-10-12
- 基线完整 SHA：`e83f13e52e4029e819dfae194edf55024f23d807`
- 候选完整 SHA：`0f60a4148840a7308925a67b8806ff5d79ba9e7d`
- 实际 HEAD（开始/结束）：均为 `0f60a4148840a7308925a67b8806ff5d79ba9e7d`
- 工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-agent-runtime-0f60a41`
- 工作树状态：开始 `git status --short` 为空；结束 tracked/untracked 状态仍为空，detached HEAD。仅生成被忽略的 `bin/` 和 `scripts/__pycache__/`。未修改产品代码、Git refs/config/hooks 或其他 worktree，未提交/合并；探针和报告均在 `/tmp`。
- 环境/工具版本：Linux `6.6.87.2-microsoft-standard-WSL2`，amd64，uid 1000；Go `1.27.2 linux/amd64`；最低工具链 `/tmp/lingcell-go126/go/bin/go` 实测 `go1.26.0 linux/amd64`；Staticcheck `2026.2.1 (0.8.1)`。
- E127：`env PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOTOOLCHAIN=local GOCACHE=/tmp/lingcell-final-go127`。
- E126：`env PATH=/tmp/lingcell-go126/go/bin:/usr/bin:/bin GOTOOLCHAIN=local GOCACHE=/tmp/lingcell-final-go126`。
- 下表命令均在上述测试目录执行，结论依据本轮实际运行。重新阅读 AGENTS.md、AGENT.md、README、Makefile、runtime 说明、开发日志、交接条件/两轮 FAIL 记录与基线到候选差异；独立审查 runtime 保存/预算/来源/失效实现以及 Agent/App/CLI/配置/模型/PTY 集成。旧报告仅用于定位待复测项，未沿用旧候选的 PASS。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 候选身份与差异审查 | `pwd`、`git rev-parse HEAD`、`git status --short --branch`、`git log -8 --oneline`、`git diff --stat <base> <candidate>`；源码与文档审查；单独审查 f382568→0f60a41 | 完整候选 SHA、detached、干净；来源与失效使用同一个 evidenceTool，包含 list_files | PASS |
| 默认工具链全检查 | E127 `make check` | 0；fmt-check、`go mod tidy -diff`、`go vet ./...`、`go test -race ./...` 全通过；8个内部包通过，runtime 7.773s | PASS |
| 最低 Go1.26.0 全检查 | E126 `make check GO=/tmp/lingcell-go126/go/bin/go` | 0；格式、模块、vet、竞态测试全通过；8个内部包通过，runtime 8.163s | PASS |
| 全仓库 Staticcheck | E127 加 `STATICCHECK_CACHE=/tmp/lingcell-final-staticcheck`，`make lint` | 0；无诊断 | PASS |
| Staticcheck 实际分析 runtime | E127 加 `STATICCHECK_CACHE=/tmp/lingcell-final-measure-cache`，`staticcheck -debug.measure-analyzers /tmp/lingcell-final-analyzers.txt ./internal/runtime` | 0；2,360条测量，runtime实现、runtime测试变体、runtime.test各160条，实际分析新包 | PASS |
| 默认构建 | E127 `make build` | 0；CLI构建成功，make terminal亦独立构建 | PASS |
| 最低版本构建 | E126 `make build GO=/tmp/lingcell-go126/go/bin/go` | 0；CLI构建成功 | PASS |
| 默认/最低工具链模块校验 | E127 `go mod verify`；E126 `/tmp/lingcell-go126/go/bin/go mod verify` | 均0；均输出 all modules verified | PASS |
| 现有 Bash/路由/PTY/确认/取消 | E127 `make terminal` → `verify_terminal.py` | 0；三个独立真实CLI/PTY会话，vim/top、本地ssh版本、Bash状态、多行、作业、缩放、Agent工具、错误、拒绝/取消、历史和终端恢复通过 | PASS |
| 通用任务闭环与读取边界 | 同一 `make terminal` → `verify_tasks.py` | 0；route/route_path/route_var、understand、diagnose、machine、modify、failed、boundary、cancel十场景通过；modify 6决策/3确认，boundary 5决策/0确认 | PASS |
| 多轮/重启恢复/新会话隔离/Bash状态 | 同一 `make terminal` → `verify_runtime.py` | 0；第一轮约束进入第二轮与显式恢复追问；新会话无旧对话/自动记忆注入；新Bash变量unset；锁和根不匹配拒绝；历史调用/结果配对 | PASS |
| 拒绝/中断/未知结果与存储故障 | runtime PTY脚本 + 独立存储探针 | 0；拒绝不执行；kill后unknown提示，once文件仅1行，不重放；损坏DB不调用模型但普通shell可用；恢复unknown历史补齐，不赋予权限 | PASS |
| ephemeral/no-history | runtime PTY脚本 + Init/CLI源码核对 + 独立探针 | 0；ephemeral仍进程内连续，不创建state/cache、不检索既有记忆、不提取；Init跳过持久输入历史；no-history生成会话DB但不生成输入历史 | PASS |
| 首轮五项缺陷重测 | E127 `go test -overlay=/tmp/lingcell-final-overlay.json ./internal/runtime ./internal/agent -run 'TestIndependent|TestFinal' -v -count=1` | 0；等价decoded参数空白/重排变化失效；用户授权前/后缀省略拒绝；完整工具结果中的记忆指令前/后缀拒绝；带引号/反斜杠/换行合成凭据从嵌套JSON过滤；当前轮大Unicode结果有archive_turn引用且分页可完整重建 | PASS |
| 第二轮目录列表缺陷重测与新证据矩阵 | 同一overlay命令：TestIndependentListEvidenceInvalidation、TestFinalEvidenceToolContract | 0；真实list_files Data变化后旧事实pending_verification；list_files/read_file/read_file_range/search_text/run_command五工具同契约：未变仍valid、不同参数不失效、等价参数的新结果变化使事实失效 | PASS |
| 来源、敏感/授权/瞬态边界 | 同一overlay：TestIndependentSourceBoundary + 仓库memory测试 | 0；伪造session/turn/message、模型提案、失败工具/命令、error、项目外cwd、历史/记忆工具作为证据、工具偏好、敏感/瞬态内容与超2KiB拒绝；实际用户偏好和成功工具事实接受 | PASS |
| 请求字节/摘要失败/原档/分页 | 同一overlay：RedactionAndActiveFragments、SummaryBudgetAndFailure；仓库context测试；下述额外探针 | 0；实际EncodeRequest字节与LastBytes一致；保留最近两轮/当前约束，不能满足预算时停止；摘要请求受预算；模拟摘要故障只请求一次且through/summary不变，五轮原档保留；Unicode当前轮fragment逐字节重建保存记录，每页<=32KiB | PASS |
| 记忆范围/索引/版本/冲突/过期/检索预算 | 同一overlay：TestIndependentMemoryIsolationIndexBounds + 仓库memory测试 | 0；项目/本机隔离、FTS长查询/短字面查询；每次<=8项/8KiB；管理修正更新索引，删除移除索引/版本；偏好版本更新；事实冲突与过期明确；旧归档不自动提取 | PASS |
| SQLite权限/迁移/故障/归档限额 | 同一overlay：StorageTransactionsFaultAndRestore、ArchiveBoundsAndPermissions + 仓库store测试 | 0；目录0700、DB/WAL/SHM/锁0600；无权限目录拒绝；未知/损坏/部分schema拒绝，迁移事务回滚；query_only故障锁存、max_page_count写失败原记录不变；2MiB轮次/64MiB会话超限拒绝且不部分写入 | PASS |
| 四个保存门禁与新确认 | 同一overlay：agent/TestIndependentRuntimeGates、TestIndependentHistoricalPermission | 0；request/response/start/result处关闭DB，模型/确认/执行次数严格符合门禁；结果写失败无下一模型决策，下一任务停止；历史获批后再次提案仍确认并可拒绝 | PASS |
| 自动提取界限/失败语义 | 同一overlay：TestIndependentExtractionBoundsEphemeral + PTY/源码 | 0；成功已保存Agent轮次由App最多调用一次有界提取；6候选整体拒绝无写入，1合法候选接受；失败轮次/ephemeral不提取；请求<=上下文预算且<=64KiB；普通shell不提取 | PASS |
| 默认存储位置、包含工具schema的真实wire预算、明确摘要故障 | E127 `go test -overlay=/tmp/lingcell-final-overlay.json ./internal/runtime -run 'TestFinalDefaultStoragePath|TestFinalToolSchemaBudgetAndSummaryFailure' -v -count=1` | 0；合成HOME/XDG路径符合默认值；工具schema、HTML转义/中文JSON工具数据全部计入实际请求；模型摘要故障返回错误，仅请求一次，原摘要/覆盖及5轮记录保留 | PASS |
| 文档、Python/hooks语法、差异 | `git diff --check <base> <candidate>`、`git diff --check`；3 Python脚本ast.parse；本地Markdown链接扫描；`bash -n .githooks/pre-commit`；`bash -n .githooks/commit-msg` | 均0；19份Markdown/42本地链接无缺失，3 Python及2 hooks语法通过 | PASS |
| 结束身份与工作树核对 | `git rev-parse HEAD`、`git status --short --branch`、`git status --short --ignored` | HEAD未变、detached、tracked/untracked为空；仅忽略的bin和Python缓存 | PASS |

## 缺陷与复现

本次已执行检查未发现产品缺陷。首轮五项及第二轮目录列表失效均在本候选重新实测通过；没有尚未解决的本轮阻塞项。上述检查不能推断所有输入、真实模型语义或环境均正确。

独立探针先逐项审读此前临时测试，复制到新的/tmp文件并更新overlay指向本候选；新增五工具证据矩阵、默认路径和包含工具schema/转义的预算探针。本轮额外预算夹具初次直接使用原始文本工具Content，未遵循Agent实际json.Marshal结果形状，导致预算探针退出1；修正/tmp夹具为真实JSON工具数据并补助手调用配对后额外探针退出0。这不是产品路径缺陷，未将该失败计作PASS证据；最终保存的探针为修正夹具。

## 证据文件

- `/tmp/lingcell-final-check127.log`、`/tmp/lingcell-final-check126.log`
- `/tmp/lingcell-final-lint.log`、`/tmp/lingcell-final-runtime-lint.log`、`/tmp/lingcell-final-analyzers.txt`
- `/tmp/lingcell-final-build127.log`、`/tmp/lingcell-final-build126.log`
- `/tmp/lingcell-final-mod127.log`、`/tmp/lingcell-final-mod126.log`
- `/tmp/lingcell-final-terminal.log`
- `/tmp/lingcell-final-probes.log`、`/tmp/lingcell-final-extra-probes.log`
- `/tmp/lingcell-final-runtime_test.go`、`/tmp/lingcell-final-agent_test.go`、`/tmp/lingcell-final-overlay.json`

## 未运行项与限制

- 未授权也未运行真实供应商、远程SSH连接、个人bashrc；PTY脚本明确SKIP后两项，本地ssh版本检查不证明远程连接成功。
- 摘要/提取只用mock验证程序契约；来源校验不能证明真实模型语义判断正确，也不保证识别任意未知秘密；SQLite物理安全擦除不在承诺范围内。
- 未做真实磁盘耗尽/断电硬件测试；以SQLite max_page_count、query_only与关闭数据库验证故障和事务门禁。
- 最低Go1.26.0已跑完整check、build、mod verify；PTY和Staticcheck在默认Go1.27.2执行，不宣称最低版PTY/Staticcheck另行通过。
- 没有缺少权限/工具或尚未运行的本轮必需检查；运行仅使用本地模拟服务、临时文件和合成配置。

## 返回开发者的动作

保存本报告与开发日志证据，核对main仍为交接基线、任务候选仍为已验收SHA以及AGENT.md合入门禁。验收后仅可追加证据文档；代码、测试、依赖、脚本、hooks、Makefile或规范/操作说明变化需要新的独立验收。本报告仅适用于0f60a4148840a7308925a67b8806ff5d79ba9e7d，本身不授予合并权限。

## PR 发布记录（2026-10-12）

用户明确授权创建 PR 后，已将 `feat/agent-runtime` 推送到 origin 并创建 [PR #1](https://github.com/GuangcanLiang/LingCell/pull/1)：`feat(runtime): add resumable sessions and sourced long-term memory`，目标 main。创建时 head 为 `2f2c80123483dca8cbc4f146724682f384b90e5f`，目标基线仍为 `e83f13e52e4029e819dfae194edf55024f23d807`；受验产品代码仍为 `0f60a4148840a7308925a67b8806ff5d79ba9e7d`，之后仅追加日志/交接证据。PR 描述包含最终验收结果和报告链接，不授予合并权限；本次没有合并或修改 main。
