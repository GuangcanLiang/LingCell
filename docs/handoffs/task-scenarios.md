# 通用任务回归：交接

- 需求：同一Agent完成项目理解、开发排障、机器排障、修改代码与实际验证；拒绝/失败/边界/取消可观察。模拟测试不代表真实模型效果。
- 职责：开发Agent维护固定fixture与回归；独立测试Agent审查/运行，不修改产品代码、提交、合并、refs/config 或调用真实服务。
- 根：`/home/liang_guang_can/LingCell`；开发 `.worktrees/task-scenarios` / `test/task-scenarios`。基线main：`03897097aefdb89c5544231c50316b5b3b2bf597`（已独立验收工具集），候选完整SHA由派生消息提供；测试固定候选 detached `.worktrees/verify-task-scenarios-<sha>`。
- 范围：scripts/verify_tasks.py 新固定任务模拟服务+真实CLI/PTY；Makefile terminal 接入；README、场景文档与日志。另修正 agent 初始上下文的注释以匹配字符串/字符串列表结构，Go执行行为不变。
- 验收：详见 [固定任务场景](../task-scenarios.md)，每项都检查真实文件/命令结果和工具数据，模型不接受伪造预期输出；所有生成命令须确认；修改前后差异/验证实际，验证失败不可输出模拟成功；机器状态先获批查询，处理拒绝后无执行/后续请求；边界不泄漏；Ctrl-C和终端恢复；历史无生成命令/工具数据；记录决策/确认数；旧三会话兼容。
- 命令：make check、make lint（实际包分析）、make build、make terminal（含新任务脚本）；Python语法、文档链接、git diff --check。可写临时Go/Staticcheck缓存、本地HTTP/PTY权限；不得升级依赖。
- 限制：固定项目是Bash代码，机器输出来自合成文件；模拟模型最终文本按fixture证据生成，不验证真实模型决策质量；扫描中取消/完整工具边界由已有Go回归覆盖，CLI取消在文件读取后的模型等待阶段。真实供应商、SSH、个人配置、人工日常试用仍未验证。
- 首轮候选 `90110036e58b5db3f21e9a3e244d5fbbb4833c79`：开发侧额外审阅发现模拟配置遗漏api_key_env，默认LINGCELL_API_KEY可从父环境继承并发送本地mock的Authorization；已通知独立测试Agent确认，旧候选不合入。
- 复现/期望：仅设置合成LINGCELL_API_KEY哨兵运行任务fixture，旧配置默认读入该值；测试必须不读取或发送父环境凭据。修复显式api_key_env为空，并在mock断言无Authorization；复验必须带合成父环境哨兵验证所有场景通过，不读取真实值。
- 新候选完整SHA由派生消息提供；需全新上下文重新执行原验收条件及该回归。
- 独立报告：待复验。
