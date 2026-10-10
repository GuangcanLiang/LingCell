# 开发、独立测试与 Git 工作流

规范以 [AGENT.md](../AGENT.md) 为准。本文给出操作步骤，适用于开发者和后续接手的 Agent。本项目采用稳定 `main` + 短期任务分支；完成并通过验收的功能最终整合到 `main`。

## 工作树布局

```text
LingCell/                           main：稳定整合工作树
├── .git/                           共享的 Git 对象、引用和工作树登记
├── .githooks/                      本地提交检查
├── .worktrees/                     被忽略，不提交
│   ├── <topic>/                    feat/fix/docs/... 任务分支
│   └── verify-<topic>-<short-sha>/  指定候选 SHA 的 detached 测试工作树
└── docs/handoffs/<topic>.md         可追踪的需求、交接与验收证据
```

worktree 共享 Git 历史，但工作目录、索引和 HEAD 分开；测试时固定完整提交 SHA，避免验证过程读到正在变化的开发文件。[Git worktree 文档](https://git-scm.com/docs/git-worktree)

当前已有的初始化提交保留。默认分支已统一命名为 `main`；后续任务按本文建新分支，无需改写旧历史。工作树目录放在仓库内是为了适配当前工作区写权限，普通本地环境也可使用有权限的仓库同级目录。

## 1. 开发前建立任务

在主工作树执行，先检查存在的工作树、分支和用户修改。目录或分支已经存在时核实归属，不能用覆盖参数。

```bash
git status --short --branch
git worktree list
git log -5 --oneline
git config --local core.hooksPath .githooks
git config --local --get core.hooksPath
git worktree add -b feat/example .worktrees/example main
cd .worktrees/example
git status --short --branch
git rev-parse HEAD
```

把 `example` 改为实际任务名，按需要使用 `fix/`、`docs/` 等前缀。开发 Agent 阅读规范、明确验收行为、修改任务目录，补必要测试与说明。并行任务各用自己的分支和目录，不共享未提交修改。[Git 分支工作流](https://git-scm.com/book/en/v2/Git-Branching-Branching-Workflows)

## 2. 保存候选提交

开发 Agent 做必要自查，按独立改动单元明确暂存。候选必须先提交才能交接完整 SHA；提交只是开发记录，是否通过验收另由独立测试报告证明。

```bash
git diff --check
git diff
git add -- <本任务相关文件>
git diff --cached --check
git diff --cached
git commit
git rev-parse HEAD
```

`git commit` 的标题和正文示例：

```text
fix(shell): preserve foreground job state

Explain the observed problem and the resulting behavior.
Validation: developer preflight; independent acceptance pending.
Limitations: list relevant unverified behavior.
```

每个有意义的功能、修复、重构或其他变动都要保存提交。可以先提交多个完整改动单元，再验收任务全体；不要留未提交的最终修改。WIP 检查点用 `chore(wip): ...`，只用于任务分支，未完成状态不能整合到主分支。

## 3. 固定版本并交给测试 subagent

用 [交接模板](templates/handoff.md) 填写 `docs/handoffs/<topic>.md` 并提交。候选 SHA 在该次提交后从 Git 获取，通过消息发送；不在文件内写该文件所属提交自身的 SHA。

在主工作树执行：

```bash
git worktree add --detach .worktrees/verify-example-<short-sha> <candidate-full-sha>
git -C .worktrees/verify-example-<short-sha> rev-parse HEAD
git -C .worktrees/verify-example-<short-sha> status --short
```

派生独立测试 Agent 时用 `fork_turns="none"` 或工具等价的全新上下文选项，提供：

```text
角色：独立测试 Agent；不得修改产品代码、提交、合并或修改 Git 配置。
仓库根：<absolute-repository-root>
测试目录：<absolute-detached-worktree>
基线完整 SHA：<base-full-sha>
候选完整 SHA：<candidate-full-sha>
交接文件：docs/handoffs/<topic>.md
任务：先读该工作树的 AGENTS.md / AGENT.md，再独立检查差异和验收条件。
回报：按 docs/templates/test-report.md 返回结果、证据及未运行项。
```

不要复制整个开发对话，或要求测试者证明开发实现正确。测试者需独立看差异和需求。检查前后核对 HEAD 和状态，不升级依赖，不修改共享 refs/config，不读真实密钥，不调用未授权供应商。

| 改动范围 | 独立验收最低要求 |
| --- | --- |
| 纯说明文档 | 链接/命令/规范一致性及差异空白检查 |
| Go 代码、测试或依赖 | 相关回归测试、`make check`、`make lint`、`make build` |
| 终端、Agent 交互或信号/作业 | 上项加 `make terminal` |
| hooks、构建/工具脚本、工作流配置 | 语法/适用工具检查和正反行为验证；影响应用时追加相应 Go/终端验收 |

工具缺失或必要权限受限，测试者报告 BLOCKED 及原因。权限审批按运行环境要求申请；不能把没有执行的验收列为通过。

## 4. 反馈、修复与保存证据

测试者按 [报告模板](templates/test-report.md) 返回状态、实际提交、命令/退出码、缺陷复现与未运行项。开发者把真实报告摘要追加到交接文件，更新开发日志并提交。

FAIL 后修复代码并提交新候选，每轮重新派生干净上下文测试 Agent，附上复现和新 SHA。验收后的代码/依赖/测试/脚本/hooks/规范变化会使原报告失效。只追加验收证据和日志的后续提交可沿用原代码验收，但协调 Agent 必须检查追加差异只包含证据，并检查文档。

## 5. 整合已验证成果

协调 Agent 核对：测试 PASS、要求满足、没有阻塞项、候选身份正确、开发工作树干净、后续差异仅有证据。还需核实任务历史没有待完成的 WIP，以及主工作树干净、`main` 等于已验收的基线。

若 `main` 更新过，在任务分支合并最新 `main`，解决冲突后提交并重新独立验收。不要在主工作树直接解冲突提交未验收成果。

满足上述条件且有整合授权时，在主工作树执行：

```bash
git status --short --branch
git rev-parse main
git log --oneline main..feat/example
git diff <candidate-full-sha>..feat/example
git merge --no-ff feat/example -m "merge: integrate example"
git diff --exit-code main feat/example
git log --graph --oneline --decorate -12
git status --short --branch
```

`--no-ff` 保留独立合并提交，任务提交仍可追踪，整项任务也有明确边界。这是本项目选用的整合策略；Git 也支持其他合并方式。[Git merge 文档](https://git-scm.com/docs/git-merge)

当前没有远程平台。连接远程后应通过 PR、独立审查和所需状态检查合入，并配置服务器分支保护；本地 hooks 可以被绕过，不能代替服务器保护或测试报告。推送、发布按用户授权执行。

## 6. 清理与回滚

确认报告已提交、工作树无未保存内容且无人使用后，正常删除工作树，再删除已合入分支：

```bash
git worktree remove .worktrees/verify-example-<short-sha>
git worktree remove .worktrees/example
git branch -d feat/example
git worktree list
```

不要用 `--force` 清理；构建输出等被忽略的文件也需检查是否应保留，工作树有内容时先处理再移除。无需维护一个永久空开发分支；下个任务从新的 `main` 建分支/worktree。

需要回滚时从 `main` 新建 `revert/<topic>` 工作树，新增反向提交并走独立验收。普通提交可用：

```bash
git revert --no-commit <bad-commit-sha>
git diff --cached
git diff --cached --check
git commit -m "revert: undo example change"
```

撤销任务合并时，先核实合并提交的父提交，再使用 `git revert --no-commit -m 1 <merge-sha>`，按同样流程审查、提交、测试和合入。撤销合并会影响未来再次合并同一历史的行为，需要评估；不是清除旧提交。若发生冲突，修复后完成候选再验收。[Git revert 文档](https://git-scm.com/docs/git-revert)

## 本地 hooks 的范围

`.githooks/pre-commit` 拒绝 main/master、detached HEAD 和不符合任务前缀的直接提交。`.githooks/commit-msg` 检查提交首行标题，支持可选 scope 和破坏性标记 `!`。正常整合使用规范的 `merge: ...` 标题。

这些 hooks 检查正常的 `git commit` 流程。Git 某些自动创建提交的操作（例如直接 `git revert`）不执行相同的 hooks，所以回滚必须使用上文的 `--no-commit`，审查后再通过正常提交流程生成 `revert: ...` 记录。

hooks 未实现独立 PASS 自动判定或服务端分支保护；这些条件由协调 Agent 和今后的远程平台检查。不得用跳过 hooks 或重写共享历史来绕开规范。
