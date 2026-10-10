# 开发交接：Agent 分工与 Git 工作流

## 需求和角色

用户要求开发与测试 Agent 分开，正式测试派生干净上下文 subagent，规范双方交接；所有有意义的修改保存 Git 记录，使用任务分支/worktree 开发，让后续 Agent 遵守。

- 开发/协调：主 Agent，负责文档、hooks、分支/worktree 配置及测试后的整合。
- 测试：另行派生无开发对话继承的测试 Agent，独立检查候选，默认只读源文件，不提交/合并，不修改共享配置。
- 项目根：`/home/liang_guang_can/LingCell`。
- 开发工作树：`.worktrees/agent-workflow`，分支 `docs/agent-workflow`。
- 基线：`main`，完整 SHA 为 `948a7a0153eb4246fed64783b7180849167584d3`。
- 候选完整 SHA 与 detached 测试目录在派生消息中提供，从提交后 Git 查询，避免文件自引用。

## 改动范围

- AGENT.md 强制分工、上下文/文件隔离、提交与合入/回滚规则；AGENTS.md 和 CONTRIBUTING.md 发现入口。
- docs/workflow.md 实际操作手册与 docs/templates 的双方交接模板；README 入口与开发日志。
- .gitignore 排除 .worktrees；.githooks 的主分支/分支名提交拦截与提交标题格式检查。
- 本地 Git 默认分支由 master 改为 main，原有提交不改写；开发和测试使用独立 worktree。

## 验收条件

1. 后续 Agent 能从入口发现强制规范；产品“无多 Agent”限制与维护 subagent 区分清楚。
2. 文档要求独立新上下文、固定候选 SHA、不同工作树、最小交接、可复现反馈及修复后重测，没有自测替代验收或循环自引用要求。
3. main 只整合已验证成果，所有有意义的改动需提交；任务分支命名、原子提交、WIP、整合基线变化、回滚与清理规则清楚可执行。
4. 所有 Markdown 相对链接有效，差异空白检查通过，hooks 可执行且 POSIX shell 语法通过。
5. 隔离临时 Git 仓库测试 hooks：main/master 与 detached/不规范分支直接提交被拒；合法任务分支提交成功；合规和带 scope/! 的标题通过，非法标题拒绝；规范标题的 no-ff merge 到 main 成功。
6. 检查当前实际分支/worktree 与候选身份，测试结束源树不变，无真实服务或凭据访问。
7. 复测回滚修正：普通/合并 revert 使用 --no-commit，暂存审查能看到实际反向改动，以 revert: 标题正常提交，文档准确说明 hooks 对自动提交的覆盖边界。

## 建议检查和边界

建议执行 `git diff --check <base>..<candidate>`、Markdown 本地链接检查、`sh -n`、临时目录内 hooks 正反验证及工作树身份检查。测试仓库使用临时 HOME/本地身份，不推送、不接入外部服务。

此次没有 Go 产品代码变动；Go 与真实 PTY 整套测试不属于本次最低验收，hooks 行为需实际运行验证。不得把先前 Go 测试结果记为本次执行。服务器分支保护尚未配置，本地 hooks 不能证明远程权限受控。

## 测试报告

独立报告将在验收后追加，包含实际候选 SHA、命令/结果及未运行项；追加报告不修改已验收的规范或 hooks。

### 第一轮反馈：FAIL

- 测试 Agent：workflow_acceptance，使用无开发对话继承的上下文。
- 基线：`948a7a0153eb4246fed64783b7180849167584d3`。
- 候选：`5e1757af555bf6c5e49bd5b0cb2cf9accc039644`，detached 工作树保持不变。
- 已返回结果：hooks 正反检查、文档链接和身份检查通过，回滚说明存在两个需修正的问题。
- 复现 1：`git revert --no-commit` 的改动在暂存区，旧示例的 `git diff` 输出为空；`git diff --cached` 才能审查实际反向差异。
- 复现 2：Git 2.43.0 的直接 `git revert -m 1` 自动生成 `Revert "merge: ..."` 提交，并不经过相同的 commit-msg 检查，违反项目标题规则。
- 修订：统一使用 `--no-commit`、检查暂存差异、用 `revert: ...` 正常提交，并明确 hooks 覆盖边界。修订后的候选需由新的干净上下文测试 Agent 完整复测。
