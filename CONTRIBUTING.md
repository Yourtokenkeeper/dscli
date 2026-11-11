# 贡献指南

## 环境

- Go 1.21+（`go.mod` 里声明的最低版本）
- 无需 CGO、无需 Node、无需 Docker

```bash
make test     # go test ./...
make vet      # go vet ./...
make cross    # 交叉编译四个目标平台
```

## 提交前

1. `gofmt -l .` 必须无输出
2. `go vet ./...` 与 `go test ./...` 全绿
3. 改动涉及协议时，同时更新 `docs/protocol.md`
4. 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)：
   `feat: …` / `fix: …` / `docs: …` / `chore: …`

## 不要提交

- 任何真实 token、uid、cookie（`examples/session.example.json` 只是形状示范）
- `dist/` 产物（已 gitignore，由 CI 生成）
- 抓包原文、含个人信息的日志

## 代码约定

- 新增端点请成对补 `internal/api` 方法与 `cmd` 子命令
- 解析侧改动必须带测试：`internal/api/client_test.go` 是 SSE 回归防线
- 不要在库代码里 `panic`，错误一律向上返回

## PR

- 一个 PR 只做一件事；附上 `go test` 输出
- UI/输出格式变更请贴一段真实运行结果（可脱敏）
