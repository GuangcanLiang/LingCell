# 通用任务回归：交接

- 需求：同一Agent完成项目理解、开发排障、机器排障、修改代码与实际验证；拒绝/失败/边界/取消可观察。模拟测试不代表真实模型效果。
- 职责：开发Agent维护固定fixture与回归；独立测试Agent审查/运行，不修改产品代码、提交、合并、refs/config 或调用真实服务。
- 根：`/home/liang_guang_can/LingCell`；开发 `.worktrees/task-scenarios` / `test/task-scenarios`。基线main：`03897097aefdb89c5544231c50316b5b3b2bf597`（已独立验收工具集），候选完整SHA由派生消息提供；测试固定候选 detached `.worktrees/verify-task-scenarios-<sha>`。
- 范围：scripts/verify_tasks.py 新固定任务模拟服务+真实CLI/PTY；Makefile terminal 接入；README、场景文档与日志。另修正 agent 初始上下文注释，并补齐 router 无空格自然语言前缀含路径/赋值示例的安全优先级。
- 验收：详见 [固定任务场景](../task-scenarios.md)，每项都检查真实文件/命令结果和工具数据，模型不接受伪造预期输出；所有生成命令须确认；修改前后差异/验证实际，验证失败不可输出模拟成功；机器状态先获批查询，处理拒绝后无执行/后续请求；边界不泄漏；Ctrl-C和终端恢复；历史无生成命令/工具数据；记录决策/确认数；旧三会话兼容；未知首词“帮我解释PATH=/bin; echo ROUTE_MARKER”“请解释/tmp/file”“帮我解释$PATH=/bin”仍进入Agent，普通./脚本、赋值、复合结构和已知名称仍直通，:sh保留强制执行。
- 命令：make check、make lint（实际包分析）、make build、make terminal（含新任务脚本）；Python语法、文档链接、git diff --check。可写临时Go/Staticcheck缓存、本地HTTP/PTY权限；不得升级依赖。
- 限制：固定项目是Bash代码，机器输出来自合成文件；模拟模型最终文本按fixture证据生成，不验证真实模型决策质量；扫描中取消/完整工具边界由已有Go回归覆盖，CLI取消在文件读取后的模型等待阶段。真实供应商、SSH、个人配置、人工日常试用仍未验证。
- 首轮候选 `90110036e58b5db3f21e9a3e244d5fbbb4833c79`：开发侧额外审阅发现模拟配置遗漏api_key_env，默认LINGCELL_API_KEY可从父环境继承并发送本地mock的Authorization；已通知独立测试Agent确认，旧候选不合入。
- 复现/期望：仅设置合成LINGCELL_API_KEY哨兵运行任务fixture，旧配置默认读入该值；测试必须不读取或发送父环境凭据。修复显式api_key_env为空，并在mock断言无Authorization；复验必须带合成父环境哨兵验证所有场景通过，不读取真实值。
- 新候选完整SHA由派生消息提供；需全新上下文重新执行原验收条件及该回归。
- 次轮候选 `eccb70c576af6efca68162aa410e3fc80bcfcb75` 复验中补查发现基线 router 在请求前缀前用 first 包含 /= 过滤，导致上述无空格自然语言输入误路由Shell。不得执行这些测试输入，使用纯Route断言即可复现。已删除过宽过滤并扩展回归矩阵，已知命令仍最高优先，其余普通路径/赋值由原有结构规则处理。
- 最终新候选必须重新派生全新上下文独立验收，覆盖首轮凭据隔离、次轮路由缺陷与全部任务场景/门禁。
- 最终独立报告：候选 `20279d727a42ba6d0210a34516f3e21871ca71f7` 全新上下文验收 **PASS**；完整两轮FAIL及最终PASS报告如下。Go1.27完整门禁与最低Go1.26路由/构建/新场景通过，真实模型效果仍未验证。受测之后仅追加证据，实际整合以Git历史为准。


## 首轮独立验收原始报告

# 通用任务场景独立验收报告

- 状态：**FAIL**（有阻止合入的凭据隔离缺陷）。
- 测试 Agent：`/root/accept_task_scenarios`，fresh context，独立 detached 工作树。
- 基线完整 SHA：`03897097aefdb89c5544231c50316b5b3b2bf597`。
- 候选完整 SHA：`90110036e58b5db3f21e9a3e244d5fbbb4833c79`。
- 实际 HEAD（开始/结束）：均为候选完整 SHA。
- 工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-task-scenarios-9011003`；下述命令均在此执行。
- 工作树状态（开始/结束）：开始 `## HEAD (no branch)`；结束 tracked/untracked 干净，仅忽略的 `!! bin/` 构建产物。未修改仓库源码、提交、refs、config 或 hooks。
- 环境：Linux 6.6.87.2-microsoft-standard-WSL2 / amd64；Go 1.27.2；Staticcheck 2026.2.1 (0.8.1)；Python 3.13.5；独立 /tmp Go 与 Staticcheck 缓存。

| 验收条件/检查 | 确切命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 仓库规范/最终冲突解决审查 | 读取 AGENTS.md、AGENT.md、README.md、Makefile、开发日志、任务交接、场景文档、报告模板；`git diff --stat 03897097aefdb89c5544231c50316b5b3b2bf597 90110036e58b5db3f21e9a3e244d5fbbb4833c79`；审查完整新脚本和最终差异 | 7文件、327插入/1删除；Makefile保留旧验收并追加新脚本；产品执行源码仅普通注释变化；发现fixture凭据隔离缺陷 | FAIL |
| make check | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/task-scenarios-accept-go127-check make check` | 2；格式、tidy、vet已完成；agent/app/config/project/router/shell竞态包测试通过；model包 httptest 监听 `[::1]:0` 被沙箱拒绝。发现阻止合入缺陷后按协调者要求停止昂贵门禁，未重跑 | BLOCKED |
| make lint，实际分析非零 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/task-scenarios-accept-go127-lint STATICCHECK_CACHE=/tmp/task-scenarios-accept-staticcheck make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/task-scenarios-staticcheck-measurements.txt'` | 0，无诊断；5,456条分析记录，3,520条LingCell分析记录，全部8个应用包均出现（含测试变体） | PASS |
| make build | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/task-scenarios-accept-go127-check make build` | 0，生成忽略的bin/lingcell | PASS |
| 凭据隔离负向复现 | `env LINGCELL_API_KEY=synthetic-acceptance-probe-only PYTHONDONTWRITEBYTECODE=1 python3 /tmp/task-scenarios-credential-probe.py` | 沙箱首次1（本地监听EPERM）；按require_escalated重跑0；真实CLI第一场景3个本地HTTP请求均携带合成Bearer凭据，独立确认缺陷 | FAIL |
| Python语法、注释执行内容、文档文件链接、Staticcheck证据 | `python3 /tmp/task-scenarios-source-audit.py` | 0；两脚本AST语法通过；Go文件只替换那条普通注释后字节完全一致，含字符串与构建内容未变；4文档23本地文件链接存在；分析记录验证通过 | PASS |
| 差异空白 | `git diff --check 03897097aefdb89c5544231c50316b5b3b2bf597 90110036e58b5db3f21e9a3e244d5fbbb4833c79` | 0，无输出 | PASS |
| 开始/结束版本与状态 | `git rev-parse HEAD`；开始 `git status --short --branch`；结束 `git status --short --branch --ignored` | 0；HEAD相同，tracked/untracked干净，仅忽略构建产物 | PASS |

## 缺陷与复现

**高严重程度 / 阻止合入：固定本地模拟任务可能读取并发送真实环境凭据。**

- 位置：`scripts/verify_tasks.py:187` fixture配置省略 `api_key_env`；`internal/config/config.go:49` 默认该字段为 `LINGCELL_API_KEY`；`scripts/verify_terminal.py:103` Terminal从父环境复制环境；`internal/model/client.go:109-110`读取配置指定环境变量，并设置Authorization。
- 期望：无认证本地mock任务明确禁用凭据读取，不发送Authorization。仓库说明声称不使用凭据。
- 实际：父进程若含默认凭据变量，真实CLI会读取并向本地mock发送其值。本复现只用显式合成值覆盖变量，没有查询、打印或使用原有真实值。
- 操作：运行表格中的负向复现命令。临时probe导入候选脚本、仅包裹TaskModel HTTP处理器检查Authorization是否等于已知合成字符串，然后实际执行verify；首个项目理解场景完成后终止剩余场景并正常清理临时fixture/PTY/server。
- 最小输出：`CONFIRMED defect: 3 local model requests carried the synthetic inherited default-key Authorization header`。
- 影响：本地验收隔离契约不成立；不得据此候选合入。
- 建议：fixture显式写 `"api_key_env": ""`；mock显式断言无Authorization；修订后以默认凭据变量为合成非空值运行完整新任务脚本，并在新候选上重新独立验收所有门禁。

## 未运行项与限制

- 发现阻止合入缺陷并收到协调者停止昂贵门禁指示后，未运行完整 `make terminal`、旧三会话完整验收、新7场景完整验收、最低Go1.26新脚本、追加取消/历史负向检查。只有凭据复现中的第一项目理解场景实际完成搜索→片段→最终回答；不把其余场景标为通过。
- 本轮make check因沙箱本地监听限制未完整通过；Go1.27竞态全套仍需新候选权限重跑。
- 静态审查确认七场景设计与文档相符，但未据此称实际全流程通过。
- 真实供应商、SSH、个人配置、真实机器处理与人工日常试用未运行，也未经授权；本地模拟结果不证明真实模型任务决策效果。
- 本地Markdown检查验证文件目标存在，没有网络验证外链。

## 返回开发者的动作

修复显式无凭据配置及mock授权头断言，提交新候选，再派生fresh-context独立测试Agent在新detached工作树完整复验。本FAIL报告只适用于上列候选；报告不授予合并权限。


## 次轮独立复验原始报告

# 通用任务复验报告

- 状态：**FAIL**（凭据修复与既定任务回归通过；独立补查确认中文请求路由缺陷，阻止本候选整合）
- 测试 Agent：`/root/retest_task_scenarios`，fresh context；只在独立 detached 工作树读与测试，不修改产品代码、提交、合并、refs/config。
- 基线完整 SHA：`03897097aefdb89c5544231c50316b5b3b2bf597`
- 候选完整 SHA：`eccb70c576af6efca68162aa410e3fc80bcfcb75`
- 实际 HEAD（开始/结束）：均为上述候选。
- 工作树：`/home/liang_guang_can/LingCell/.worktrees/verify-task-scenarios-eccb70c`；开始/结束 tracked 状态干净，结束仅忽略的 `bin/` 构建产物。
- 环境：Linux 6.6.87.2-microsoft-standard-WSL2 x86_64；Go 1.27.2、最低工具链 Go 1.26.0；Staticcheck 2026.2.1 (0.8.1)；GNU Bash 5.2.21；默认 Python 3.13.5，固定 PATH 的 make terminal 使用 /usr/bin/python3 3.12.3。
- 独立完整差异审查：基线到候选仅7文件，332添加/1删除；Makefile新增任务脚本调用、任务Python回归、README和3份文档、一行Go注释。全部差异含最终整合结果已审查。对唯一Go文件做完整字节比较，仅将新注释归一回旧注释后与基线完全相同；可执行内容、字符串、约束不变。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 独立环境与身份 | `pwd; git status --short --branch; git rev-parse HEAD`，开始/结束均执行 | 0；detached候选，tracked干净 | PASS |
| 差异空白 | `git diff --check 03897097aefdb89c5544231c50316b5b3b2bf597 eccb70c576af6efca68162aa410e3fc80bcfcb75` | 0 | PASS |
| 格式、模块、vet、竞态 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/lingcell-task-retest-go127-cache make check` | 初次沙箱2：model httptest禁止监听；按require_escalated重跑0，7内部包竞态测试通过，cmd无测试 | PASS |
| Staticcheck实际分析 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/lingcell-task-retest-go127-cache STATICCHECK_CACHE=/tmp/lingcell-task-retest-staticcheck-cache make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/lingcell-task-retest-staticcheck-measurements.txt'` | 0；计时文件5456行，8项目包（cmd与7内部包）均实际分析，包含测试变体，无诊断 | PASS |
| Go1.27构建 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/lingcell-task-retest-go127-cache make build` | 0 | PASS |
| 原3会话+新7场景；父环境合成哨兵 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/lingcell-task-retest-go127-cache LINGCELL_API_KEY=SYNTHETIC_TASK_CREDENTIAL_SENTINEL PYTHONDONTWRITEBYTECODE=1 make terminal` | 初次沙箱2：禁止socket；按require_escalated重跑0。3旧会话PASS；vim/top实际交互和ssh本地版本PASS；7新场景全部PASS | PASS |
| 最低Go1.26构建 | `env GOTOOLCHAIN=local PATH=/tmp/lingcell-go126/go/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/lingcell-task-retest-go126-cache make build GO=/tmp/lingcell-go126/go/bin/go` | 0；后续补充任务脚本用此二进制 | PASS |
| 独立HTTP/确认/历史/迟到响应观察；Go1.26新7场景 | `env LINGCELL_API_KEY=SYNTHETIC_TASK_CREDENTIAL_SENTINEL PYTHONDONTWRITEBYTECODE=1 python3 /tmp/lingcell-task-retest-extra.py`（require_escalated） | 0；25请求、0 Authorization头、7次真实确认、JSONL历史逐项等于用户顶层输入；等待2秒确保迟到回答已到达，CLI仍活着并能运行普通命令，取消回答未显示；7场景PASS | PASS |
| 凭据断言负向注入 | `/tmp/lingcell-task-retest-negative/verify_tasks.py`只删除显式空api_key_env，保持mock断言；`env LINGCELL_API_KEY=SYNTHETIC_TASK_CREDENTIAL_SENTINEL PYTHONDONTWRITEBYTECODE=1 python3 /tmp/lingcell-task-retest-negative-probe.py`（require_escalated） | **1（预期）**；1个合成Authorization头，mock errors严格等于 `AssertionError: mock received an unexpected credential header`，首场景未通过 | PASS |
| Python语法/文档链接/Go执行内容 | `python3 - <<'PY'`用ast.parse读取2脚本；re匹配4份修改Markdown的本地链接并检查目标exists；git show基线Go文本与新文本注释归一后做完整相等断言 | 各检查通过：2Python脚本、4Markdown/23本地链接、Go可执行内容完全相同 | PASS |
| 无副作用路由补查 | 临时复制go.mod/go.sum/internal/router到`/tmp/lingcell-route-retest`，添加只调用Router.Route的测试；`env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/usr/bin:/bin GOCACHE=/tmp/lingcell-task-retest-go127-cache go test ./internal/router -run TestIndependentCompactRequestPrefix -count=1 -v`（cwd临时副本） | **1**；2个未知首词/中文请求前缀输入实际Shell，预期Agent；未执行输入 | FAIL |

## 七场景实际观察

| 场景 | 决策/确认 | 证据 |
| --- | --- | --- |
| understand | 3/0 | search122行，再实际119–122行片段，引用入口 |
| diagnose | 4/1 | 真正用户命令退出1；错误作为tool数据；配置9000证据与语法验证 |
| machine | 2/2 | 获批读取合成状态；处理被拒，前后无marker，未继续决策 |
| modify | 6/3 | 确认前源码原样；只改9000→9100；真实diff退出1；实际验证成功 |
| failed | 2/1 | 真正验证脚本退出7和失败证据反馈，无模拟成功声明 |
| boundary | 5/0 | `.env.local`、ignored、`../outside`、FIFO均tool错误；哨兵不入请求/历史 |
| cancel | 3/0 | 实际搜索+读取后模型等待Ctrl-C；shell恢复；延迟回答被丢弃；退出终端模式恢复 |

## 缺陷与复现

- 严重程度：高；阻止合入。位置 `internal/router/router.go:53`（该缺陷继承基线，非此次新增，但不符合本轮完整验收意图及AGENT.md必须保持行为第2条）。
- 复现：令 `Router.Knows` 总是返回false，只调用 `Route("帮我解释PATH=/bin; echo ROUTE_MARKER")` 和 `Route("请解释/tmp/file")`。
- 期望：明确自然请求前缀且首词未知，返回Agent。
- 实际：两者均返回Shell，Reason为 `Bash command or shell structure`。在真实CLI中第一项会被交给Bash，分号后的命令具有执行风险；本验收没有执行原输入。
- 原因：requestPrefix前的 `!strings.ContainsAny(first, "/=")` 排除过宽，中文请求不需要在请求前缀与路径/赋值之间插入空格。
- 开发协调者已收到复现，准备新候选；本报告不作为ecc候选的整合PASS证据。
- 已修复的旧候选凭据问题本轮独立正负检查均通过；仅使用合成环境哨兵，从未读取或显示真实凭据。

## 检查过程中的探针调整

- 初次临时Staticcheck证据脚本错误地把测试变体也算为独立产品包，断言8包失败；随后按不含空格/`.test`规范化统计确认8产品包实际分析。这不是产品或lint失败。
- 初次额外历史探针把JSONL整行当用户字符串比较，退出1；读取readline依赖的既有持久化格式后改为提取`block`再比对，重新运行退出0。7个任务场景本体在该次也均通过。
- 以上修改均仅/tmp验收探针，未改候选文件。

## 未运行项与限制

- 本轮Go1.26仅构建和7新任务+独立观察补充；按最小变化范围，没有重复最低版本全套check/lint/旧PTY。基线最低版本全套PASS是交接的既有证据，本报告不把未重跑项声称为本轮运行。
- 未选定/授权真实供应商、真实模型决策效果、远程SSH、个人bashrc或人工日常试用。ssh仅本地版本；固定项目是Bash，机器状态是合成文件。
- CLI取消发生在读取后模型等待阶段；扫描期间取消、其他文件边界与预算由既有Go测试覆盖。
- 全部Git共享refs/config/hooks保持未修改；临时探针与报告保存在/tmp，构建产物在被忽略bin中。

## 返回开发者的动作

修复请求前缀的路径/赋值误分类，加入路由矩阵并产生新候选，按AGENT.md另派fresh-context测试Agent在新detached工作树复验；本候选不可按本报告合入。报告不授予合并权限。


## 最终独立验收原始报告

# 通用任务固定场景最终独立验收

- 状态：**PASS**（本地模拟程序工具闭环与本次路由修复范围）。
- 测试 Agent：`/root/final_task_acceptance`，全新上下文、独立 detached worktree。
- 基线：`03897097aefdb89c5544231c50316b5b3b2bf597`。
- 候选：`20279d727a42ba6d0210a34516f3e21871ca71f7`。
- 实际 HEAD 开始/结束均为候选，`git status --short --branch` 均为 `## HEAD (no branch)`，无源码/未跟踪修改。结束 `git status --short --ignored` 仅 `!! bin/`；二进制是可删除的忽略生成物，正常 worktree remove 可清理。
- 唯一受测目录：`/home/liang_guang_can/LingCell/.worktrees/verify-task-scenarios-20279d7`。未更改产品代码、提交、合并、refs、配置、hooks 或依赖；独立辅助脚本、覆盖文件、缓存及本报告均在 `/tmp`。
- 环境：Linux `6.6.87.2-microsoft-standard-WSL2`、x86_64/amd64，Go `1.27.2` 与最低 `1.26.0`，Staticcheck `2026.2.1 (0.8.1)`，Python `3.13.5`。本轮本地 HTTP socket/PTY 权限可用，没有权限阻塞。

已独立阅读 AGENT.md、README、Makefile、开发日志、交接、场景文档与基线到候选的完整改动。变更为 9 文件、358 增行/5 删行；依赖未变。Agent 初始上下文注释改为 strings/string slice，与 runtimeContext 的全部字段相符，不改变运行行为。Router 去除 first 含 `/=` 对明确请求前缀的排除，已知名称仍优先，其他路径/赋值/复合结构保留原规则。

## 命令、退出码与证据

以下命令均在受测目录运行。为精确表示环境前缀，使用下列简称（只是报告简称，不是已修改的环境文件）：

```text
E127 = env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/task-final-go127-cache
E126 = env GOTOOLCHAIN=local PATH=/tmp/lingcell-go126/go/bin:/home/liang_guang_can/.local/share/go-dev/tools/bin:/usr/bin:/bin GOCACHE=/tmp/task-final-go126-cache
ESYN = env LINGCELL_API_KEY=SYNTHETIC_TASK_CREDENTIAL_SENTINEL PYTHONDONTWRITEBYTECODE=1
```

| 检查 | 实际命令 | 退出码与观察 | 结论 |
| --- | --- | --- | --- |
| Go 1.27 完整静态/竞态门禁 | `E127 make check` | 0；gofmt、`go mod tidy -diff`、`go vet ./...`、`go test -race ./...`；7 内部测试包通过，命令包无测试 | PASS |
| Go 1.27 有效 Staticcheck | `E127 STATICCHECK_CACHE=/tmp/task-final-staticcheck-cache make lint STATICCHECK='staticcheck -debug.measure-analyzers /tmp/task-final-staticcheck-measurements.txt'` | 0；无诊断；新缓存实际生成 5,456 条计时记录，8 个实际产品包均有分析记录 | PASS |
| Go 1.27 构建 | `E127 make build` | 0；生成候选目录 `bin/lingcell` | PASS |
| Go 1.27 原 PTY 与新场景 | `E127 LINGCELL_API_KEY=SYNTHETIC_TASK_CREDENTIAL_SENTINEL PYTHONDONTWRITEBYTECODE=1 make terminal` | 0；再次构建后旧三会话及新10子场景均通过 | PASS |
| Go 1.27 独立增强审计 | `ESYN python3 /tmp/task-final-extra.py bin/lingcell` | 0；实际7完整确认、6批准1拒绝、精确17条顶层历史；已等待1.8秒让迟到响应到达而不显示；普通Shell矩阵零模型请求 | PASS |
| 凭据隔离负向复现 | `ESYN python3 /tmp/task-final-negative.py bin/lingcell` | 0（预期缺陷被捕获）；仅在临时fixture配置移除 `api_key_env`，mock 立即得到 `AssertionError: mock received an unexpected credential header`，正文未存入 requests | PASS |
| 旧路由缺陷纯Route负向复现 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/usr/bin:/bin GOCACHE=/tmp/task-final-go127-cache go test -overlay=/tmp/task-final-router-overlay.json -run '^TestRequestPunctuation$' -count=1 ./internal/router` | 1（预期失败）；overlay只把router实现替换成从 `eccb70c576af6efca68162aa410e3fc80bcfcb75` 读出的 `/tmp/task-final-router-before.go`。4个新请求均实际返回Shell，未执行输入 | PASS 负向探针 |
| 新路由同矩阵 | `env GOTOOLCHAIN=local PATH=/home/liang_guang_can/.local/share/go/current/bin:/usr/bin:/bin GOCACHE=/tmp/task-final-go127-cache go test -run '^TestRequestPunctuation$' -count=1 ./internal/router` | 0；相同断言在候选实现通过 | PASS |
| Go 1.26 路由局部 | `E126 go test -count=1 ./internal/router` | 0；路由/请求标点/失败政策测试通过 | PASS |
| Go 1.26 构建 | `E126 make build` | 0；`go version -m bin/lingcell` 显示 go1.26.0 | PASS |
| Go 1.26 新任务场景 | `ESYN python3 scripts/verify_tasks.py --binary bin/lingcell` | 0；新10子场景全部通过，28决策/7确认 | PASS |
| Go 1.26 独立增强审计 | `ESYN python3 /tmp/task-final-extra.py bin/lingcell` | 0；同样的完整显示、准确历史、迟到响应、Shell直通与场景次数断言通过 | PASS |
| Python 语法、文档、分析证据 | Python临时只读检查使用 `compile(path.read_bytes(), str(path), 'exec')` 编译 scripts 下2份Python；检查5份本次相关文档的28个本地链接；解析Staticcheck记录，排除测试变体后枚举包 | 0；链接均存在，语法均通过；初次计数助手误把测试变体视为独立产品包而断言失败，修正筛选后确证8个包，未影响lint执行结果 | PASS |
| 格式/工作树 | `git diff 03897097aefdb89c5544231c50316b5b3b2bf597..HEAD --check`，开始/结束 `git rev-parse HEAD`、`git status --short --branch`、结束 `git status --short --ignored` | 各0；候选HEAD不变，无源码修改，仅忽略bin/ | PASS |

Staticcheck实际产品包：`lingcell/cmd/lingcell`、`lingcell/internal/agent`、`lingcell/internal/app`、`lingcell/internal/config`、`lingcell/internal/model`、`lingcell/internal/project`、`lingcell/internal/router`、`lingcell/internal/shell`。原始记录保存在 `/tmp/task-final-staticcheck-measurements.txt`，不是“零包”通过。

## 覆盖结果

| 场景 | 实际决策/确认 | 观察证据 |
| --- | --- | --- |
| 无空格路由1 | 1/0 | `帮我解释PATH=/bin; touch ROUTE_MUST_NOT_RUN` 直接请求Agent，不创建marker |
| 无空格路由2 | 1/0 | `请解释/tmp/file` 直接请求Agent，无classifier与Shell执行 |
| 无空格路由3 | 1/0 | `帮我解释$PATH=/bin` 直接请求Agent，无classifier与Shell执行 |
| 项目理解 | 3/0 | 真实search命中嵌套 `src/nested/service.sh:122`；read_file_range返回119至122行，非截断，最终引用文件/行号 |
| 开发排障 | 4/1 | 实际普通Shell `printf 'expected service_port=9100\n'; false` 退出1；结果保持工具数据。search命中121行9000，read真实片段，再批准真实 Bash语法/grep验证；说明9000/9100不符且尚未修改 |
| 模拟机器排障 | 2/2 | 先批准 `cat machine-status.txt`，工具结果含实际合成状态 `LISTEN port=9000 pid=4242`；处理touch拒绝，前后marker不存在，没有后续请求 |
| 修改 | 6/3 | 编辑确认前完整源码等于原fixture；批准后只9000→9100变化，真实diff返回1与实际删除/新增行；另行批准Bash语法与端口验证返回0/TASK_VERIFY_OK，反馈在同一Agent循环 |
| 验证失败 | 2/1 | 获批固定脚本实际返回7/TASK_FAILURE_EVIDENCE；模型检查数据后报告验证失败与未修复，不出现成功结论 |
| 边界 | 5/0 | `.env.local`、gitignore排除文件、`../outside`、FIFO四个read_file_range均反馈error；敏感哨兵不进入模型或历史 |
| 取消 | 3/0 | search/read完成后模型等待时Ctrl-C；提示取消、普通Shell恢复；额外等1.8秒后再读终端，迟到回答未显示；退出0，termios等于启动前 |

总计实际28次模型决策（原7场景25 + 新3输入各1）、7次生成命令确认（6批准/1拒绝）。独立辅助脚本从实际请求与proposal统计，并逐场景断言精确数量。7次确认的完整命令、实际目录、目的均从PTY显示文本比对提案数据；所有确认均等显示完才回答。

历史通过独立精确列表核对：17条记录逐条等于发送的顶层输入（包括独立Shell矩阵及退出），没有确认回答、生成命令、工具数据或输出。独立Shell矩阵还运行已知 `help` builtin、可执行 `./route-script.sh`、赋值、if复合结构与 `:sh` 强制命令，状态变量保留且模型请求数不增加。候选Go矩阵补充中文路径、普通赋值、已知前缀名称help、别名与续行，纯Route不执行任何测试输入。

旧三真实CLI进程兼容通过：会话1 Shell状态、多行、作业、尺寸、工具反馈、确认/拒绝、模型错误、历史与恢复；会话2模型离线仍可Shell；会话3新Bash隔离、持久状态、多行、作业、模型取消与恢复。实际vim编辑/退出、top显示/退出、ssh本地版本命令通过；远程SSH和个人bashrc明确SKIP。

Go 1.27完整竞态测试同时执行现有项目读取/搜索边界、符号链接、私钥、UTF-8、扫描/匹配/返回预算、取消前与扫描中取消；CLI本批取消范围是文件读取后模型等待，未夸称扫描中CLI取消。

## 缺陷、环境观察与限制

本次已执行检查未发现候选缺陷或未解决阻塞。两项旧FAIL均独立负向复现，当前候选正向通过。

构建元数据观察：Go1.26 `go version -m bin/lingcell` 的 `vcs.revision` 是主工作树基线 `03897097aefdb89c5544231c50316b5b3b2bf597`，尽管受测worktree的HEAD始终为候选。已另核对 `go env GOMOD GOWORK` 指向受测worktree的go.mod且GOWORK为空、`go list -f '{{.Dir}}' ./cmd/lingcell ./internal/router` 均指向候选目录；二进制为该目录构建，旧实现必败的新路由真实CLI场景通过。因此此处保留worktree VCS stamping观察，不用嵌入revision代替真实受测HEAD，也不扩展本次任务修复该Go元数据行为。

未运行：Go1.26完整check/lint和旧三PTY会话重跑（本次最低版本门槛仅router局部、build、新场景；Go1.27完整门禁已执行）；真实供应商/模型决策效果、真实凭据、成功远程SSH、个人bashrc、人工日常试用、root身份与非当前内核环境。全部CLI fixture使用临时HOME与合成配置；父环境凭据检测只设置合成 `LINGCELL_API_KEY` 哨兵，没有读取、打印或调用真实凭据/真实服务。

固定Bash项目和合成机器状态验证的是程序证据传递、确认、工具执行及失败/取消边界；mock最终文本由实际数据驱动，不证明真实模型通用任务质量。原始模型/个人终端数据没有作为报告内容保存。

## 返回协调者

候选可按AGENT.md继续核对main基线、分支与仅证据追加的合入条件。本报告不授予新的合并/发布权限。无需产品修复；请保存本独立PASS证据，确认正常清理测试worktree。报告完成后受测树源码仍干净，仅忽略bin/。
