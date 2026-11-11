# 协议笔记：chat.deepseek.com

本文记录实现 `dscli` 时逆出来的要点，供维护者对照。所有路径都相对于
`https://chat.deepseek.com`，响应统一包在 `{code,msg,data:{biz_code,biz_msg,biz_data}}` 里。

## 1. 请求头

```
accept: */*
content-type: application/json
user-agent: <移动浏览器 UA>
x-client-platform: web
x-client-version: 1.0.0-always
x-app-version: 20241129.1
x-client-locale: zh_CN
x-client-timezone-offset: 28800
authorization: Bearer <token>
x-ds-pow-response: <base64(poW json)>     # 仅完成/上传接口
```

**版本门**：把 `x-client-version` 换成安卓端的 `2.7.5` 会返回
`{"code":40005,"msg":"CLIENT_VERSION_TOO_LOW"}`。web 的 `1.0.0-always` 放行。

## 2. 鉴权

`GET /api/v0/users/current` 验证 token，返回 `id / email / mobile_number / status`。

token 是 64 字符不透明串，App 存在
`files/mmkv/mmkv.default` 的 `key_user_info` 里（`{"token":"...","id":"..."}`）。

## 3. 工作量证明

```
POST /api/v0/chat/create_pow_challenge   {"target_path": "/api/v0/chat/completion"}
→ biz_data.challenge = {algorithm, challenge, salt, difficulty, expire_at, signature}
```

- `algorithm` 为 `DeepSeekHashV1`，`difficulty` 常见 144000。
- 需要找 nonce ∈ [0, difficulty)，使站点自定义哈希满足挑战；**它不是标准
  Keccak-256，也不是 SHA3-256**：`internal/pow/keccak.go` 里的干净实现在
  20~24 轮 × 两种填充下都对不上，最终确认必须跑站点自带的
  `sha3_wasm_bg.*.wasm`（wasm-bindgen 模块，26KB），本仓库内嵌并复刻其 ABI：

```
wasm_solve(retPtr, chPtr, chLen, prefixPtr, prefixLen, difficulty:f64)
retPtr+0: i32 status (0 = 无解)
retPtr+8: f64 answer
prefix  = "<salt>_<expire_at>_"
```

- 求得的 nonce 打包成 JSON，base64 后放进 `x-ds-pow-response`：

```json
{"algorithm":"DeepSeekHashV1","challenge":"…","salt":"…","answer":99349,"signature":"…","target_path":"/api/v0/chat/completion"}
```

- 单次求解在本机实测约 200ms（arm64，单核）。

## 4. 会话

```
POST /api/v0/chat_session/create        {}               → biz_data.chat_session.id
GET  /api/v0/chat_session/fetch_page?count=10            → biz_data.chat_sessions[]
GET  /api/v0/chat/history_messages?chat_session_id=…     → biz_data.chat_messages[]
```

后两个必须用 GET；用 POST 会得到 `405 Method Not Allowed`。

## 5. 发消息（SSE）

```
POST /api/v0/chat/completion
{
  "chat_session_id": "…",
  "parent_message_id": null,
  "model_type": "default" | "expert",
  "prompt": "…",
  "ref_file_ids": [],
  "thinking_enabled": false,
  "search_enabled": false,
  "source": "default",
  "action": null,
  "preempt": false
}
```

少 `source` / `action` / `preempt` 任一字段 → `422 Unprocessable Entity`。

响应是 `text/event-stream`，帧形如：

```
event: ready
data: {"request_message_id":1,"response_message_id":2,"model_type":"default"}

data: {"v":{"response":{…,"fragments":[{"type":"RESPONSE","content":"生活"}]}}}

data: {"p":"response/fragments/-1/content","o":"APPEND","v":"不是"}
data: {"v":"等待"}                                  ← 无 p：延续上一条路径
data: {"p":"response","o":"BATCH","v":[{"p":"accumulated_token_usage","v":46}]}
```

解析规则：

1. `p` 存在 → 更新当前路径；`p` 缺失 → 沿用上一条路径。
2. `o=BATCH` → `v` 是子帧数组，逐条递归处理。
3. 首个 `{"v":{"response":…}}` 是整包快照（含已生成的前缀），要当作种子文本，
   之后的增量才追加；忽略它会吞掉回复开头几个字。
4. 路径含 `thinking` 的进思考流，含 `content` 的进正文流。

## 6. 其它（未实现，留作扩展）

| 接口 | 用途 |
|---|---|
| `POST /api/v0/file/upload_file` | 附件上传（同样需要 PoW，`target_path` 换成该路径） |
| `POST /api/v0/chat/stop_stream` | 中止生成 |
| `POST /api/v0/chat/regenerate` / `edit_message` | 重新生成 / 编辑 |
| `POST /api/v0/share/create` | 分享会话 |
| `GET  /api/v0/download_export_history` | 导出全部历史 |

## 7. 免责

这些接口由官方客户端使用，未公开承诺兼容性；字段与路径随时可能变。本项目仅用于
在自己账号上把已登录的会话搬到终端，请遵守服务条款并控制调用频率。
