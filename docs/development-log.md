# 开发日志

记录已完成的工作与实际验证，按日期及主题倒序排列。日期使用 Asia/Shanghai 时区；架构与日常约定见 [AGENT.md](../AGENT.md)。

## 2026-10-11: Agent runtime 会话、上下文及长期记忆升级

### 目标与分批实施

基线 `e83f13e52e4029e819dfae194edf55024f23d807`，开发分支 `feat/agent-runtime`，独立 worktree `.worktrees/agent-runtime`。按用户要求用会话记录替代独立请求；恢复只加载历史，不恢复 Bash 状态或执行授权。

第一批新增 `internal/runtime/store.go`：纯 Go SQLite 固定依赖 `modernc.org/sqlite v1.46.1`，版本化事务迁移、WAL + synchronous FULL、0700/0600 权限、进程独占会话锁，以及有界归档和已知凭据过滤。每轮记录最多 2 MiB、每会话最多 64 MiB，超限拒绝写入。`store_test.go` 覆盖迁移、权限、损坏/链接、写入故障、锁、边界及过滤不会修改执行参数。

第二批新增 `runtime.go` / `context.go`：会话连续性、显式恢复与中断状态；完整记录和模型请求分离，按 `model.EncodeRequest` 计量，默认 128 KiB；分批摘要保留最近两轮，原始归档保留。未完成历史工具调用补齐状态结果且不执行；旧记录工具最多 32 KiB，大工具结果缩减附引用，无法满足预算则停止。开发局部回归覆盖恢复/隔离、工具配对、摘要成功/失败、字节预算和归档上限。

第三批接入 `agent` / `app` / CLI：每轮新预算但沿用 runtime；命令写入开始/结果门禁、普通命令记录、会话/记忆管理、`--resume` / `--ephemeral`；新增 `memory.go` / 回归，含来源校验、版本、FTS5 trigram/短查询、范围隔离、冲突/过期与重新观察证据失效。提取一次有界请求，失败不影响任务。输入历史过滤已知凭据，不修改实际执行参数。维护指南同步解除旧“不保存上下文/无跨会话记忆”边界，新增 [runtime 说明](runtime.md)。

第四批新增 `scripts/verify_runtime.py` 并接入 `make terminal`；既有 PTY 模型 fixture 选择当前轮次、独立处理记忆提取，临时 XDG state 隔离宿主。新增磁盘满事务回滚回归、上下文参数 null 校验、历史调用 ID 的轮内唯一命名；闭合轮次使用已过滤归档继续上下文。PTY 发现过滤误伤 task-specific，修正词边界并补回归。

### 验证状态

开发 SQLite/上下文/记忆/agent/app/config/model 局部测试通过；runtime 新 PTY 四组场景通过（连续/恢复/隔离/管理/未知结果、临时模式、现存记忆隔离/no-history、存储失败）。完整 make check/lint/build/terminal 和最低 Go 1.26 正式验收交独立 Agent，尚未完成。默认缓存及本地监听受限，已使用经批准的 Go 1.27.2 一致工具链、可写缓存和本地模拟服务。后续独立结果补充到 [runtime 交接](handoffs/agent-runtime.md)。

### 未验证项

真实供应商、SSH 与个人 bashrc 未调用。自动记忆属于可能出错的模型判断；凭据过滤不声称能够识别任意秘密文本。

## 2026-10-11: 通用任务固定场景回归

### 目标与改动

第三批 `test/task-scenarios` / 独立worktree 使用 [固定场景](task-scenarios.md) 和 `scripts/verify_tasks.py`，通过本地模型与真实CLI/PTY验证同一Agent的项目理解、开发排障、模拟机器排障、经确认修改代码/差异/验证，另覆盖验证失败、边界和取消，以及3个无空格自然语言前缀路由输入。Makefile terminal 保留三会话验收并追加任务场景；同时修正初始上下文的注释，匹配其字符串与字符串列表字段（该注释无执行行为变化）；补齐 router 无空格自然语言前缀包含路径或赋值示例时的安全路由，保留已知名称、普通脚本/赋值/复合结构与强制指令。每场景输出决策/确认数，不保存私密原始内容。

### 验证状态与限制

开发侧 Python 语法与局部自查通过。首轮候选 `90110036e58b5db3f21e9a3e244d5fbbb4833c79` 因模拟配置遗漏禁用默认凭据变量独立 **FAIL**；已显式关闭 api_key_env，并增加 mock 无 Authorization 断言。次轮 `eccb70c576af6efca68162aa410e3fc80bcfcb75` 因紧凑自然语言前缀被 /= 过滤误路由独立 **FAIL**；已删除过宽过滤并扩展 Go/CLI 回归。两轮候选均未按 FAIL 结论合入，完整原始证据与最终报告见 [任务回归交接](handoffs/task-scenarios.md)。

最终候选 `20279d727a42ba6d0210a34516f3e21871ca71f7` 全新上下文测试 Agent 在固定 detached worktree 独立 **PASS**。Go 1.27.2 的 make check、有效 make lint（8实际包 / 5,456分析记录）、make build、make terminal 均退出0；原三独立CLI会话与新增10子场景通过，共28模型决策 / 7命令确认（6批准、1拒绝）。最低 Go 1.26.0 的 router 局部测试、构建、新10场景及增强审计通过；该版本本批未重跑完整 check/lint/旧三会话。两旧缺陷负向探针有效，当前修复正向通过；独立核对7次完整命令/目录/目的展示、精确17条顶层历史、迟到回答丢弃及普通shell零模型调用。5文档/28本地链接、Python语法与空白检查通过，开始/结束HEAD一致、源码干净，仅忽略bin/。Go1.26构建嵌入VCS revision显示主树基线，报告已记录这一worktree元数据观察；GOMOD、源码目录和新行为CLI证据均来自固定候选。验收后仅追加日志/交接证据，实际no-ff整合以Git历史为准。

任务 fixture 为 Bash 项目与合成机器输出；模拟最终回答不证明真实模型效果，真实供应商、SSH、个人配置、root身份、其他内核环境及人工日常试用未验证。只使用合成父环境凭据哨兵验证隔离，未读取真实值。扫描期间取消由Go回归验证，CLI验证读取后的模型等待取消。

## 2026-10-11: 项目搜索与分段读取

### 目标与改动

第二批 `feat/project-search` / 独立 worktree 同步已验收第一批 main `c5f0bd0`。`internal/project/search_linux.go` 实现受限 Go 搜索/分段读取，复用内核范围、忽略、UTF-8 与私钥检查，共享返回预算及独立文件扫描预算，报告截断/限制/跳过和实际行号。旧读取工具兼容。`internal/agent` 增加 schema、默认值、非法行参数工具反馈与取消终止；提示策略要求先搜索后读取、查询真实系统状态、修改后提出差异/验证、区分执行与验证结果。README / AGENT.md 同步契约。

### 验证状态与限制

开发侧 project/agent 局部测试和格式化完成。首轮候选 `208ece09aa8fe775248976c824f87fcefc0b5f92` 独立验收发现超长原始路径元数据导致裁剪无限循环，未合入；已修复读取前预算检查和循环取消，并新增回归测试。修复候选 `d2a17cb13220f91e21e96730f65a5c1a48bc8b3a` 已由全新上下文测试Agent独立 **PASS**，完整FAIL/PASS证据保存在 [搜索工具交接](handoffs/project-search.md)。Go 1.27.2与1.26.0各自make check、有效make lint（8实际包，分析记录5456/5360）、make build、make terminal均退出0；每版本9项独立附加测试通过，超长路径约180/157µs返回预算错误且零扫描/返回记账；裁剪期间取消及JSON转义/共享预算通过。5文档/25本地链接与空白检查通过，开始/结束HEAD相同、源码干净，仅忽略bin/。验收后仅追加证据，实际no-ff整合由Git历史记录。预算覆盖新工具扫描的文件内容；忽略文件检查沿用旧实现。文本/私钥检查限已扫描前缀，截断结果不可推断未扫描后缀。真实模型、SSH、个人配置及人工试用尚未验证。

## 2026-10-11: 运行上下文与输入路由

### 目标与改动

第一批从 main `4876fba` 创建独立任务分支 `feat/runtime-routing` / worktree。`internal/agent/context_linux.go` 每任务生成有限 Linux 运行信息；不读取环境值或身份详情。`internal/router` 保留已知命令优先，把未知首词的明确自然语言请求放到符号规则之前；仅包含 `$` 或反引号的模糊输入走模型分类。README 与维护指南同步契约，回归矩阵覆盖命令、脚本、赋值、复合结构和含符号的请求。

### 验证状态

开发侧完成格式化和 agent/router 局部测试。候选 `03ea0b500bb5e48f8a138a98e002fda64f25a276` 已由全新上下文测试 Agent 独立验收 **PASS**：[运行上下文与路由报告](handoffs/runtime-routing.md)。Go 1.27.2 下 make check、make lint（8 实际包 / 5,456 分析记录）、make build、make terminal 均退出 0；额外真实 CLI 上下文/环境哨兵/固定根/符号请求/强制指令/分类确认通过，5 文档 / 24 本地链接及空白检查通过。开始/结束 HEAD 相同、源码干净，仅忽略生成物。沙箱监听受限的检查按权限流程重跑成功。真实模型、远程 SSH、个人配置、root 身份、Go 1.26 本批重跑和日常人工试用尚未验证。验收后仅追加证据文档，实际 no-ff 合并以 Git 历史为准。

## 2026-10-11: 三会话终端验收与最低工具链准备

### 目标与改动

第三批可独立完成的本地验收设施在 `test/terminal-daily`、`.worktrees/terminal-daily` 实施，基线 `6ad5b17ba8d93d98b5b74f8bcbcc64d97a6b734d`。第二批仍因缺服务配置与授权阻塞，检查点保存在 `feat/live-model-validation`，未合入。

- PTY 脚本分别报告通过、失败与跳过，缺 vim/top/ssh 不再被笼统 PASS 覆盖，`ssh -V` 与成功远程连接分开。
- 第三个独立 CLI 进程检查新 Bash 状态隔离、持久变量、多行、作业控制、Agent 取消后恢复和退出终端状态；自动会话不能替代用户真实日常试用。
- 新增显式 `--ssh-target` 和 `--bashrc`：前者只使用既有认证/known_hosts，检查远端只读握手、缩放、中断与本地输入归还；后者在临时 HOME 加载指定配置。私密会话错误不打印原始输出，临时目录清理。
- Go 官方发行清单与归档下载到 `/tmp`，Go 1.26.0 Linux/amd64 归档 SHA256 `aac1b08a0fb0c4e0a7c1555beb7b59180b05dfc5a3d62e40e9de90cd42f88235` 校验相符，工具路径 `/tmp/lingcell-go126/go/bin/go`。不替换系统工具链、不更改依赖。

### 验证状态与范围

开发侧 Python 源码编译（不写缓存）和 `git diff --check` 通过。候选 `e960692de1382a1c1d460bfae448fab4a2359afe` 经独立 `terminal_daily_acceptance` Agent 在 detached worktree 验收，本地设施 **PASS**，详见 [交接报告](handoffs/terminal-daily.md)。Go 1.27.2 / 1.26.0 均固定 `GOTOOLCHAIN=local`，各自 `make check`、有效 `make lint`（5,456 / 5,360 条记录、8 实际包）、`make build`、`make terminal` 全部退出 0；三独立 CLI 自动会话、缺工具 SKIP、合成 bashrc/SSH CLI 参数路径与失败脱敏通过。初次本地监听受沙箱限制，按权限流程重跑；测试开始/结束候选相同，源码干净，仅忽略 bin/。真实远程 SSH、个人配置和人工三会话日常试用尚未完成，完整第三批仍 **BLOCKED/未完成**；本地函数 fixture 不证明真实 SSH 网络、认证、raw 模式或远端 PTY。后续仅追加证据文档，合入范围限于已独立通过的本地设施。

## 2026-10-11: 交互阶段、模型错误与离线诊断

### 目标与改动

第一批在 `feat/interaction-doctor`、`.worktrees/interaction` 实施，基线 `ec411712d610ec3de24420649b3826fbf8a49b02`。

- App 检查确认详情的写入和短写错误，失败时在读取确认输入前返回。静态提示覆盖分类、模型决策轮次及 Ctrl-C、文件读取、确认与命令执行；不加工 Bash PTY 输出。
- Agent 增加同步 `Notify(Progress) error` 回调，展示失败阻止对应操作。工具名、配置字段、非流式、逐条确认和无自动重试保持兼容。
- Model 增加可判别错误类型与七类固定提示，保留传输取消/超时身份，不展示远端正文或 URL。
- CLI/App 增加离线 `--doctor`，检查本地依赖、配置、模型完整性、凭据变量存在性及项目读取；输出固定状态标签，无终端要求、网络请求或写入。
- 回归覆盖部分写入/短写、阶段失败前无副作用、错误分类/身份/无重试、诊断脱敏与真实 CLI 错误后的 shell 恢复。

### 验证状态

开发侧 `make fmt`、`git diff --check` 与局部 `go test ./internal/app ./internal/agent ./internal/model` 通过。初次局部测试受沙箱只读 Go 缓存阻止，按权限流程在沙箱外重跑退出 0。候选 `8fb386f2f893731c7bcc91a57787007bf852100e` 经独立 `interaction_acceptance` Agent 在 detached worktree 验收 **PASS**，见 [交接报告](handoffs/interaction-doctor.md)。`make check`、`make lint`（fresh cache，5,456 条记录、8 个实际包）、`make build`、`make terminal` 及独立非终端 doctor 七场景全部退出 0。测试开始/结束 SHA 一致、源码干净，仅忽略 bin/；后续仅追加证据。doctor 通过固定脱敏标签、目录内容/元数据快照和本地 endpoint 零请求验证，未执行 syscall trace。

### 未验证项

真实供应商/凭据使用授权、成功的远程 SSH、个人 `.bashrc` 以及最低 Go 1.26 工具链不属于第一批已验证行为；后续批次分别记录。没有请求真实模型或修改用户配置。

## 2026-10-10: Go 注释规范与契约说明

### 目标与改动

按用户要求采用 Google Go Style Guide 和 Go 官方 Doc Comments 的惯例，为现有代码补充准确的注释。在 `docs/go-comments` 分支、`.worktrees/go-comments` 开发；基线为主分支合并提交 `18f99b4`。

- 8 个 Go 实现文件补充命令包说明、路由常量及接口/字段契约，解释配置默认值、请求限制、数据与指令边界、命令确认和取消、资源所有权及并发条件。
- `internal/shell` 解释独立管道的 NUL 帧、旧提示符状态保留、非阻塞描述符、输出捕获和输入恢复；`internal/project` 解释固定范围、忽略规则、符号链接检查和字节预算。
- 7 个现有测试文件补充被验证条件和模拟依赖说明；修正作业测试关于 Ctrl-Z 注入方式的描述。未新增测试行为或修改执行逻辑。
- `AGENT.md` 保存注释语言、Go doc 格式、解释意图、同步维护和仅注释变更的验证要求；[任务交接](handoffs/go-comments.md) 规定独立验收条件。

### 验证状态

开发侧执行 `make fmt` 和 `git diff --check`，完成格式化且空白检查通过。候选 `d85a0dc74c01c06edf4ad3ab40568e9694ed20db` 已由新的 `go_comments_acceptance` 测试 Agent 在 detached 工作树独立验收，状态 **PASS**，报告摘要保存于 [任务交接](handoffs/go-comments.md)。

- Linux/amd64、Go 1.27.2、Staticcheck 2026.2.1 环境下，全部 15 个 Go 文件共 18,423 token（含字面量和自动分号）、构建/编译/生成指令一致；非 Go 执行文件、依赖和配置未变。注释逐包审查未发现缺陷。
- `make check`、有效运行的 `make lint`、`make build` 退出 0；7 个内部包竞态测试通过，8 包 Go doc 可读，11 份 Markdown / 27 个本地链接和差异空白检查通过。
- 初次 Staticcheck 在沙箱缓存只读时提示零包，未作为通过证据；按工具权限流程重跑，再用显式 8 包和独立临时缓存验证，实际分析证据为 5,456 条计时记录，无诊断。
- 测试开始/结束 HEAD 相同且源码干净，仅生成忽略的 bin/；无缺陷或阻塞。受测候选之后的提交只追加日志/交接证据；实际 no-ff 合并和工作树清理以 Git 历史及最终状态记录。

### 未验证项

本次只调整 Go 注释和维护文档，没有选择真实模型供应商、配置凭据、验证远程 SSH 或发布。仅注释任务不要求重跑完整 CLI PTY 验收，独立测试仍需证明执行 token 未变并运行既有 Go 会话测试；最低 Go 1.26 工具链没有单独验证。

## 2026-10-10: 开发/测试 Agent 分离与 Git 工作树流程

### 目标与改动

根据用户要求，建立后续 Agent 必须执行的独立测试、交接、提交及分支/worktree 规范。

- 保留原有历史，将默认分支 master 重命名为 main；本任务在 docs/agent-workflow 分支和独立 .worktrees/agent-workflow 开发。
- AGENT.md 作为规范源，要求正式验收派生无开发对话继承的测试 Agent，固定候选提交、独立 detached 工作树、最小交接、复现反馈和改动后重验；明确这是维护分工，产品仍保持单 Agent MVP。
- 规定有意义的代码、测试、配置和文档变动必须提交，使用短期任务分支和 no-ff 整合，禁止主分支直接开发；补充 WIP、基线变化、回滚、工作树清理及远程保护边界。
- 增加 docs/workflow.md 操作手册、双向交接模板、CONTRIBUTING.md 入口及本地提交 hooks；README、AGENTS.md 链接到强制流程，忽略 .worktrees。
- 实际开发交接见 [agent-workflow 报告](handoffs/agent-workflow.md)，基线为先前文档提交 948a7a0；候选在提交后交给独立测试 Agent。

### 验证状态

首个候选 5e1757a 已由干净上下文测试 Agent 在 detached 工作树验收。hooks 正反行为、链接和身份检查通过，但回滚说明导致 FAIL：限制提交的 hooks 不覆盖直接 revert 的所有自动提交途径，且反向改动需检查暂存区。

修订 AGENT.md 与操作手册，统一回滚的 --no-commit / 暂存差异审查 / revert: 提交流程，补充本地 hooks 的真实边界。修订候选将另派干净上下文测试 Agent，结果再追加。当前只改变维护文档、Git hooks 与忽略规则，没有修改 Go 产品代码。

后续复测：候选 `42c2c384dba1f72a370c2933878de2416c07c6b2` 由新的 workflow_retest 独立 Agent 在另一 detached 工作树验收，状态 PASS。hooks 正反矩阵、规范 no-ff 合并、普通及合并回滚、身份/文件完整性通过；10 份 Markdown、25 个本地链接和空白检查通过。测试记录 169 条正式命令及结果，真实源码、refs/config 未修改，没有缺陷或阻塞。

报告摘要追加到 [任务交接](handoffs/agent-workflow.md)，后续提交仅保存验收证据。本轮没有运行 Go/PTY 整套验收；整合前协调者将检查差异只有证据、main 基线未变且两侧工作树干净，通过规范的任务合并保存边界。实际合并与工作树清理结果以 Git 历史和最终工作树状态为准，不在文档中预写该次提交自身 SHA。

### 未验证与后续事项

远程仓库、服务器分支保护、PR 状态检查尚未配置；本次只落地本地开发流程。本仓库已设置 core.hooksPath=.githooks，主工作树更新 hooks 文件后生效；实际整合与工作树清理在验收通过后执行，并记录结果。

## 2026-10-10: Go 规范检查与维护流程

### 目标与改动

检查现有 Go 应用的开发惯例，修正维护问题，并让后续开发有文档、统一验证命令与可审查的 Git 提交。

- 补充导出接口和类型注释，说明终端会话、项目读取句柄、Agent 的使用和并发约束；统一格式及导入分组。
- `internal/model` 用可解包的请求错误保留取消/超时身份，终端仍显示简洁且不含敏感详情的消息。
- `internal/config` 显式限制配置体积为 64 KiB，拒绝 `null` 等非对象以及被限制流截断的超大配置。
- `internal/project` 让根目录句柄关闭支持重复调用；`internal/shell` 检查非阻塞设置，初始化失败后清理进程与句柄。
- `internal/agent` 检查文本输出、工具参数解码和结果编码错误；无法显示模型提案时停止，不进入命令确认/执行。
- `internal/app` 检查历史文件初始化关闭错误。新增相关回归测试，覆盖配置边界、超时原因、重复关闭和输出失败。
- 扩展 `Makefile`，增加 `fmt`、`fmt-check`、`mod-check`、`test-race`、`vet`、`lint`；基础检查统一为 `make check`，终端检查为 `make terminal`。
- 新建 [AGENT.md](../AGENT.md) 维护指南与标准发现入口 [AGENTS.md](../AGENTS.md)、[Go 检查记录](go-review.md) 及本日志；README 添加入口和最新验证命令。
- Git 以 MVP 提交 `bb4df55` 为基线，代码维护与文档分别形成提交；增加 Python 缓存忽略项，不提交二进制或真实凭据。

### 验证

在 Linux/amd64、Go 1.27.2、Staticcheck 2026.2.1 下执行：

- `make fmt`：完成格式化。
- `make check`：通过格式与 `go mod tidy -diff` 检查、`go vet ./...` 和 `go test -race ./...`，7 个实现包测试通过。
- `make lint`：通过，无报告。
- `make terminal`：CLI 构建与本地模拟服务/真实 PTY 验收通过，包含状态、输入、信号、作业、窗口、Agent 确认/拒绝、诊断、取消、历史及恢复流程。
- `git diff --check`：通过；检查 5 份 Markdown 文档的相对链接与空白格式，没有发现问题。

以上应用检查针对代码改动执行；后续文档修改检查链接和差异格式。依赖未升级，最低 Go 1.26.0 工具链未单独验证。

### 未验证与后续事项

1. 用户选定供应商后，验证真实分类、工具调用和结果反馈；当前配置仅为模板，没有真实凭据或供应商请求。
2. 在用户选择的测试主机上联调成功的 SSH 交互；本地脚本仅检查 `ssh -V`。
3. 确定远程仓库及远程安装需求后调整 module/import 路径，并按需求配置 Linux CI。当前没有远程仓库推送、发布或部署。

## 2026-10-10: Linux Agent Shell MVP 基线

### 已实现

本地 Git 基线：`bb4df55 feat: implement Linux agent shell MVP`。

CLI 使用持久 Bash + PTY；Go 负责编辑、补全和路由，命令及完成事件使用独立管道。普通 shell 命令保留工作目录、环境、别名、函数和作业；自然语言任务使用兼容 Chat Completions 的有界工具循环，所有模型提案的命令逐条确认。

项目读取范围固定到启动时最近 Git 根，使用 Linux `openat2` 保证打开范围，遵循敏感路径排除与 `.gitignore`，限制读取及输出预算。失败命令的诊断结果作为工具数据传入，拒绝后不执行修复；输入历史与模型上下文分离。

### 主要文件职责

| 文件或目录 | 职责 |
| --- | --- |
| `cmd/lingcell/main.go` | CLI 配置、信号取消和终端恢复。 |
| `internal/app`、`internal/router` | 输入编辑与补全、路由、续行、失败接管及确认。 |
| `internal/shell` | Bash 生命周期、PTY、状态管道、捕获输出及前台任务。 |
| `internal/model`、`internal/agent` | HTTP 模型协议、工具参数、决策循环和执行确认。 |
| `internal/project`、`internal/config` | 项目读取边界/预算及配置默认值/合法性校验。 |
| `scripts/verify_terminal.py` | 临时 HOME、本地模拟模型与真实 CLI 交互验收。 |
| `README.md`、`config.example.json` | 构建运行说明与无凭据配置模板。 |

本日志建立时重新阅读了实现、测试、脚本及 Git 差异。维护修改后的统一测试结果见上方条目；真实供应商、远程 SSH 和远程发布仍属于未验证事项。
