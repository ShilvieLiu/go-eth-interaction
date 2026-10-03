# go-eth-interaction

> Go + Ethereum Interaction — 用 Go 语言与以太坊区块链交互的命令行工具

基于 [go-ethereum](https://github.com/ethereum/go-ethereum) 官方 Go 库开发，支持**区块查询**与 **ETH 转账**两种核心操作。

## 功能概览

| 模式 | 说明 | 命令示例 |
|------|------|----------|
| 区块查询 | 按区块号查询区块 hash、时间戳、交易数量 | `--blockNumber 12345` |
| ETH 转账 | 向指定地址发送 ETH（EIP-1559 动态费用） | `--send --to 0x... --amount 0.01` |

## 环境要求

- Go 1.25.7+
- 以太坊 JSON-RPC 节点（Infura / Alchemy / 本地节点等）

## 环境变量

| 变量名 | 必需 | 说明 |
|--------|------|------|
| `ETH_RPC_URL` | ✅ | 以太坊 RPC 节点地址，如 `https://sepolia.infura.io/v3/YOUR_KEY` |
| `SENDER_PRIVATE_KEY` | 转账时 | 发送方钱包私钥（支持 `0x` 前缀） |

## 安装

```bash
cd go-eth-interaction
go mod download
```

## 运行方式

支持两种运行方式，功能完全相同：

### 方式一：`go run`（直接运行，无需编译）

适合开发调试，修改代码后立即生效：

```bash
# 查询区块
go run main.go --blockNumber 19000000

# 发送转账
go run main.go --send --to 0xRecipientAddress --amount 0.001
```

### 方式二：`go build`（先编译，再运行）

适合重复使用，编译一次后可多次执行，启动更快：

```bash
# 编译为可执行文件
go build -o go-eth-interaction .

# 查询区块
./go-eth-interaction --blockNumber 19000000

# 发送转账
./go-eth-interaction --send --to 0xRecipientAddress --amount 0.001
```

## 使用方法

### 查询区块信息

```bash
export ETH_RPC_URL="https://sepolia.infura.io/v3/YOUR_KEY"

# 查询创世区块（默认区块号 0）
go run main.go
# 或
./go-eth-interaction

# 查询指定区块
go run main.go --blockNumber 19000000
# 或
./go-eth-interaction --blockNumber 19000000
```

输出示例：

```
ETH_RPC_URL is set to: https://sepolia.infura.io/v3/...
Fetched block: 19000000
- block hash: 0x...
- block timestamp (UTC): 2024-01-01T12:00:00Z
- block timestamp (Local): 2024-01-01 20:00:00 CST
- block timestamp (Unix Timestamp): 1704100800
- transaction count (from block): 150
- transaction count (from client): 150
```

### 发送 ETH 转账

```bash
export ETH_RPC_URL="https://sepolia.infura.io/v3/YOUR_KEY"
export SENDER_PRIVATE_KEY="你的私钥"

go run main.go --send --to 0xRecipientAddress --amount 0.001
# 或
./go-eth-interaction --send --to 0xRecipientAddress --amount 0.001
```

输出示例：

```
ETH_RPC_URL is set to: https://sepolia.infura.io/v3/...
Sender address: 0x...
Recipient address: 0x...
Chain ID: 11155111
Pending nonce: 42
Suggested gas tip cap: 2000000000
Base fee (from latest block): 15000000000
Final base fee: 15000000000
Final gas fee cap: 32000000000
Gas limit: 21000
Total cost (wei): 21000000000000
Sender balance (wei): 1000000000000000000
Transaction sent! Hash: 0x...
```

## 命令行参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--blockNumber` | uint64 | 0 | 查询的区块号（0 = 创世区块） |
| `--send` | bool | false | 启用转账模式 |
| `--to` | string | "" | 接收方地址（转账模式必需） |
| `--amount` | float64 | 0 | 转账金额，单位 ETH（转账模式必需） |

## 技术细节

### 转账流程

```
私钥 → 推导地址 → 获取 nonce → 获取 Gas 参数 → 余额校验 → 构造交易 → 签名 → 广播
```

1. 从环境变量读取私钥，推导出发送方地址
2. 连接 RPC 节点，获取链 ID 和账户 nonce
3. 获取 EIP-1559 动态费用参数（`GasTipCap`、`BaseFee`）
4. 计算 `GasFeeCap = BaseFee × 2 + GasTipCap`（2 倍 BaseFee 提高打包概率）
5. 校验发送方余额 ≥ 转账金额 + Gas 费用
6. 构造 `DynamicFeeTx` 并用 London 签名器签名
7. 广播交易，输出交易 Hash

### 上下文管理

- 所有 RPC 调用使用 10 秒超时（`context.WithTimeout`）
- `dialEthClient` 返回 `(ctx, cancel, client)`，由调用者通过 `defer` 管理生命周期
- 连接失败时主动调用 `cancel()` 释放资源

### Gas 费用策略

| 参数 | 策略 |
|------|------|
| Gas Limit | 普通转账固定 21000 |
| GasTipCap | `SuggestGasTipCap` 节点建议值 |
| GasFeeCap | `BaseFee × 2 + GasTipCap` |

若链不支持 EIP-1559（`BaseFee` 为 nil），自动回退到传统 `SuggestGasPrice` 模式。

## 安全提醒

- 私钥仅通过环境变量传入，不会硬编码在代码中
- 使用完毕后执行 `unset SENDER_PRIVATE_KEY` 清除环境变量
- 切勿将私钥提交到 Git 仓库
- 建议先在测试网（Sepolia / Holesky）验证，再用于主网

## 项目结构

```
go-eth-interaction/
├── main.go      # CLI 入口 + 区块查询 + 转账逻辑
├── go.mod       # Go 模块定义
├── go.sum       # 依赖校验
└── README.md    # 本文档
```

## 依赖

- [go-ethereum v1.17.6](https://github.com/ethereum/go-ethereum) — 以太坊官方 Go 库，提供 RPC 客户端、交易构造与签名