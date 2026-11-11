# dscli — DeepSeek C 端终端客户端

在终端里使用 **DeepSeek 消费端**（chat.deepseek.com，也就是手机 App 用的那套后端），复用你已经登录的账号，不需要 API Key。支持流式输出、会话列表、接续既有会话、把手机上的讨论搬到终端。

> 不是官方平台 API（api.deepseek.com）。调用计费走你的账号额度，与 App 内对话同等对待。请自行控制频率。

## 特性

- **单文件二进制**：Go 编写，无 CGO、无运行时依赖，Linux/macOS/Windows 均可交叉编译
- **官方 PoW 原样复刻**：内置站点自己的 26KB `sha3_wasm_v1.wasm`，用纯 Go 的 [wazero](https://wazero.io) 驱动，`DeepSeekHashV1` 一次求解约 200ms
- **流式**：SSE 增量解析，逐字打印
- **会话能力**：`list` / `history` / `send -session` / `tail`（把手机上的对话实时镜像到终端）

## 快速开始

```bash
# 1. 拿到凭证：App 的登录 token（root 设备）
#    /data/data/com.deepseek.chat/files/mmkv/mmkv.default → key_user_info → {"token":"..."}
mkdir -p ~/.dsh/deepseek
cat > ~/.dsh/deepseek/session.json <<'JSON'
{"token": "你的64位token", "uid": "用户id"}
JSON
chmod 600 ~/.dsh/deepseek/session.json

# 2. 用
dscli whoami
dscli list -n 5
dscli send "用一句话说明你在终端里"
dscli send "接着刚才那个说" -session <session-id>
dscli tail <session-id>          # 手机上继续聊，终端实时看到
```

凭证来源优先级：`-session-file` → `$DS_TOKEN` → `$DS_SESSION_FILE` → `~/.dsh/deepseek/session.json`。

## 构建

```bash
make build          # 本机
make cross          # linux/amd64, linux/386, windows/amd64, darwin/amd64 → dist/
make test
```

| 平台 | 架构 | 产物 |
|---|---|---|
| Linux | x86_64 | `dscli_linux_amd64` |
| Linux | x86 (i386) | `dscli_linux_386` |
| Windows | x86_64 | `dscli_windows_amd64.exe` |
| macOS | x86_64 | `dscli_darwin_amd64` |

发布流程见 `.github/workflows/release.yml`：打 tag（`v1.2.3`）后自动编译全部架构、生成 `SHA256SUMS`、上传 Release。

## 命令

```
dscli whoami                        当前登录身份
dscli list [-n 10]                  最近会话（id + 标题）
dscli history <session> [-n 20]     某会话的消息
dscli send "文本" [-session id] [-think] [-search] [-quiet]
dscli tail <session> [-interval 3]  轮询并打印新消息
dscli version
```

## 协议要点

详见 [docs/protocol.md](docs/protocol.md)。三个容易翻车的点：

1. **版本门**：必须发 `x-client-version: 1.0.0-always` + `x-client-platform: web`，用安卓端的版本号会得到 `CLIENT_VERSION_TOO_LOW`。
2. **PoW**：`DeepSeekHashV1` 不是标准 Keccak/SHA-3 的任何一种（轮数与填充都不同），只能跑站点自带 wasm —— 本仓库直接嵌入并复刻 glue ABI。
3. **请求体**：`/api/v0/chat/completion` 必须带 `source` / `action` / `preempt`，少一个就 422；`fetch_page` 与 `history_messages` 是 GET，用 POST 会 405。

## 安全与边界

- token 等价于账号登录态，**不要提交进仓库、不要贴进对话**；`session.json` 权限 600。
- 本项目只做「复用自己账号的登录态」这一件事，不包含任何注入、绕过限流或批量注册逻辑。
- 请求有节流（每次 `send` 前留白），别高频轰炸，异常调用最先影响的是这个号。

## 参与

见 [CONTRIBUTING.md](CONTRIBUTING.md)。安全问题请走 [SECURITY.md](SECURITY.md)，版本变更见 [CHANGELOG.md](CHANGELOG.md)。

## License

MIT，见 [LICENSE](LICENSE)。

---

## English

A terminal client for DeepSeek's **consumer** backend (`chat.deepseek.com`) that reuses the
token the official mobile app already holds — no API key, no proxy layer. Single static
binary (Go, no CGO), streaming SSE output, session list/history, and `tail` to mirror a
conversation the app is driving.

It reimplements the `DeepSeekHashV1` proof-of-work by embedding the site's own
`sha3_wasm_v1.wasm` and running it through [wazero](https://wazero.io), because the hash is
a non-standard Keccak variant that no stock library reproduces.

Build with `make cross` for `linux/amd64`, `linux/386`, `windows/amd64`, `darwin/amd64`.
MIT licensed. Use your own account; keep the token out of version control.
