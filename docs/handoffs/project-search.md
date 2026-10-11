# 项目搜索与分段读取：交接

- 需求：增加 Go 内置只读 search_text / read_file_range，复用固定项目边界，有限搜索结论可见，保留旧工具、单 Agent、逐条确认命令。
- 职责：开发 Agent 实现/修复；独立测试 Agent 审查/验收，不改产品代码、提交、合并、refs/config 或调用真实服务。
- 根目录：`/home/liang_guang_can/LingCell`。开发 `.worktrees/project-search` / `feat/project-search`；从原 main 建立后同步独立通过的第一批。
- 基线 main：`c5f0bd09db4b1baff21fab406bc13ae3cd09a415`。候选完整 SHA 由派生消息提供；测试固定 SHA 的 detached `.worktrees/verify-project-search-<sha>`。
- 范围：internal/project 扫描/结果预算与新工具；internal/agent schema、参数、取消、反馈和任务策略；README / AGENT.md 契约和回归测试。
- 验收：递归字面匹配大小写敏感、路径/行号稳定排序、同一行一次；片段实际行范围、1/100 默认、最多200、EOF空、非法参数工具错误；忽略/敏感/越界/安全文件链接/不跟随目录链接/特殊文件/私钥/UTF-8；100匹配、32KiB完整JSON、256KiB共享返回、每任务16MiB扫描、每次8MiB、搜索1MiB单文件和4096项；显式限制和超大跳过；任务预算重置；操作期间Context取消；无shell搜索进程/索引/缓存；旧接口与确认仍兼容。
- 命令：make check、make lint、make build、make terminal，文档链接和 git diff --check；临时可写缓存，本地模拟监听。
- 已知边界：忽略文件检查沿用原机制，不计入文件内容扫描预算；文本/私钥检查只覆盖已扫描前缀；上限恰等于EOF也保守标截断。新工具完整JSON计返回预算，旧工具内容计费。真实模型任务效果、远程SSH、个人配置、人工试用未验证；不配置凭据。
- 首轮候选 `208ece09aa8fe775248976c824f87fcefc0b5f92`：独立测试 Agent 发现阻止合入的缺陷，FAIL，正式报告随后追加。其他 make check/lint/build/terminal 及额外边界检查通过不抵消该缺陷。
- 上轮复现：普通 file 内容 `ok\n`；path = strings.Repeat("./", 17000) + "file"；Context 超时100ms，ReadFileRange 仍卡在 JSON 裁剪，go test -timeout=3s 超时。原始path元数据超32KiB而Clean后路径合法，Agent 128KiB参数上限可触达。期望立即返回预算错误并可取消；实际无限裁剪内容为空后仍继续。
- 修复与复验目标：读取前限制原始路径和元数据预算；裁剪循环检查Context，空内容仍超限时报错；新增超长相对路径回归，复验须独立覆盖该缺陷及全部原验收条件。新候选 SHA 由派生消息提供，旧报告不适用于新代码。
- 新一轮独立报告：PASS，候选 `d2a17cb13220f91e21e96730f65a5c1a48bc8b3a`，完整复验记录见下。

## 首轮独立 FAIL 报告（旧候选）

# 项目搜索/分段读取独立验收报告

- 状态：**FAIL**（一个已复现、阻止合入的无限循环缺陷；其余检查通过）
- 测试 Agent：`/root/accept_project_search`，fresh-context 最小交接。
- 基线完整 SHA：`c5f0bd09db4b1baff21fab406bc13ae3cd09a415`
- 候选完整 SHA：`208ece09aa8fe775248976c824f87fcefc0b5f92`
- 实际 HEAD（开始/结束）：均为 `208ece09aa8fe775248976c824f87fcefc0b5f92`。
- 工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-project-search-208ece0`，detached HEAD。
- 工作树（开始/结束）：开始干净；结束源码/受跟踪文件干净，只生成忽略的 `bin/`。未改产品代码、未提交/合并/修改 refs/config/hooks。
- 环境：Linux 6.6.87.2-microsoft-standard-WSL2 x86_64，UID 1000；Go 1.27.2；Staticcheck 2026.2.1 (0.8.1)；make terminal 使用 `/usr/bin/python3` 3.12.3，临时辅助脚本使用默认 Python 3.13.5。

下列命令均在上述固定目录执行。`GOCACHE`/`STATICCHECK_CACHE` 均为本次独立临时目录，不借用开发者结论。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 固定候选、独立差异审查 | `git rev-parse HEAD`; `git status --short --branch`; `git diff c5f0bd09db4b1baff21fab406bc13ae3cd09a415 208ece09aa8fe775248976c824f87fcefc0b5f92 -- internal/project internal/agent` | 0；9文件，825增加/12删除；安全路径仍使用原 checked/openat2；搜索无 shell 进程/索引/缓存实现 | PASS |
| 全套格式/模块/vet/竞态 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-accept-go127-cache STATICCHECK_CACHE=/tmp/project-search-accept-staticcheck-cache make check` | 初次2，httptest 本地监听被沙箱禁止；按 require_escalated 原命令重跑0。7内部包竞态通过，命令包无测试 | PASS |
| 实际 Staticcheck 分析 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-accept-go127-cache STATICCHECK_CACHE=/tmp/project-search-accept-staticcheck-cache make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/project-search-staticcheck-measurements.txt'` | 0，无诊断。计时文件5456行；全部8源包和测试变体均有160条分析器计时。不是零包运行 | PASS |
| CLI构建 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-accept-go127-cache make build` | 0；生成忽略的 bin/lingcell | PASS |
| 兼容与真实终端 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-accept-go127-cache make terminal` | 初次2，本地socket创建被沙箱禁止；require_escalated 重跑0。三独立CLI会话、shell状态、多行、作业、resize、旧Agent工具/确认/拒绝、错误/取消、历史/终端恢复通过；vim/top交互、ssh -V通过 | PASS |
| 搜索与范围、文本/内核边界 | make check 内的新 `TestSearchAndRange`, `TestSearchBoundaries`, `TestSearchLimitsAndSharedBudgets`, `TestScanAndEntryCaps`, `TestSearchAndRangeCancellation`, `TestCancellationDuringScan` 和既有 project 测试，独立核对代码 | 0；递归字面/区分大小写/同一行一次/排序、EOF 0/0、非法range、嵌套ignore与negation、敏感/越界/安全文件链接/不跟随目录链接/FIFO/私钥/UTF-8；100匹配、1MiB跳过、4096枚举、8MiB调用与16MiB任务扫描、任务重置、预取消和扫描中取消覆盖 | PASS |
| 新工具 schema/实际Reader→Agent反馈/取消终止 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-accept-go127-cache go test -overlay=/tmp/project-search-independent-overlay.json ./internal/project ./internal/agent -run '^TestIndependent' -count=1 -timeout=30s -v` | 最终0，6独立测试通过。真实Reader匹配路径/行号与range内容保持tool角色/ID，空查询是工具错误，无shell调用，取消错误阻止后续模型请求，五工具schema/default/maximum/required符合契约。首次辅助测试源码缺少interface类型断言而编译退出1，修正仅/tmp辅助源码后重跑通过 | PASS |
| 完整JSON预算、实际行范围与共享返回 | 同一独立overlay中的 `TestIndependentEscapedRangeAndActualLines`, `TestIndependentSharedResultAccounting`, `TestIndependentLiteralAndScannedPrefixChecks` | 0；200行范围、转义含中文完整JSON≤32768；新工具按实际JSON字节与旧工具内容字节相加，精确耗尽256KiB后报错且新Task重置；元字符字面匹配、所选范围之后已扫描的NUL/UTF8错误/私钥也拒绝并计扫描字节 | PASS |
| 原始路径元数据极限与取消 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-accept-go127-cache go test -overlay=/tmp/project-search-repro-overlay.json ./internal/project -run '^TestIndependentRangeOversizedMetadata$' -count=1 -timeout=3s -v` | 1；100ms Context已过期仍不停，3s测试超时；栈停在ReadFileRange:387 JSON编码循环 | FAIL |
| 文档与空白 | Python逐一解析Markdown本地链接并检查相对目标；`git diff --check c5f0bd09db4b1baff21fab406bc13ae3cd09a415 208ece09aa8fe775248976c824f87fcefc0b5f92`; `git diff --check` | 0；15份Markdown/32本地链接存在，候选差异和结束工作树空白通过 | PASS |
| 结束身份与干净工作树 | `git rev-parse HEAD`; `git status --short --branch`; `git status --short --ignored`; `git diff --name-only` | 0；HEAD固定，受跟踪/未忽略工作树干净；仅 `!! bin/` | PASS |

## 缺陷与复现

**高严重度、阻止合入：read_file_range 超长但可规范化的路径使Agent永久卡住，Context取消失效。**

- 位置：`internal/project/search_linux.go:311` 初始化 `RangeData.Path` 保留原始 path；`:386-396` 裁剪完整 JSON 循环仅缩短 Content，没有元数据容量限制或空内容终止/Context检查。
- 在临时项目创建常规 UTF-8 文件 `file`，内容 `ok\n`。
- 调用：`ReadFileRange(ctx, strings.Repeat("./",17000)+"file", 1, 1)`；`ctx` 为100ms超时。原始 path 长34004字节，`filepath.Clean` 后是合法 `file`；自动文件边界检查与打开成功。
- JSON 内 path 单独已超过32KiB。Content被清空后每轮编码结果仍超限，循环永远继续且忽略Context。独立测试3s超时退出1，goroutine栈明确在ReadFileRange:387。
- Agent层 `decode` 允许128KiB参数，此约34KiB输入可由模型工具调用触达。顶层取消等待该同步调用结束，无法正常返回用户提示符。
- 期望：对无法容纳的参数/元数据及时返回工具参数或预算错误，并保持取消有效。
- 最小独立复现源：`/tmp/project-search-repro_test.go`，覆盖映射 `/tmp/project-search-repro-overlay.json`。测试源码在/tmp，候选文件未变。
- 已提前通知开发Agent；本报告只适用于旧候选208ece09。开发者在另一个worktree拟定的修复尚未在本报告中验收。

## 未运行项与限制

- 最低Go 1.26未单独运行：本轮已发现阻止合入的功能缺陷，所有正式检查固定Go 1.27.2；新候选复验可视实际收益补最低版本。
- 未配置/读取真实凭据、未请求真实模型供应商、未联调真实模型任务策略效果、未授权远程SSH、未使用个人bashrc、未人工试用或发布。PTY脚本明确报告SSH/个人bashrc SKIP；ssh -V只证明本地命令可运行。
- 文档检查仅核实本地链接目标和差异空白，未联网检查外部文档URL。
- 忽略文件读取预算沿用旧机制，不计入新增扫描预算；扫描前缀文本/私钥检查与恰好EOF上限保守截断已在交接明确，按当前契约验收。
- 既有PTY脚本主要覆盖旧工具与确认；新搜索/range闭环通过真实Reader→Agent的独立overlay测试和现有工具协议测试验证，未另写真实CLI新工具HTTP端到端脚本。

## 返回开发者的动作

1. 修复原始路径/JSON元数据不可容纳时的循环，并添加覆盖取消和工具反馈的回归。
2. 在开发任务分支产生新候选完整SHA，按仓库规则重新派生fresh-context独立测试Agent至新的detached worktree。
3. 本候选不得合入；报告不授予合并权限。协调者核对新候选PASS与AGENT.md各合入条件。

## 修复后独立 PASS 报告

# 项目搜索与分段读取：独立复验报告

- 状态：**PASS**
- 测试 Agent：`/root/retest_project_search`，由全新上下文派生，仅接收最小交接。
- 基线完整 SHA：`c5f0bd09db4b1baff21fab406bc13ae3cd09a415`
- 候选完整 SHA：`d2a17cb13220f91e21e96730f65a5c1a48bc8b3a`
- 实际 HEAD（开始/结束）：均为 `d2a17cb13220f91e21e96730f65a5c1a48bc8b3a`。
- 工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-project-search-d2a17cb`，detached HEAD。
- 工作树状态（开始/结束）：`git status --short --branch` 均只输出 `## HEAD (no branch)`；结束 `git status --short --ignored` 仅输出 `!! bin/`。未修改产品代码、已有测试、Git refs/config/hooks，未提交或合并。
- 环境：Linux `6.6.87.2-microsoft-standard-WSL2`、linux/amd64；Go `1.27.2` / `/tmp/lingcell-go126/go/bin/go` 的 `1.26.0`；Staticcheck `2026.2.1 (0.8.1)`；Python 3.12；本地 Bash/PTY、模拟 HTTP 服务。
- 独立临时测试：`/tmp/project-search-retest-extra_test.go`、`/tmp/project-search-retest-agent_test.go`，通过 `/tmp/project-search-retest-overlay.json` 在原候选目录进行虚拟测试文件覆盖，不改变工作树。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 仓库规范与完整差异自审 | 阅读 AGENTS.md、AGENT.md、README.md、Makefile、development-log、handoff、test-report 模板；`git diff c5f0bd09db4b1baff21fab406bc13ae3cd09a415 d2a17cb13220f91e21e96730f65a5c1a48bc8b3a -- internal` 并逐文件读取实现/测试，审查9个差异文件 | 命令均0；新工具仅Go文件IO/字面匹配，未引入shell搜索进程/索引/缓存；旧接口、确认流程和固定根复用原代码 | PASS |
| 原首轮超长路径无限裁剪复现 | `TestIndependentOriginalTimeoutReproduction`：file=`ok\n`；path=`strings.Repeat("./",17000)+"file"`；Context100ms；独立overlay测试 | Go1.27约180µs / Go1.26约157µs返回 `read_file_range path exceeds result budget`；扫描/返回使用量均0，未触发超时 | PASS |
| 递归字面匹配、大小写、路径/行号稳定排序、同一行一次 | 仓库 `TestSearchAndRange`；独立 `TestIndependentLiteralSpecialAndDirectoryAlias` | `.*`按字面匹配；多次出现一行仅1结果；子目录结果包含相对路径和正确行号，排序稳定 | PASS |
| 片段默认/范围/EOF/非法行参数 | 仓库 `TestSearchAndRange`、`TestSearchRangeToolFeedback`、`TestRangeParameterErrorsAreToolData`；独立行范围矩阵 | 1/100默认；最多200，201/0/负数/null/非整数为工具错误；空行/无最终换行/超过EOF返回实际行范围或0/0空结果 | PASS |
| 忽略、敏感路径、越界、安全文件链接、目录链接、特殊文件、私钥/UTF-8 | 仓库 `TestSearchBoundaries`、旧reader测试；独立目录alias/字面查询测试 | 根/嵌套.gitignore及negation、敏感排除、绝对/../路径、范围外和ignored链接、FIFO、私钥和无效文本均受限；安全文件链接允许；目录链接和链接搜索根不递归；拒绝的私钥仍消耗实际扫描字节 | PASS |
| 100匹配、32KiB完整JSON、256KiB共享返回、UTF-8截断和实际行范围 | 仓库 `TestSearchLimitsAndSharedBudgets`；独立 `TestIndependentLineAndEscapingBudgets`、`TestIndependentMixedReturnAndRejectedScan` | 匹配上限100明确match_limit；JSON转义结果32767B、内容15067B、实际行1..26且UTF-8有效，完整编码长度等于记账；旧读取7次+新搜索共享记账，后续range不能突破256KiB；新Task重置 | PASS |
| 16MiB任务 / 8MiB单次扫描、1MiB搜索文件、4096枚举、显式限额/超大跳过 | 仓库 `TestScanAndEntryCaps`、`TestSearchLimitsAndSharedBudgets`；独立 `TestIndependentScanTaskLimitAcrossTools` | 两次range各扫描8388608B，共16777216B；后续search扫描0并显式task_scan_limit；range预算耗尽错误；超1MiB文件SkippedLarge及large_files_skipped；枚举4096明确directory_entry_limit；新Task扫描重置 | PASS |
| 操作期间取消及Agent取消终止 | 仓库 `TestCancellationDuringScan`、预取消测试；独立 `TestIndependentTrimCancellation` 和 `TestIndependentNewToolCancellationStopsDecision` | 裁剪第8次Context检查返回Canceled，已扫描32768B、返回0；搜索/range的取消和接口Canceled均不继续模型请求、确认或执行同批命令 | PASS |
| schema、批量参数校验、反馈工具角色/ID、旧确认兼容 | 仓库Agent测试；独立 `TestIndependentNewSchemas`、`TestIndependentNewToolArgumentBatchValidation` | 五工具schema顺序与运行上下文一致、附加字段关闭、required/default/min/max准确；缺省/unknown/trailing/128KiB超限参数在任何同批读取前拒绝；新结果继续role=tool和原call ID；旧run_command确认/拒绝、多命令拒绝通过 | PASS |
| Go1.27 make check/lint/build/terminal | 下列完整命令 | 最终全0；Staticcheck5456计时记录、8实际包；PTY三独立会话及vim/top/ssh本地版本通过 | PASS |
| 最低Go1.26 make check/lint/build/terminal | 下列完整命令 | 全0；Staticcheck5360计时记录、8实际包；PTY三独立会话及vim/top/ssh本地版本通过 | PASS |
| 两版本独立临时附加测试 | 下列完整overlay命令，6个project+3个agent独立测试 | 两版本均0，9项PASS | PASS |
| 文档链接和空白 | `python3 /tmp/project-search-retest-docs.py`；`git diff --check c5f0bd09db4b1baff21fab406bc13ae3cd09a415 d2a17cb13220f91e21e96730f65a5c1a48bc8b3a`；`git diff --check` | 最终均0；5份文档25个本地链接存在；两版本Staticcheck8实际包被独立确认 | PASS |

## 完整门禁命令与退出码

所有以下命令均在上列固定detached工作目录执行；`GOTOOLCHAIN=local`禁止工具链自动切换。Go/Staticcheck缓存均为本轮独立/tmp目录。

```bash
# Go1.27.2
env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go127-cache make check
# 首次沙箱内退出2：internal/model httptest listen tcp6 [::1]:0: socket: operation not permitted；按require_escalated重跑退出0。
env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go127-cache STATICCHECK_CACHE=/tmp/project-search-retest-staticcheck-cache make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/project-search-retest-staticcheck-measurements.txt'
# 退出0；5456条记录，8个实际生产包（另含test变体）。
env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go127-cache make build
# 退出0。
env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go127-cache make terminal
# 首次沙箱内退出2：Python socket PermissionError errno1；按require_escalated重跑退出0。
env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go127-cache go test -overlay=/tmp/project-search-retest-overlay.json -count=1 -timeout=30s -v ./internal/project ./internal/agent -run TestIndependent
# 退出0，9个独立附加测试通过。

# Go1.26.0
env GOTOOLCHAIN=local PATH=/tmp/lingcell-go126/go/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go126-cache make check GO=/tmp/lingcell-go126/go/bin/go
# 已知需要模拟服务监听，按require_escalated执行，退出0。
env GOTOOLCHAIN=local PATH=/tmp/lingcell-go126/go/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go126-cache STATICCHECK_CACHE=/tmp/project-search-retest-staticcheck126-cache make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/project-search-retest-staticcheck126-measurements.txt'
# 退出0；5360条记录，8个实际生产包。
env GOTOOLCHAIN=local PATH=/tmp/lingcell-go126/go/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go126-cache make build GO=/tmp/lingcell-go126/go/bin/go
# 退出0。
env GOTOOLCHAIN=local PATH=/tmp/lingcell-go126/go/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go126-cache make terminal GO=/tmp/lingcell-go126/go/bin/go
# 按require_escalated执行，退出0。
env GOTOOLCHAIN=local PATH=/tmp/lingcell-go126/go/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/project-search-retest-go126-cache go test -overlay=/tmp/project-search-retest-overlay.json -count=1 -timeout=30s -v ./internal/project ./internal/agent -run TestIndependent
# 退出0，9个独立附加测试通过。
```

`make check`两版本均包括gofmt、`go mod tidy -diff`、`go vet ./...`和`go test -race ./...`，7个内部包测试通过。Staticcheck记录中实际生产包为cmd/lingcell及internal/{agent,app,config,model,project,router,shell}，未把零包运行作为通过。

两版本PTY最终输出均为：vim/top交互PASS、ssh本地版本PASS、会话1/2/3及终端恢复PASS；remote SSH和personal bashrc明确SKIP。

独立测试设施首次曾有两项预期错误并已纠正：短行200行未达到返回上限，改为每行含100段转义字符以实际触发预算；Staticcheck计数最初包含test变体，改为仅生产包。其初次临时命令退出1不是产品缺陷；修正后的完整独立命令退出0。候选工作树未改变。

## 缺陷与复现

本次已执行检查未发现缺陷。首轮 `208ece09aa8fe775248976c824f87fcefc0b5f92` 的超长原始相对路径无限裁剪问题，在本轮固定候选上已独立复验修复：读取前报预算错误、零扫描/返回记账；另在结果JSON裁剪期间确定性取消通过。

## 未运行项与限制

- 未授权且未调用真实模型/真实供应商/凭据；模拟与接口测试不证明真实模型任务策略质量。
- 成功远程SSH、用户个人bashrc、日常人工试用未运行；PTY输出分别明确SKIP。
- 无syscall/process trace或并发文件替换压力测试；无shell搜索/索引/缓存通过源码及工具行为审查，内核固定根和特殊文件边界复用既有openat2与回归。
- 忽略文件检查不计入扫描预算；文本/私钥检查限于实际扫描前缀，上限恰等于EOF保守截断，均为交接中明确的契约。
- 报告只适用于指定候选；任何后续产品/测试/脚本/依赖/规范变更须重新独立验收。本报告不授予合并权限。

## 返回开发者的动作

无产品修复或关键验收阻塞项。协调者按AGENT.md核对main基线、候选身份与工作树，再追加本轮证据文档并执行已授权的整合流程。
