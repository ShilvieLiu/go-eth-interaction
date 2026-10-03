package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 支持两种操作模式：
// 1. 查询交易：--blockNum <number> 按区块号查询该区块相关信息
// 2. 发送交易：--send --to <address> --amount <eth> 发起 ETH 转账交易
func main() {
	blockNumber := flag.Uint64("blockNumber", 0, "block number to fetch (default: 0 for genesis block)")
	send := flag.Bool("send", false, "send transaction to another address")
	to := flag.String("to", "", "recipient address for the transaction (required if -send is true)")
	amount := flag.Float64("amount", 0, "amount in ETH (required if -send is true)")
	flag.Parse()

	ethRpcUrl := os.Getenv("ETH_RPC_URL")
	if ethRpcUrl == "" {
		log.Fatal("ETH_RPC_URL is not set")
	}
	fmt.Println("ETH_RPC_URL is set to:", ethRpcUrl)

	if *send {
		// Send transaction to another address
		if *to == "" || *amount <= 0 {
			log.Fatal("Recipient address (-to) and amount (-amount) must be provided when -send is true")
		}

		sendTransaction(ethRpcUrl, *to, *amount)

	} else {
		// Fetch block by number & Print block details
		queryBlockByNumber(ethRpcUrl, *blockNumber)
	}

}

func sendTransaction(ethRpcUrl, toAddress string, amount float64) {
	senderPrivateKey := os.Getenv("SENDER_PRIVATE_KEY")
	if senderPrivateKey == "" {
		log.Fatal("SENDER_PRIVATE_KEY is not set")
	}

	privateKey, err := crypto.HexToECDSA(trim0x(senderPrivateKey))
	if err != nil {
		log.Fatal("Failed to convert private key:", err)
	}

	publicKey, ok := privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("Failed to get public key from private key")
	}

	fromAddr := crypto.PubkeyToAddress(*publicKey)
	fmt.Println("Sender address:", fromAddr.Hex())

	toAddr := common.HexToAddress(toAddress)
	fmt.Println("Recipient address:", toAddr.Hex())

	ctx, cancel, client := dialEthClient(ethRpcUrl)
	defer cancel()
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatal("Failed to fetch chain ID:", err)
	}
	fmt.Println("Chain ID:", chainID)

	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		log.Fatal("Failed to fetch pending nonce:", err)
	}
	fmt.Println("Pending nonce:", nonce)

	// 获取建议的 Gas 价格（使用 EIP-1559 动态费用）
	tipCap, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		log.Fatal("Failed to fetch suggested gas tip cap:", err)
	}
	fmt.Println("Suggested gas tip cap:", tipCap)

	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatal("Failed to fetch latest block header:", err)
	}

	baseFee := header.BaseFee
	fmt.Println("Base fee (from latest block):", baseFee)
	if baseFee == nil {
		// 如果不支持 EIP-1559，使用传统 gas price
		gasPrice, err := client.SuggestGasPrice(ctx)
		if err != nil {
			log.Fatalf("failed to get gas price: %v", err)
		}
		baseFee = gasPrice
	}
	fmt.Println("Final base fee:", baseFee)

	// fee cap = base fee * 2 + tip cap（简单策略）
	gasFeeCap := new(big.Int).Add(
		new(big.Int).Mul(baseFee, big.NewInt(2)),
		tipCap,
	)
	fmt.Println("Final gas fee cap:", gasFeeCap)

	// 估算 Gas Limit（普通转账固定为 21000）
	gasLimit := uint64(21000)
	fmt.Println("Gas limit:", gasLimit)

	weiAmount, _ := new(big.Float).Mul(big.NewFloat(amount), big.NewFloat(1e18)).Int(nil) // Convert ETH to Wei

	// 计算总费用：weiAmount + gasFeeCap * gasLimit
	totalCost := new(big.Int).Add(weiAmount, new(big.Int).Mul(gasFeeCap, big.NewInt(int64(gasLimit))))
	fmt.Println("Total cost (wei):", totalCost)

	balance, err := client.BalanceAt(ctx, fromAddr, nil)
	if err != nil {
		log.Fatal("Failed to fetch balance:", err)
	}
	fmt.Println("Sender balance (wei):", balance)

	if balance.Cmp(totalCost) < 0 {
		log.Fatalf("Insufficient funds: balance %s wei, required %s wei", balance.String(), totalCost.String())
	}

	// 创建交易
	txData := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: tipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        &toAddr,
		Value:     weiAmount,
		Data:      nil,
	}
	tx := types.NewTx(txData)

	// 签名交易
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), privateKey)
	if err != nil {
		log.Fatal("Failed to sign transaction:", err)
	}

	// 发送交易
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		log.Fatal("Failed to send transaction:", err)
	}
	fmt.Println("Transaction sent! Hash:", signedTx.Hash().Hex())
}

// trim0x 移除十六进制字符串前缀 "0x"
func trim0x(s string) string {
	if len(s) >= 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}

func dialEthClient(ethRpcUrl string) (context.Context, context.CancelFunc, *ethclient.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	client, err := ethclient.DialContext(ctx, ethRpcUrl)
	if err != nil {
		cancel()
		log.Fatal("Failed to connect to Ethereum client:", err)
	}

	return ctx, cancel, client
}

func queryBlockByNumber(ethRpcUrl string, blockNumber uint64) {
	ctx, cancel, client := dialEthClient(ethRpcUrl)
	defer cancel()
	defer client.Close()

	num := big.NewInt(0)
	if blockNumber > 0 {
		num = num.SetUint64(blockNumber)
	}

	block, err := client.BlockByNumber(ctx, num)
	if err != nil {
		log.Fatal("Failed to fetch block:", err)
	}
	fmt.Println("Fetched block:", block.Number())

	// 区块 hash
	fmt.Println("- block hash:", block.Hash().Hex())

	// 区块时间戳
	blockTime := time.Unix(int64(block.Time()), 0)
	fmt.Println("- block timestamp (UTC):", blockTime.Format(time.RFC3339))
	fmt.Println("- block timestamp (Local):", blockTime.Local().Format("2006-01-02 15:04:05 MST"))
	fmt.Println("- block timestamp (Unix Timestamp):", block.Time())

	// 区块交易数量
	txCount1 := block.Transactions().Len()
	fmt.Println("- transaction count (from block):", txCount1)

	txCount2, err := client.TransactionCount(ctx, block.Hash())
	if err != nil {
		log.Fatal("Failed to fetch transaction count:", err)
	}
	fmt.Println("- transaction count (from client):", txCount2)
}
