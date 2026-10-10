# Go 开发规范检查

检查日期：2026-10-10（Asia/Shanghai）。依据 Go 官方建议与当前源码检查，并修正发现的问题。Go 没有要求所有应用使用同一种目录模板；本项目把通用惯例与项目自身的维护约定分别记录。

## 检查结果

| 项目 | 结论与处理 |
| --- | --- |
| 项目组织 | CLI 入口放在 `cmd/lingcell`，实现包放在 `internal`；职责按终端、路由、模型、Agent、项目读取与配置划分，适合当前应用规模。 |
| 格式与命名 | 使用 `gofmt`，标准库、第三方依赖与项目导入分组；沿用简短包名及 `HTTP`、`URL`、`ID` 等命名。 |
| 导出 API 文档 | 补齐主要导出类型、接口与方法的 Go doc，说明 Session、Reader、Agent 的生命周期、资源所有权和并发使用范围。 |
| 错误处理 | HTTP 请求错误保留 `Unwrap`，允许 `errors.Is` 判断超时/取消，同时保持终端消息不含传输详情；关键初始化、历史文件关闭、工具参数解码、工具结果编码和模型文本输出错误得到检查。 |
| 资源释放 | 固定项目根描述符的 `Close` 支持重复调用；非阻塞描述符设置在启动轮询前完成，失败会终止 Bash 并释放句柄；保留 Session 的幂等退出和终端恢复。 |
| 输入校验 | 配置读取明确拒绝超过 64 KiB 的文件及非 JSON 对象，避免限制读取流后把超大文件截断为合法配置。 |
| 接口与取消 | Agent 的 shell/文件能力由消费者定义小接口，便于模拟；可取消操作使用首参数 Context；HTTP 与 Agent 有请求超时，终端会话有取消流程。 |
| 测试 | 已有包级测试、竞态检查、本地 HTTP 模拟及真实 CLI PTY 验收；增加配置边界、HTTP 原因保留、重复 Close 和输出失败阻止后续确认的回归测试。 |
| 依赖 | `go.mod` / `go.sum` 固定依赖，`go mod tidy -diff` 检查无差异；本次没有升级依赖。 |
| 日常检查 | `Makefile` 提供格式、模块、测试、竞态、vet、Staticcheck、构建和终端验收命令；`make check` 是统一基础检查入口。 |
| 版本管理 | MVP 已有本地 Git 基线，后续代码与维护文档分别提交；构建输出、缓存、Python 字节码与 `.env*` 被忽略。 |

关键路径会检查错误；只读句柄的清理、进程已退出后的信号等使用尽力清理。当前由固定字符串字段组成的初始 JSON 编码没有运行时编码失败来源。

## 实际验证

验证环境：Linux/amd64，Go 1.27.2，Staticcheck 2026.2.1（模块版本 `v0.8.1`）。`go.mod` 声明最低 Go 1.26.0；本次使用本机工具链，未单独运行最低版本矩阵。

| 命令 | 结果 |
| --- | --- |
| `make fmt` | 已格式化 Go 源码。 |
| `make check` | 通过：格式、模块整理检查、`go vet ./...`、`go test -race ./...`；7 个实现包通过，CLI 入口没有单元测试。 |
| `make lint` | 通过，Staticcheck 没有报告问题。 |
| `make terminal` | 构建成功，真实 CLI PTY 与本地模拟模型验收通过。 |
| 文档检查 | `git diff --check`、5 份 Markdown 的相对链接与空白检查通过。 |

终端验收覆盖持久状态、别名及路径补全、续行、Ctrl-C/Z、`jobs/bg/fg`、窗口尺寸、Agent 多步工具反馈及确认/拒绝、失败诊断、分类、请求取消、输入历史与终端恢复。`vim` / `top` 存在时实际运行；SSH 检查只执行 `ssh -V`。

## 后续边界

- 真实供应商工具调用和成功的远程 SSH 会话尚未联调。
- 当前 `module lingcell` 可本地构建；选定远程仓库并需要远程安装时，再统一调整 module 与 import 路径。
- 当前检查由开发者本地执行，尚未配置远程 CI，也未声明发布或部署完成。需要 CI 时可复用 `make check`、固定版本 Staticcheck 与 Linux PTY 验收。
- lint 和测试通过代表已执行检查未发现问题，不能推导为所有个人 Bash 配置、Linux 环境或异常场景都已覆盖。

## 参考依据

- [Go 模块组织](https://go.dev/doc/modules/layout)：命令与内部实现包的组织方式。
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)：命名、注释、错误、Context、接口与并发维护建议。
- [Effective Go](https://go.dev/doc/effective_go)：格式、包命名、控制结构及错误处理惯例。

项目具体维护流程见 [AGENT.md](../AGENT.md)，开发记录见 [开发日志](development-log.md)。
