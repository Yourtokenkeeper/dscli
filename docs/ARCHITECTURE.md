# 结构说明

```
cmd/dscli/        命令行入口：参数解析 + 输出排版，不做协议细节
internal/api/     HTTP 层：头、信封解码、端点封装、SSE 增量解析
internal/pow/     PoW：内嵌 wasm + wazero 驱动；keccak.go 为家族参考实现
internal/config/  凭证加载（旗标 → env → 默认路径）
docs/             协议笔记与结构说明
```

## 一次 `send` 的调用链

```
cmd.dscli send
  └─ config.Load            读 token
  └─ api.NewSession         没有 -session 时先建会话
  └─ api.powHeader          POST create_pow_challenge → pow.Solve(wasm) → base64
  └─ api.Ask                POST chat/completion（带 x-ds-pow-response）
       └─ consumeSSE        逐帧解析：种子帧 / 增量帧 / BATCH 帧
            └─ OnText       回调给命令行逐字打印
```

## 为什么 PoW 要跑 wasm

`DeepSeekHashV1` 是站点改造过的 Keccak 变体：轮数、填充、输入拼装都与标准不同，
用任何一个现成库都算不出同一个摘要（见 `keccak.go` 里的实验记录）。
与其逆向置换表，不如直接跑官方模块——26KB，wazero 解释执行，单次求解约 200ms，
换来的是「无需 CGO、任意平台交叉编译、行为与服务端逐位一致」。

代价：二进制多约 8MB；wasm 以 `go:embed` 打进产物，运行时不落地临时文件。

## 可测试性设计

- `api.BaseURL` 是变量，测试用 `httptest` 指过去
- `consumeSSE` 接受任意 `io.Reader`，不依赖网络
- `pow.Keccak256Std` 保留标准向量测试，防止置换表被改坏
- 所有网络错误向上返回，不在库代码里 `panic` 或 `os.Exit`
