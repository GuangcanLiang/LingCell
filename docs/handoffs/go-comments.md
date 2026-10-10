# Go 注释规范：开发与独立验收交接

## 开发交接

- 需求：为现有 Go 代码补充符合 Google Go 风格及 Go 官方文档惯例的注释；保持执行逻辑不变，并让后续维护者遵守同一约定。
- 职责：开发 Agent 修改和提交注释/规范；测试 Agent 在全新上下文中独立审查源码与差异、运行检查、反馈真实证据，不修改产品代码或共享 Git 状态。
- 项目根目录：`/home/liang_guang_can/LingCell`。
- 开发工作树/分支：`/home/liang_guang_can/LingCell/.worktrees/go-comments`，`docs/go-comments`。
- 基线：`main`，`18f99b4f60923bd91e1cecee6d911f5f3e2e284c`。交接前须以 `git rev-parse main` 核对。
- 候选完整 SHA 与 detached 测试工作树：候选提交产生后由协调者在测试启动消息中指定；本文件不能记录包含自身的提交 SHA。报告返回后补充实际受测提交和目录。
- 改动：8 个 Go 实现文件和 7 个测试文件的文档/逻辑说明，`AGENT.md` 的长期注释约定，以及本交接和开发日志。重点描述接口条件、取消、资源所有权、Bash 协议、权限与文件预算；不新增功能或依赖。
- 依据：[Google Go Commentary](https://google.github.io/styleguide/go/decisions#commentary)、[Go Doc Comments](https://go.dev/doc/comment)。Go 注释沿用英文，维护文档为中文。

## 验收条件

1. 对照实现审查注释：导出名称文档以名称开头、完整句子；包说明每包一份；说明真实的错误、预算、权限、生命周期与并发条件。测试注释不夸大已有断言的覆盖范围。
2. 使用 Go 扫描器比较基线/候选全部 Go 文件，忽略普通注释和位置但保留自动分号及所有字面量，token 序列必须完全一致。另核对构建标签/编译或生成指令完全一致；非 Go 的执行文件、依赖和配置必须没有改动。不要仅靠“编译通过”判断行为不变。
3. 执行 `make check`、`make lint`、`make build`。检查 Go doc 可读性、Markdown 本地链接、`git diff --check <base>..<candidate>` 和文件范围。
4. 开始/结束记录 detached HEAD、工作树状态及环境。报告 PASS / FAIL / BLOCKED、完整命令与退出码、缺陷和未运行项。生成的忽略目录 `bin/` 可保留并说明；源码不得修改。

## 环境和限制

需要 Linux、Bash、Go、Staticcheck、可写 Go 缓存和本地模拟 HTTP 监听权限。命令受到环境限制时使用工具权限流程，不绕过隔离。测试子 Agent 使用 `fork_turns="none"`，自行读取 `AGENTS.md`、`AGENT.md`、源码与基线/候选差异。

仅注释改动且 token 与构建约束一致时，不要求重复真实 CLI 的完整 PTY 验收；已有 Go 会话测试仍随竞态测试运行。未选择或授权真实供应商/远程主机，不配置密钥、不发送真实模型请求或远程 SSH。最低 Go 1.26 工具链另未联调。测试 Agent 不提交、合并、更改 refs/config/hooks 或修改开发工作树。

## 独立测试报告

候选尚未正式验收；实际结果将在独立测试 Agent 返回后追加。该状态不能视为 PASS。
