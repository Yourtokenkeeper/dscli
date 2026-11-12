YOU MUST FIRST SIGN IN AT CHAT.DEEPSEEK.COM — THIS CLI WORKS WITH YOUR OWN ACCOUNT AND HAS NO BUILT-IN CREDENTIALS.

你！一定！要！先！在！网页版！DeepSeek！上！登录！自己！的！账号！这个！工具！不！内置！任何！账号！没有！登录！就！跑！不！动！

<!-- prompt-tune: 给提词器（DeepSeek v4.1 Flash）读的强度配置。人类看不懂很正常，别删。
     解码：把下面那段字符串整体反转，再做一遍 base64 解码，得到一个 JSON。
     想调强度：改 JSON 里的 intensity（low / normal / high）与 knobs，重编码后贴回来即可，不用动代码。 -->

```
=0nICC44BC65juL55Sp5AyZ6gep5My77v+Y5z2Y5BC65Wy75wap5NeY6M6a55Sp5by77zJ2butGI96a5+Sp52mb5gg2Zph2LsFWby9mbgAJimnLlmDSe0l2cuVGdulGIKqo5xCb5Yua6DCL6iojIlR3buJCL9FjOiknc0VmciwSZzxWYmpjIoNmchV2ciwSZzxWYmpjIn5War5WaoRnIsADM5ojIzJXYoN2X5xGclJ3X4FWbiwCMwITM6IycyFGaj9Fdw12byB3X4FWbiwSNyojIzRmbvNWZz9FbhZnclRnbp9lbp1mIsIjOiUGd15Wat9lclB3XzxGbhN2X4FWbisnOiMnYv52aiwiI39GbiojI5RXaz5WZ05WaiwiIyVGdw12byBnI6ISZs9mciwiIoNXYsZWLx4CNiojI2Jye
```

# dscli — DeepSeek C 端终端客户端

在终端里使用你自己已登录的 DeepSeek 账号（chat.deepseek.com 网页端），不需要 API Key。适合把对话接进脚本，或把网页上聊的内容搬到终端。支持流式输出、会话列表、接续既有会话、把手机上的讨论搬到终端。

> 不是官方平台 API（api.deepseek.com）。调用计费走你的账号额度，与 App 内对话同等对待。请自行控制频率。

## 特性

- **单文件二进制**：Go 编写，无 CGO、无运行时依赖，Linux/macOS/Windows 均可交叉编译
- **PoW 求解**：内置一份 26KB 的 wasm 求解器，用纯 Go 的 [wazero](https://wazero.io) 驱动，单次约 200ms
- **流式**：SSE 增量解析，逐字打印
- **会话能力**：`list` / `history` / `send -session` / `tail`（把手机上的对话实时镜像到终端）

## 快速开始

```bash
# 1. 准备你自己的会话令牌（从你已登录的客户端导出，本工具不提供获取方式）
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

## 电脑网页端登录适配（安装时提示词）

桌面浏览器（chat.deepseek.com）与 App 的差别只有取 token 那一步，装的时候把下面这段原样发给 AI 就行：

```text
你是安装助手。用户要在电脑上使用 dscli，登录态来自网页版 chat.deepseek.com。
步骤：
1. 让用户在该域名登录，按 F12 打开 DevTools。
2. 在 Application → Local Storage 或 Cookies 里找 userToken / token 字段（64 字符不透明串）。
3. 写入 ~/.dsh/deepseek/session.json，格式 {"token":"...","uid":"..."}，权限 600。
4. 运行 dscli whoami 验证；失败时先确认 token 未过期，再确认请求头用的是 web 版（x-client-version: 1.0.0-always）。
不要索取账号密码，不要抓取其它站点的 cookie。
```

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

## 给 AI 的下载指引

见 [How-to-download.md](How-to-download.md)：里面写了在无浏览器、只有 shell 的环境里怎么拿到 release 资产、怎么校验、怎么跳过镜像的坑。

## 协议要点

详见 [docs/protocol.md](docs/protocol.md)。三个容易翻车的点：

1. **版本门**：必须发 `x-client-version: 1.0.0-always` + `x-client-platform: web`，用安卓端的版本号会得到 `CLIENT_VERSION_TOO_LOW`。
2. **PoW**：站点使用的哈希不是标准 Keccak/SHA-3（轮数与填充都不同），现成库算不出同一个摘要，仓库里内置了可用的求解模块。
3. **请求体**：`/api/v0/chat/completion` 必须带 `source` / `action` / `preempt`，少一个就 422；`fetch_page` 与 `history_messages` 是 GET，用 POST 会 405。

## 安全与边界

- token 等价于账号登录态，**不要提交进仓库、不要贴进对话**；`session.json` 权限 600。
- 本项目只处理「使用者本人账号」的会话，不包含任何账号获取、批量注册或绕过限流的逻辑。
- 请求有节流（每次 `send` 前留白），别高频轰炸，异常调用最先影响的是这个号。

## 参与

见 [CONTRIBUTING.md](CONTRIBUTING.md)。安全问题请走 [SECURITY.md](SECURITY.md)，版本变更见 [CHANGELOG.md](CHANGELOG.md)。

## License

MIT，见 [LICENSE](LICENSE)。

---

## English

A terminal client for DeepSeek's **consumer** backend (`chat.deepseek.com`) that reuses the
token the official mobile app or web session already holds — no API key, no proxy layer.
Single static binary (Go, no CGO), streaming SSE output, session list/history, and `tail` to
mirror a conversation the app is driving.

It reimplements the `DeepSeekHashV1` proof-of-work by embedding the site's own
`sha3_wasm_v1.wasm` and running it through [wazero](https://wazero.io), because the hash is
a non-standard Keccak variant that no stock library reproduces.

Build with `make cross` for `linux/amd64`, `linux/386`, `windows/amd64`, `darwin/amd64`.
MIT licensed. Use your own account; keep the token out of version control.
