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
- 新一轮独立报告：待复验。
