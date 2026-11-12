# 更新日志

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [SemVer](https://semver.org/lang/zh-CN/)。

## [1.0.0] - 2026-11-12

### 新增

- `dscli whoami / list / history / send / tail` 五个子命令
- `DeepSeekHashV1` 求解：内嵌站点 `sha3_wasm_v1.wasm`，经 wazero 驱动（约 200ms/次）
- SSE 增量解析，支持思考流与正文流分离
- 会话接续（`-session`）与手机端对话镜像（`tail`）
- 交叉编译发布矩阵：linux/amd64、linux/386、windows/amd64、darwin/amd64
- GitHub Actions：tag 触发编译 + SHA256SUMS + Release
- 附带 `docs/protocol.md`（端点/头/PoW/SSE 全部实测记录）

### 修复

- 首个整包帧被忽略导致回复开头丢字的问题（有回归测试）
- `fetch_page` / `history_messages` 用 POST 触发 405 的问题

[1.0.0]: https://github.com/Yourtokenkeeper/dscli/releases/tag/v1.0.0
