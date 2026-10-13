# Agent skills 开发 → 独立测试交接

- 任务/需求：给 LingCell Agent 加上发现并读取本地 skills 的能力。
- 职责：开发 Agent 实现代码/回归/文档并提交；独立测试 Agent 自行审查与验收，不修改产品代码、不提交/合并或修改共享配置/refs。
- 项目根：`/home/liang_guang_can/LingCell`。
- 开发 worktree / 分支：`/home/liang_guang_can/LingCell/.worktrees/agent-skills` / `feat/agent-skills`。
- 基线：main，`ef5acba95a5a728e82b62437d6251a652c93899d`。
- 最终受验候选：`73c403606aeba6b73cb750e994d63c935e2de5c4`；detached测试目录：`/home/liang_guang_can/LingCell/.worktrees/verify-agent-skills-r3`（验收完成后清理）。
- 改动范围：internal/skills（技能库/文档/预算）、internal/project（精确范围和元数据检查）、internal/agent（工具/提示/校验）、internal/app（按任务生命周期接入）、internal/config（skill_dirs）、相关测试、scripts/verify_tasks.py、scripts/verify_terminal.py（辅助函数参数名与XDG隔离）、README/AGENT/docs/skills.md。
- 验收条件：
  1. list_skills 能发现默认 project/user 和显式 extra-N 库；同名按来源区分；仅含普通 SKILL.md 的一级目录返回，不自动上传正文，缺失库允许。
  2. read_skill 默认读取 SKILL.md，支持技能相对参考文件；外部库精确范围不因 Git 祖先扩张；固定项目路径不因 Bash cd 改变。
  3. 拒绝绝对/遍历、符号链接（含根）、敏感/忽略/私钥/非文本/特殊文件；遵守项目各层忽略规则；取消终止工具。
  4. 完整技能 JSON 单次32 KiB，累计与项目工具共享256 KiB；目录限制及不可用库可见。配置目录为绝对路径、最多16个。
  5. Agent 只在配置 Skills 时广告工具，参数严格校验，读取返回工具数据；技能不绕过逐条命令确认，拒绝不执行。
  6. 真实 CLI/本地模拟模型完成发现→正文→参考文件，既有 Agent/PTY/runtime 行为回归通过；文档符合实现。
- 建议命令：相关 go test；make check；make lint；make build；make terminal；git diff --check。环境提供 Go 与 staticcheck，不升级依赖。
- 约束：不读取真实凭据/个人技能内容，不联网请求真实模型/SSH、不修改产品/测试代码、依赖或规范、不提交/合并。独立探针可放临时目录并使用 Go overlay。
- 已知未验证：真实供应商工具决策、真实SSH/个人配置；默认模拟测试不代表真实模型技能选择效果。最低 Go1.26 可用时追加，未运行须注明。
- 第三轮必须复测：首轮三个缺陷以及第二轮空技能库取消遗漏，复现见下方两轮报告。新候选SHA与detached目录通过协调消息提供。

开发自查不作为验收结论。

第二轮修复：read_skill 使用 RawMessage 区分省略/null并补参数回归；MockModel.tool 工具名形参改为 tool_name；Terminal 固定临时 XDG_CONFIG_HOME。开发局部 Go 测试、构建与通用任务 PTY 自查通过；第二轮独立验收待执行。


# Agent skills 首轮独立验收报告

- 状态：**FAIL**（三项可复现缺陷；本候选不应合入）。
- 测试 Agent：`/root/skills_acceptance`，最小交接、独立 detached worktree，自行读取 AGENTS.md / AGENT.md、README、Makefile、开发日志、交接及差异。
- 基线完整 SHA：`ef5acba95a5a728e82b62437d6251a652c93899d`。
- 候选完整 SHA：`07c9f364a747a273e81f6985f3d481f5a10989e9`。
- 实际 HEAD（开始/结束）：均为 `07c9f364a747a273e81f6985f3d481f5a10989e9`。
- 所有仓库命令工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-agent-skills-07c9f36`。
- 状态：开始 `## HEAD (no branch)`、无修改；结束源码仍干净，`git diff --exit-code` 退出0；仅忽略生成物 `bin/` 与 `scripts/__pycache__/`。独立探针及报告位于 `/tmp/lingcell-skills-probes-7h8qjiz2`，Go overlay 接入；未修改产品、仓库测试、规范、依赖，未提交/合并/修改共享 refs/config。
- 环境：Linux `6.6.87.2-microsoft-standard-WSL2` x86_64；Go `go1.27.2 linux/amd64`；Staticcheck `2026.2.1 (0.8.1)`；Python `3.13.5`。`go list ./...` 有命令包及9个实际内部包。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 相关回归 | `go test -count=1 -v ./internal/skills ./internal/agent ./internal/config ./internal/project` | 0；四包全过，覆盖默认/额外库、正文/引用、边界/预算/取消、工具角色和拒绝不执行 | PASS |
| 格式/模块/vet/race | `make check` | 0；tidy -diff、vet、9个内部包race全通过；命令包无测试 | PASS |
| Staticcheck | `make lint` | 0；`staticcheck ./...` 无诊断 | PASS |
| 构建 | `make build` | 0；`go build -o bin/lingcell ./cmd/lingcell` | PASS |
| 真实CLI技能闭环 | 空临时XDG_CONFIG_HOME包装 `make terminal`（确切命令见下） | 2；verify_terminal三会话、vim/top/ssh本地版本通过；verify_tasks路由3场景/项目理解通过，skills第二决策参数名冲突失败，脚本退出1 | FAIL |
| 来源/一级发现/精确范围 | overlay命令（见下）`TestAcceptanceCatalogAndExactScope` | PASS；project/user/extra-1同名分开，不上传正文；无SKILL、非法名称、嵌套、目录型SKILL排除；Git祖先范围外技能不能读 | PASS |
| 目录限制 | overlay `TestAcceptanceCatalogLimits` | PASS；257技能返回256，JSON10525字节，truncated=true；4100普通条目返回0，JSON30字节，truncated=true | PASS |
| 项目各层忽略 | overlay `TestAcceptanceAncestorIgnoreRules` | PASS；根、.lingcell、skills库、技能自身四层gitignore均阻止reference，正文仍能读 | PASS |
| 根/内部链接、库不可用 | overlay `TestAcceptanceSymlinkAndUnavailableRoots` | PASS；根链接、祖先链接、普通文件根产生3个可见issues；缺失库跳过、内部链接拒绝 | PASS |
| 完整JSON及双向共享预算 | overlay `TestAcceptanceJSONBudgetBothDirections` | PASS；转义技能完整JSON32764字节、内容5447字节、truncated=true；精确剩余预算用完后技能目录及项目工具均报预算错误 | PASS |
| cd不扩张项目 | overlay `TestAcceptanceFixedProjectAfterCwdChange` | PASS；改变cwd到另一合成项目后仍返回启动固定项目正文 | PASS |
| 16目录配置上限 | overlay `TestAcceptanceSkillDirectoryConfigCap` | PASS；16个有效、17个拒绝；既有config回归覆盖相对/null元素/NUL路径 | PASS |
| 严格参数 | overlay `TestAcceptanceExplicitNullSkillPath` | FAIL；validate接受`{"source":"project","name":"review","path":null}`，与schema字符串类型不一致 | FAIL |
| 广告/命令确认 | 相关回归`TestSkillArgumentValidation`、`TestSkillGuidanceCannotAuthorizeCommands`及源码审查 | 0；未配置Skills不广告/不接受工具；未知字段/非法名称终止；恶意正文以role=tool反馈，一次确认拒绝、零命令执行 | PASS（null例外见上） |
| runtime回归 | 临时XDG包装 `python3 scripts/verify_runtime.py --binary bin/lingcell` | 0；连续轮次、恢复/Bash重置、锁/根、隔离记忆、历史工具、拒绝/未知结果不重放、ephemeral/no-history/存储故障保留shell通过 | PASS |
| 测试环境隔离 | 外部Python诊断harness设置临时XDG，写唯一合成user技能后调用`verify_tasks.verify()` | 1；实际catalog含project/review和user/synthetic-user，only-project断言失败，未读取真实个人技能 | FAIL |
| 文档/空白 | `git diff --check ef5acba95a5a728e82b62437d6251a652c93899d 07c9f364a747a273e81f6985f3d481f5a10989e9`；Python本地链接检查 | 0；5份改动Markdown、36个本地链接存在；主要边界与实现一致 | PASS |
| 结束HEAD/状态 | `git rev-parse HEAD`、`git status --short --branch`、`git status --short --ignored`、`git diff --exit-code`、`git diff --check` | 0；HEAD不变，源码干净，仅上述忽略生成物 | PASS |

overlay确切命令（整体退出1，8项顶层独立探针7 PASS/1 FAIL）：

```bash
go test -count=1 -race -v -overlay=/tmp/lingcell-skills-probes-7h8qjiz2/overlay.json ./internal/skills ./internal/config ./internal/agent -run TestAcceptance
```

make terminal确切命令（退出2）：

```bash
python3 - <<'PY'
import os, subprocess, tempfile
with tempfile.TemporaryDirectory(prefix='lingcell-skills-xdg-') as task_xdg:
    raise SystemExit(subprocess.call(['make', 'terminal'], env=dict(os.environ, XDG_CONFIG_HOME=task_xdg)))
PY
```

runtime确切命令（退出0）：

```bash
python3 - <<'PY'
import os,subprocess,tempfile
with tempfile.TemporaryDirectory(prefix='lingcell-runtime-xdg-') as task_xdg:
    raise SystemExit(subprocess.call(['python3','scripts/verify_runtime.py','--binary','bin/lingcell'],env=dict(os.environ,XDG_CONFIG_HOME=task_xdg)))
PY
```

## 缺陷与复现

1. **高，阻止合入：新增真实CLI技能验收无法执行到正文。** `scripts/verify_tasks.py:101,104`、`scripts/verify_terminal.py:90`。运行上述空临时XDG的make terminal；发现实际正确返回`{"skills":[{"source":"project","name":"review"}],"truncated":false}`；随后`self.tool("skills-read", "read_skill", source="project", name="review")`与基类`tool(id_, name, **arguments)`的name形参冲突。外部诊断harness独立读取模拟模型错误：`TypeError: MockModel.tool() got multiple values for argument 'name'`。期望完成发现→正文→引用，实际模拟服务500、TASK_SKILLS标记缺失。建议工具名形参改为tool_name并覆盖引用分支。
2. **中，阻止合入：read_skill接受path:null，未严格执行参数契约。** `internal/agent/agent.go:183-194`。overlay直接传`{"source":"project","name":"review","path":null}`给validate，实际返回nil错误。期望按schema拒绝显式null（文档仅省略/空串默认SKILL.md），实际null变nil指针、当作省略。这不绕过命令确认，但违反交接严格参数条件。建议区分省略/null并补回归。
3. **中，阻止合入：终端测试继承宿主XDG_CONFIG_HOME，新增技能发现不隔离。** `scripts/verify_terminal.py:101`、`scripts/verify_tasks.py:99`。设置临时XDG_CONFIG_HOME，写入`lingcell/skills/synthetic-user/SKILL.md`仅含合成文本，再启动原任务脚本，退出1。外部harness确认实际catalog为`{"skills":[{"source":"project","name":"review"},{"source":"user","name":"synthetic-user"}],"truncated":false}`，断言错误`AssertionError`。期望测试只使用合成库、与宿主无关；实际Terminal继承XDG并枚举用户库、only-project断言失败；真实自定义XDG环境下会自动读取个人技能名称。建议Terminal明确设置临时root下XDG_CONFIG_HOME。

## 未运行项与限制

- Go1.26工具链未找到，未下载；Go1.27.2通过不能替代最低版本验证。此项为交接可选检查。
- 真实供应商模型决策/技能选择、远程SSH、个人bashrc/技能/真实凭据未授权、未读取、未联调；本地ssh -V不代表远程SSH通过。
- make terminal在skills场景提前失败；后续通用任务场景未完成本轮验收，runtime已单独通过。真实CLI发现→正文→引用本轮FAIL；单元测试不能替代。
- 未进行持续恶意文件系统竞争压力测试；内容检查保持文档所述读取前缀边界，不能推断后缀安全。
- 首次带rm -rf清理的终端包装被自动审批拒绝，未执行；改用Python TemporaryDirectory清理后正式验收已执行，不是未解阻塞。
- 报告写入初次Python heredoc因嵌套终止符发生语法错误，未生成报告，误触发另一次隔离runtime运行（通过）；已改用apply_patch写临时报告，无仓库修改。

## 返回开发者的动作

修复以上三项，补必要回归，在任务分支提交新候选，再派生全新上下文测试Agent于新候选detached worktree验收。旧报告不能适用于新提交，报告不授予合并权限。


# Agent skills 第二轮独立验收报告

- 状态：**FAIL**。首轮三项缺陷均已复测通过；新增“无技能库时忽略取消”违反明确验收条件，候选不能合入。
- 测试 Agent：`/root/skills_final_acceptance`，干净上下文、最小交接、独立 detached worktree；自行读取仓库说明、交接、差异及首轮原始探针后执行。
- 基线完整 SHA：`ef5acba95a5a728e82b62437d6251a652c93899d`。
- 候选完整 SHA：`618b382591917a51f3424228e04655e7c5a78832`。
- 实际 HEAD（开始/结束）：均为 `618b382591917a51f3424228e04655e7c5a78832`。
- 所有仓库命令工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-agent-skills-r2`。
- 工作树状态：开始/结束均为 `## HEAD (no branch)`、无源码修改；结束 `git diff --exit-code` 为0，仅忽略生成物 `bin/`、`scripts/__pycache__/`。未修改产品、仓库测试、规范、依赖；未提交、合并或修改共享 refs/config。
- 环境：Linux `6.6.87.2-microsoft-standard-WSL2` x86_64；Go `go1.27.2 linux/amd64`；Staticcheck `2026.2.1 (0.8.1)`，路径 `/home/liang_guang_can/.local/share/go-dev/tools/bin/staticcheck`；Python `3.13.5`。`go list ./...` 实际列出命令包和9个内部包。
- 原始日志、报告及新 overlay：`/tmp/lingcell-skills-r2-probes-24glib6f`。首轮临时探针经独立审查复制至新临时目录，overlay 只接入虚拟测试文件，没有改候选。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 相关Go回归 | `go test -count=1 -v ./internal/skills ./internal/agent ./internal/config ./internal/project` | 0；四包全部通过，原始日志 `related.log` | PASS |
| 格式/模块/vet/race | `make check` | 0；gofmt检查、tidy -diff、vet、9个内部包race通过，命令包无测试；`check.log` | PASS |
| Staticcheck | `make lint` | 0；实际执行 `staticcheck ./...`，无诊断；`lint.log` | PASS |
| 构建 | `make build` | 0；`go build -o bin/lingcell ./cmd/lingcell`；`build.log` | PASS |
| 原首轮8项独立探针复测 | 下方overlay命令 | 0；8个顶层探针全过；`overlay.log` | PASS |
| 默认/额外来源、一级发现及精确范围 | overlay `TestAcceptanceCatalogAndExactScope` | 同名project/user/extra-1分开，正文未上传，无SKILL/非法名称/嵌套/目录型SKILL排除，Git祖先外技能不能读 | PASS |
| 目录枚举/返回限制 | overlay `TestAcceptanceCatalogLimits` | 257技能返回256，JSON10525字节；4100普通条目返回0，JSON30字节；均truncated=true | PASS |
| 四层项目忽略规则 | overlay `TestAcceptanceAncestorIgnoreRules` | 原项目根、.lingcell、技能库、技能目录四层忽略规则均阻止reference，正文仍能读 | PASS |
| 根/内部链接及不可用库 | overlay `TestAcceptanceSymlinkAndUnavailableRoots` | 根链接、祖先链接、普通文件根产生3个issues，缺失库跳过，内部链接拒绝 | PASS |
| 技能完整JSON/共享预算 | overlay `TestAcceptanceJSONBudgetBothDirections` | 转义技能完整JSON32764字节/内容5447字节，truncated=true；精确累计预算用尽后技能目录及项目读取均报错 | PASS |
| Bash cd不改变项目 | overlay `TestAcceptanceFixedProjectAfterCwdChange` | 改到另一合成项目后仍读取启动固定项目正文 | PASS |
| 配置目录上限/路径 | overlay `TestAcceptanceSkillDirectoryConfigCap`及config回归 | 16个有效，17个拒绝；回归另覆盖相对路径、null元素、NUL路径拒绝 | PASS |
| 首轮path:null缺陷 | overlay `TestAcceptanceExplicitNullSkillPath`及agent回归 | 0；显式null拒绝，省略/空字符串/参考路径仍合法 | PASS |
| 技能边界/工具角色/命令确认 | 相关回归及源码审查 | 遍历/绝对/敏感/私钥/NUL/忽略/链接/FIFO拒绝；Skills未配置不广告且调用拒绝；恶意正文以role=tool反馈，确认拒绝后零命令执行 | PASS |
| 首轮name冲突、真实CLI技能闭环 | 合成宿主XDG下 `make terminal`，确切包装见下 | 0；skills四决策/零确认完成发现→正文→references/check.md，正文/引用内容断言通过 | PASS |
| 首轮宿主XDG污染 | 同一terminal包装，宿主配置只含synthetic-user合成技能 | 0；only-project技能目录断言通过，合成宿主文件不变，Terminal固定临时.config；无真实个人技能读取 | PASS |
| 完整PTY/通用任务/runtime | 同一 `make terminal` | 0；三CLI会话、vim/top/本地ssh -V；路由/理解/诊断/机器拒绝/实际修改差异验证/失败/边界/取消；runtime连续轮次恢复/隔离记忆/不重放/临时模式/存储故障全部通过；`terminal.log` | PASS |
| 无技能库取消 | 新独立overlay `TestAcceptanceEmptyLibraryCancellation`，确切命令见下 | 1；已取消ctx返回成功空目录及nil错误；`empty-cancel.log` | FAIL |
| 文档/空白 | `git diff --check ef5acba95a5a728e82b62437d6251a652c93899d 618b382591917a51f3424228e04655e7c5a78832`；本地Python Markdown链接检查 | 0；5份改动Markdown共36个本地链接存在；主要边界与实现一致 | PASS |
| 结束HEAD/状态 | `git rev-parse HEAD`、`git status --short --branch`、`git status --short --ignored`、`git diff --exit-code`、`git diff --check` | 0；候选不变，源码干净，只有上述忽略生成物 | PASS |

首轮8探针复测确切命令（运行时尚未追加第9项空库取消探针，退出0）：

```bash
go test -count=1 -race -v -overlay=/tmp/lingcell-skills-r2-probes-24glib6f/overlay.json ./internal/skills ./internal/config ./internal/agent -run TestAcceptance
```

新增空库取消探针确切命令（退出1）：

```bash
go test -count=1 -race -v -overlay=/tmp/lingcell-skills-r2-probes-24glib6f/overlay.json ./internal/skills -run TestAcceptanceEmptyLibraryCancellation
```

完整terminal确切命令（退出0；日志保存至临时目录）：

```bash
python3 - <<'PY'
import os,subprocess,tempfile
from pathlib import Path
with tempfile.TemporaryDirectory(prefix='lingcell-r2-host-xdg-') as task_xdg:
    skill=Path(task_xdg)/'lingcell/skills/synthetic-user/SKILL.md'
    skill.parent.mkdir(parents=True)
    skill.write_text('SYNTHETIC_ONLY_HOST_GUIDANCE')
    with open('/tmp/lingcell-skills-r2-probes-24glib6f/terminal.log','w') as log:
        code=subprocess.call(['make','terminal'],env=dict(os.environ,XDG_CONFIG_HOME=task_xdg),stdout=log,stderr=subprocess.STDOUT)
    assert skill.read_text()=='SYNTHETIC_ONLY_HOST_GUIDANCE'
    print('make terminal exit',code,'synthetic host fixture unchanged')
    raise SystemExit(code)
PY
```

## 缺陷与复现

1. **低，阻止当前候选合入：无技能库时list_skills忽略取消。** 位置 `internal/skills/skills.go:116-121`（context检查仅位于libraries循环），最终返回处继续正常ChargeResult。交接验收条件3明确要求“取消终止工具”。默认project/user库都不存在是支持且常见的状态；此时libraries为空，已取消context绕过所有检查。

   最小复现（临时overlay，使用仓库现有setup创建合成固定项目及空XDG）：

   ```go
   func TestAcceptanceEmptyLibraryCancellation(t *testing.T) {
       root, budget := setup(t)
       task := NewTask(root, nil, budget)
       defer task.Close()
       ctx, cancel := context.WithCancel(context.Background())
       cancel()
       _, err := task.List(ctx)
       if err != context.Canceled {
           t.Fatalf("empty skill catalog with canceled context: got %v, want context.Canceled", err)
       }
   }
   ```

   期望：返回context.Canceled，不将取消当作成功工具结果。实际输出：`empty skill catalog with canceled context: got <nil>, want context.Canceled`，测试退出1。影响有限：这是取消契约遗漏，不存在命令执行授权绕过；Agent循环自身仍有context检查，但不能代替工具契约。建议List入口及返回前检查context，并补无库取消回归。

首轮工具辅助函数name冲突、显式path:null接受、宿主XDG污染三项已修复，独立复测均通过。

## 未运行项与限制

- Go1.26工具链不可用，未下载；Go1.27.2通过不等于最低版本验证，此项为交接可选检查。
- 未调用真实供应商模型或远程SSH，未读取真实凭据/个人skills/bashrc。脚本明确SKIP远程SSH及个人bashrc；本地ssh -V只验证本地版本命令。
- 未进行持续恶意文件系统竞争压力测试；文件内容检查只覆盖文档规定的前缀，不保证未读后缀安全。
- 代码读取时一次复合命令含不存在的`internal/project/query_linux.go`，该阅读命令退出2；随后按实际路径重新审查并单独运行diff空白检查（退出0）。此不是验收工具阻塞或产品失败。
- 未重复执行追加空库探针后的整个overlay集合；原8项有独立通过日志，新第9项单独失败日志已足够证明FAIL。协调者要求不再扩大本轮覆盖，修复后另派干净上下文验收新SHA。

## 返回开发者的动作

修复List空库取消检查并补必要回归，任务分支提交新候选；新候选需全新独立测试Agent及detached worktree验收。本报告不适用于后续代码SHA，不授予合并权限。


第三轮修复：List入口及所有技能结果返回计费前检查取消，新增空技能库取消回归。开发skills/agent局部测试通过；新候选重新独立验收。


# Agent skills 第三轮独立验收报告

- 状态：**PASS**。两轮共四项缺陷均独立复测通过，本次已执行检查未发现缺陷或未解决的验收阻塞。
- 测试 Agent：`/root/skills_r3_acceptance`，全新上下文、最小交接；自行读取 AGENTS.md / AGENT.md、README、Makefile、开发日志、交接报告、源码差异及此前独立探针。
- 基线完整 SHA：`ef5acba95a5a728e82b62437d6251a652c93899d`。
- 候选完整 SHA：`73c403606aeba6b73cb750e994d63c935e2de5c4`。
- 实际 HEAD（开始/结束）：均为 `73c403606aeba6b73cb750e994d63c935e2de5c4`。
- 所有仓库命令工作目录：`/home/liang_guang_can/LingCell/.worktrees/verify-agent-skills-r3`，detached worktree。
- 工作树状态：开始/结束 `## HEAD (no branch)`、无源码修改；结束 `git diff --exit-code` / `git diff --check` 退出0，仅忽略生成物 `bin/`、`scripts/__pycache__/`。未修改候选产品、仓库测试、规范或依赖；未提交、合并、修改共享 refs/config。报告与 overlay 均位于临时目录。
- 环境：Linux `6.6.87.2-microsoft-standard-WSL2` x86_64；Go `go1.27.2 linux/amd64`，`/home/liang_guang_can/.local/share/go/current/bin/go`；Staticcheck `2026.2.1 (0.8.1)`，`/home/liang_guang_can/.local/share/go-dev/tools/bin/staticcheck`；Python `3.13.5`。`go list ./...` 实际列出命令包和9个内部包。
- 原始日志、overlay、命令退出码及报告：`/tmp/lingcell-skills-r3-probes-bbkyskh5`。此前九项探针经逐项审查后复制到新临时目录，overlay 使用当前 worktree 的虚拟测试路径，本轮重新运行所有九项，未沿用旧结果。

| 验收条件/检查 | 命令或步骤 | 退出码/观察结果 | 判定 |
| --- | --- | --- | --- |
| 相关Go回归 | `go test -count=1 -v ./internal/skills ./internal/agent ./internal/config ./internal/project` | 0；四包通过，含新增无库取消回归；`related.log` | PASS |
| 格式/模块/vet/race | `make check` | 0；格式检查、tidy -diff、vet和9个内部包race通过，命令包无测试；`check.log` | PASS |
| Staticcheck | `make lint` | 0；实际执行 `staticcheck ./...`，无诊断；`lint.log` | PASS |
| 构建 | `make build` | 0；构建CLI；`build.log` | PASS |
| 九项独立探针 | 下方完整 overlay 命令 | 0；全部9个顶层探针通过；`overlay.log` | PASS |
| 默认/额外来源、一级发现与精确范围 | `TestAcceptanceCatalogAndExactScope` | project/user/extra-1同名分开，发现不上传正文；非法名/嵌套/缺SKILL/目录型SKILL排除，外部库不因Git祖先扩大 | PASS |
| 目录枚举/返回限制 | `TestAcceptanceCatalogLimits` | 257技能返回256，完整JSON10525字节；4100普通条目返回0，JSON30字节；均truncated=true | PASS |
| 项目各层忽略 | `TestAcceptanceAncestorIgnoreRules` | 根/.lingcell/库/技能四层忽略规则均阻止reference，正文仍可读 | PASS |
| 根及内部链接/不可用库 | `TestAcceptanceSymlinkAndUnavailableRoots` | 根链接/祖先链接/普通文件根产生3个issues；缺失库允许，内部文件链接拒绝 | PASS |
| 完整JSON及共享预算 | `TestAcceptanceJSONBudgetBothDirections` | 转义正文完整JSON32764字节、内容5447字节、truncated=true；累计预算精确用尽后技能目录与项目读取均报预算错误 | PASS |
| cd保持固定根 | `TestAcceptanceFixedProjectAfterCwdChange` | 改到另一个合成项目后仍读取启动固定项目正文 | PASS |
| 配置目录上限/类型 | `TestAcceptanceSkillDirectoryConfigCap`及config回归 | 16目录接受、17拒绝；相对路径/null元素/NUL路径拒绝 | PASS |
| 首轮path:null缺陷 | `TestAcceptanceExplicitNullSkillPath`及`TestSkillArgumentValidation` | 显式null拒绝；省略/空字符串/参考路径仍合法 | PASS |
| 第二轮无库取消缺陷 | `TestAcceptanceEmptyLibraryCancellation`及仓库`TestEmptyLibraryCancellation` | 已取消context返回context.Canceled；源码入口及计费前检查已核对 | PASS |
| 安全边界/工具角色/确认 | `TestBoundaries`、`TestSkillGuidanceCannotAuthorizeCommands`、`TestSkillArgumentValidation`与源码审查 | 绝对/遍历/敏感/私钥/NUL/忽略/链接/FIFO拒绝；Skills未配置不广告且拒绝调用；恶意技能正文以role=tool返回，确认拒绝后零命令执行 | PASS |
| 首轮name冲突/真实CLI技能闭环 | 合成宿主XDG下`make terminal`，包装见下 | 0；skills场景4决策、0确认，完成发现→SKILL.md→references/check.md，正文及参考内容断言通过 | PASS |
| 首轮宿主XDG污染 | 同一包装写入synthetic-user技能再运行terminal | 0；only-project目录断言通过；合成宿主文件内容不变，未访问真实个人技能 | PASS |
| 完整PTY/通用任务/runtime | 同一`make terminal` | 0；三CLI会话、vim/top、本地ssh -V；路由/理解/诊断/机器拒绝/实际修改差异验证/失败/边界/取消；runtime连续轮次、恢复、锁/根、隔离记忆、不重放、ephemeral/no-history/存储故障保留shell通过；`terminal.log` | PASS |
| 文档/空白 | `git diff --check ef5acba95a5a728e82b62437d6251a652c93899d 73c403606aeba6b73cb750e994d63c935e2de5c4`及Python本地Markdown链接检查 | 0；5份改动Markdown的36个本地链接存在；文档读取范围/预算/权限/前缀检查边界与实现一致 | PASS |
| 最终版本/状态 | `git rev-parse HEAD`、`git status --short --branch`、`git status --short --ignored`、`git diff --exit-code`、`git diff --check` | 0；HEAD相同且源码干净，仅上述忽略生成物 | PASS |

九项独立探针确切命令（退出0）：

```bash
go test -count=1 -race -v -overlay=/tmp/lingcell-skills-r3-probes-bbkyskh5/overlay.json ./internal/skills ./internal/config ./internal/agent -run TestAcceptance
```

完整terminal确切包装（退出0；日志至临时目录；原实际调用还将退出码写入results.json）：

```python
import os, subprocess, tempfile
from pathlib import Path
with tempfile.TemporaryDirectory(prefix='lingcell-r3-host-xdg-') as task_xdg:
    skill = Path(task_xdg)/'lingcell/skills/synthetic-user/SKILL.md'
    skill.parent.mkdir(parents=True)
    skill.write_text('SYNTHETIC_ONLY_HOST_GUIDANCE')
    with open('/tmp/lingcell-skills-r3-probes-bbkyskh5/terminal.log', 'w') as log:
        code = subprocess.call(['make', 'terminal'], env=dict(os.environ, XDG_CONFIG_HOME=task_xdg), stdout=log, stderr=subprocess.STDOUT)
    assert skill.read_text() == 'SYNTHETIC_ONLY_HOST_GUIDANCE'
    raise SystemExit(code)
```

## 缺陷与复现

本次已执行检查未发现缺陷。此前四项缺陷均用原复现条件重新执行：辅助函数name冲突由真实CLI正文与引用分支完成证明修复；path:null由独立参数探针拒绝；宿主XDG污染由合成宿主技能、only-project断言及完整terminal通过证明隔离；空库取消由原独立探针得到context.Canceled证明修复。PASS只适用于上述候选SHA及已执行覆盖范围。

## 未运行项与限制

- Go1.26工具链暂不可用，未下载、未运行；Go1.27.2通过不替代最低版本验证。这是交接明确的可选检查，不列为本轮验收阻塞。
- 未调用真实供应商模型或远程SSH，未读取真实凭据、个人skills或个人bashrc；脚本明确SKIP无授权远程SSH/个人bashrc。本地ssh -V不代表远程SSH联调。真实模型技能选择效果尚未验证。
- 未进行持续恶意文件系统竞争压力测试；内容检查只覆盖文档规定的读取前缀，不保证未读后缀安全。

## 返回开发者的动作

可以记录本轮独立PASS证据；协调者仍须核对main基线、候选SHA及AGENT.md合入门槛。验收后只能追加证据文档；修改产品、测试、规范、依赖、脚本等须重新独立验收。本报告不授予合并权限。


协调记录：最终候选与独立PASS身份已核对，main仍等于基线且干净。验收后只追加本交接/开发日志证据；当前远程已连接，未推送、未建PR、未合入main，功能保存在已提交的feat/agent-skills任务分支。
