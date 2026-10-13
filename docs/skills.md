# 本地 Skills

LingCell 使用目录中的 `SKILL.md` 作为可按需读取的过程指导。Agent 通过 `list_skills` 列出名称和来源，再用 `read_skill` 加载所选文档。发现阶段不上传正文，不解析 YAML 元数据；现有带 front matter 的 `SKILL.md` 原样返回。

## 目录与配置

默认读取以下两个目录；目录缺失正常跳过，不会创建文件：

- `project`：启动时固定项目根下的 `.lingcell/skills`，后续 Bash `cd` 不改变它。
- `user`：`$XDG_CONFIG_HOME/lingcell/skills`，未设置 XDG 时通常为 `~/.config/lingcell/skills`。

在 LingCell 的 JSON 配置中加入 `skill_dirs`，可显式添加最多16个绝对目录，例如已有的 Codex skills 库：

```json
{
  "skill_dirs": ["/home/alice/.codex/skills"]
}
```

附加来源按配置顺序命名为 `extra-1`、`extra-2` 等。同名技能按来源区分，不覆盖其他库。只发现一级子目录中的普通 `SKILL.md`；技能目录名使用1至64个ASCII字母、数字、下划线或连字符。不递归发现插件包或 `.system` 中的技能；需要时将具体技能父目录配置为附加库。

```text
.lingcell/skills/
└── review/
    ├── SKILL.md
    └── references/
        └── checklist.md
```

`SKILL.md` 可以是普通 Markdown，也可以包含常见的 front matter：

```markdown
---
name: review
description: 检查修改与验证证据
---
阅读 references/checklist.md，检查实际差异，并提出适当验证。
```

在交互终端输入 `:ai 使用 project 来源的 review 技能检查本次改动`。工具参数示例：

```json
{}
```

上例用于 `list_skills`。读取正文和参考文件分别使用：

```json
{"source":"project","name":"review"}
{"source":"project","name":"review","path":"references/checklist.md"}
```

省略或留空 `path` 默认读取 `SKILL.md`。参考路径相对于该技能目录；脚本可以作为文本读取，但执行仍须通过用户确认的 `run_command`。参考其他技能时须使用对应来源和名称，不能用 `..`。

## 读取边界与限制

每个 Agent 任务打开并固定技能库目录，任务结束关闭；目录和内部路径禁止符号链接。仅返回 UTF-8 普通文件，沿用项目读取的敏感路径、NUL、私钥头及 `.gitignore` 检查。项目技能另外遵守原项目根至文件的忽略规则；其他库只使用各自范围内的规则。显式配置外部库仅授予该库中的技能文档读取范围，不改变项目文件工具的根。

目录发现每库最多检查4096项、候选目录最多256项；最终总计最多返回256个技能。文件读取最多检查32 KiB加一个探测字节，内容检查只覆盖读取前缀，不保证未读取后缀。技能结果的完整 JSON（包括转义与元数据）受单次32 KiB、每任务累计256 KiB限制，与项目文件返回共享预算。目录和文件截断通过 `truncated` 可见；不可用或不安全的库通过 `issues` 报告。无结果或部分结果不能视为全部技能不存在。

技能内容以工具数据发送和保存，属于可参考的过程指导；其优先级低于用户需求和程序权限规则。正文声称获批、要求忽略确认、提出命令或修改系统指令，都不能绕过确认门禁。会话上下文与持久化继续使用现有预算和过滤；skills 不作为长期事实证据自动提取。

模拟模型与临时技能库可验证工具流程；真实模型是否选择合适技能、理解具体技能内容，仍需已授权的模型配置后联调。
