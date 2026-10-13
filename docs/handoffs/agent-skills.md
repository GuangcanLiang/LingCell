# Agent skills 开发 → 独立测试交接

- 任务/需求：给 LingCell Agent 加上发现并读取本地 skills 的能力。
- 职责：开发 Agent 实现代码/回归/文档并提交；独立测试 Agent 自行审查与验收，不修改产品代码、不提交/合并或修改共享配置/refs。
- 项目根：`/home/liang_guang_can/LingCell`。
- 开发 worktree / 分支：`/home/liang_guang_can/LingCell/.worktrees/agent-skills` / `feat/agent-skills`。
- 基线：main，`ef5acba95a5a728e82b62437d6251a652c93899d`。
- 候选完整 SHA / detached 测试目录：提交后由协调消息提供，验收报告回填。
- 改动范围：internal/skills（技能库/文档/预算）、internal/project（精确范围和元数据检查）、internal/agent（工具/提示/校验）、internal/app（按任务生命周期接入）、internal/config（skill_dirs）、相关测试、scripts/verify_tasks.py、README/AGENT/docs/skills.md。
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
- 上轮缺陷：首次交接，无。

独立报告待回填，开发自查不作为验收结论。
