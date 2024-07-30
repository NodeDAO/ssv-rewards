package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

var zeroAddr = common.HexToAddress("0x0000000000000000000000000000000000000000")

var nethToken = common.HexToAddress("0xC6572019548dfeBA782bA5a2093C836626C7789A")
var nethPoolV2 = common.HexToAddress("0xf3C79408164abFB6fD5dDfE33B084E4ad2C07c18")

var rnethToken = common.HexToAddress("0x9dc7e196092dac94f0c76cfb020b60fa75b97c5b")
var rnethPool = common.HexToAddress("0x0d6F764452CA43eB8bd22788C9Db43E4b5A725Bc")

var (
	rnethStartBlock uint64 = 19516980
	nethStartBlock  uint64 = 16683911 // 17979259
)

func GetEthClient(rpcHost string) (*ethclient.Client, func(), error) {
	if rpcHost == "" {
		return nil, nil, fmt.Errorf("config.yaml is missing 'ethrpc'")
	}

	client, err := ethclient.Dial(rpcHost)
	if err != nil {
		return nil, nil, err
	}

	return client, func() {
		client.Close()
	}, nil
}

var TransferTopic = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

type TransferEvent struct {
	BlockNumber uint64
	From        common.Address
	To          common.Address
	Amount      *big.Int
}

func ScanTokenInfo(startBlock uint64, rpcHost string, tokenAddr common.Address) ([]TransferEvent, error) {
	eth1Client, cancel, err := GetEthClient(rpcHost)
	if err != nil {
		return nil, err
	}
	defer cancel()

	curBlock, err := eth1Client.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}

	transferEvents := make([]TransferEvent, 0)
	for fromBlock := startBlock; fromBlock < curBlock; {
		nextBlock := fromBlock + 20000
		if nextBlock >= curBlock {
			nextBlock = curBlock
		}

		filter := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(fromBlock)),
			ToBlock:   big.NewInt(int64(nextBlock)),
			Addresses: []common.Address{tokenAddr},
			Topics:    [][]common.Hash{{TransferTopic}},
		}

		log.Infow("scan block", "fromBlock", fromBlock, "nextBlock", nextBlock)

		fromBlock = nextBlock

		addLogs, err := eth1Client.FilterLogs(context.Background(), filter)
		if err != nil {
			continue
		}

		for _, l := range addLogs {
			var from common.Address
			copy(from[:], l.Topics[1][12:])
			var to common.Address
			copy(to[:], l.Topics[2][12:])
			amount := big.NewInt(0).SetBytes(l.Data)
			transferEvents = append(transferEvents, TransferEvent{
				BlockNumber: l.BlockNumber,
				From:        from,
				To:          to,
				Amount:      amount,
			})
		}
	}

	return transferEvents, nil
}

var uniSwap = common.HexToAddress("0x24ad0af5999dd3ca3d5d9826d34a16b0cc135c83")
var zklink = common.HexToAddress("0xAd16eDCF7DEB7e90096A259c81269d811544B6B6")
