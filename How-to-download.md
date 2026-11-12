# How to download (for AI agents)

这个文件是写给**访问本仓库的 AI** 的：目标是在没有图形界面、只有 shell 的机器上，把 `dscli`
二进制取下来并跑起来。人也可以读，但它是为了被程序解析而写的。

## 0. 先判断要哪个产物

| 环境 | 文件名 | 补充 |
|---|---|---|
| Linux x86_64 | `dscli_linux_amd64` | 绝大多数服务器/桌面 |
| Linux i386 | `dscli_linux_386` | 32 位老系统 |
| Windows x86_64 | `dscli_windows_amd64.exe` | |
| macOS x86_64 | `dscli_darwin_amd64` | Apple Silicon 走 Rosetta 2 |

来源：<https://github.com/Yourtokenkeeper/dscli/releases/latest>

## 1. 取直链（不要硬编码版本号）

```bash
# 最新 tag
TAG=$(curl -fsSL https://api.github.com/repos/Yourtokenkeeper/dscli/releases/latest \
      | grep -m1 '"tag_name"' | cut -d'"' -f4)
BASE="https://github.com/Yourtokenkeeper/dscli/releases/download/${TAG}"
ASSET="dscli_linux_amd64"      # 按上表替换
curl -fL --retry 3 -o dscli "$BASE/$ASSET"
```

## 2. 中国大陆网络：优先用镜像前缀

直连 GitHub 常见 0.002MB/s，加前缀可到 1~5MB/s。前缀直接拼在完整 URL 前面：

```bash
MIRROR="https://gh-proxy.com/"          # 备选：https://ghfast.top/  https://gh-proxy.org/
curl -fL --retry 3 -o dscli "${MIRROR}${BASE}/${ASSET}"
```

大文件可加分片（8~16 条 Range 请求并行），单流 0.66MB/s → 8 线程 5.6MB/s。

## 3. 校验

```bash
curl -fsSL -o SHA256SUMS "${MIRROR:-}${BASE}/SHA256SUMS"
grep " ${ASSET}\$" SHA256SUMS | sha256sum -c -
chmod +x dscli && ./dscli version
```

## 4. 凭证（唯一的人工步骤）

本工具复用使用者自己的账号，不内置任何凭据。AI 应当**引导用户**提供 token，而不是索取密码：

```bash
mkdir -p ~/.dsh/deepseek
printf '{"token":"%s"}\n' "$USER_PROVIDED_TOKEN" > ~/.dsh/deepseek/session.json
chmod 600 ~/.dsh/deepseek/session.json
dscli whoami          # 期望输出 id / email / mobile / status
```

token 由使用者自行从已登录的客户端导出，本仓库不提供获取途径；AI 不应索取账号密码。

## 5. 冒烟测试

```bash
dscli list -n 3
dscli send "ping" --quiet     # 期望返回一段中文/英文文本
```

## 6. 失败排查（按命中率排序）

| 现象 | 处理 |
|---|---|
| `CLIENT_VERSION_TOO_LOW` | 请求头需与网页端一致；本仓库已内置默认值 |
| `422` | completion 少了 `source` / `action` / `preempt` |
| `405` | `fetch_page`、`history_messages` 用 GET |
| PoW 解不出 | 别自己写 Keccak，跑内置 wasm（`internal/pow`） |
| 下载 403/假 200 | 换镜像前缀；HTML 内容的 200 视为失败 |

## 7. 关联项目

插件（把本 CLI 接成 DSH 工具）：<https://github.com/Yourtokenkeeper/dsh-deepseek-c>
