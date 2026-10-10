# 参与开发

开始维护前必须阅读 [AGENT.md](AGENT.md)。它定义 Go 约定、开发与测试 Agent 的强制分工、Git 提交和合入条件。

操作步骤见 [开发工作流](docs/workflow.md)。每项任务从 `main` 创建短期分支与独立工作树，开发后把候选提交交给干净上下文测试 Agent，独立验收通过后整合。

新 clone 先启用仓库自带的本地 hooks：

```bash
git config --local core.hooksPath .githooks
git config --local --get core.hooksPath
```

入口文档不另写一份规范；维护规则变更统一更新 AGENT.md 及必要的操作示例、交接模板和开发日志。
