package bch

import (
	"context"
	"github.com/Ekliptor/cashwhale/internal/bch/chaintools"
	"github.com/Ekliptor/cashwhale/internal/log"
	"github.com/checksum0/go-electrum/electrum"
	"github.com/pkg/errors"
	"github.com/prompt-cash/go-bitcoin"
	"github.com/spf13/viper"
	"net"
	"strconv"
	"time"
)

const DustLimit = 546

type Bch struct {
	Nodes *Nodes
	tools *chaintools.ChainTools

	logger log.Logger
	ctx    context.Context
}

func NewBch(ctx context.Context, logger log.Logger) (*Bch, error) {
	bch := &Bch{
		Nodes: nil,
		logger: logger.WithFields(log.Fields{
			"module": "bch",
		}),
		ctx: ctx,
	}
	err := bch.loadNodeConfig()
	if err != nil {
		return nil, err
	}

	for _, node := range bch.Nodes.Nodes {
		host, port, err := net.SplitHostPort(node.Address)
		if err != nil {
			logger.Fatalf("Error parsing bitcoin node address: %v", err)
		}
		nport, _ := strconv.Atoi(port)

		node.bchClient, err = bitcoin.New(host, nport, node.User, node.Password, node.SSL)
		if err != nil {
			logger.Fatalf("Error creating bitcoin client: %v", err)
		}

		node.electrumClient, err = electrum.NewClientTCP(bch.ctx, node.Fulcrum)
		if err != nil {
			logger.Fatalf("Error creating electrum client: %v", err)
		}
	}

	bch.updateNodeStats()
	bch.tools, err = chaintools.NewChainTools()
	if err != nil {
		logger.Errorf("Error creating chaintools: %v", err)
	}

	return bch, nil
}

//func (b *Bch) GetAddressUnspentOutputs(address string, includeMemPool bool) ([]*electrum.ListUnspentResult, error) {
//	// todo: includeMemPool
//	return b.ListUnspent(b.ctx, address)
//}

func (b *Bch) updateNodeStats() {
	for _, node := range b.Nodes.Nodes {
		info, err := node.bchClient.GetBlockchainInfo()
		if err != nil {
			b.logger.Errorf("Error getting chain info: %v", err)
			continue
		}

		node.SetBlockHeight(uint32(info.Blocks), time.Now())
		node.SetConnected()
	}
}

// StartNodeTimer pulls data from nodes.
func (b *Bch) StartNodeTimer() {
	go (func() {
		tickerInterval := time.Duration(1) * time.Minute
		var ticker = time.NewTicker(tickerInterval)
		terminating := false
		for !terminating {
			select {
			case <-ticker.C:
				b.updateNodeStats()

			case <-b.ctx.Done():
				terminating = true
				break
			}
		}
	})()
}

// WatchNewBlocks is a blocking call to wait for new block headers.
func (b *Bch) WatchNewBlocks(ctx context.Context) (<-chan *bitcoin.BlockHeaderAndCoinbase, error) {
	best := b.Nodes.GetBestBlockNode()

	// https://electrum.readthedocs.io/en/latest/protocol.html#blockchain-headers-subscribe
	// https://docs.bitcoincashnode.org/doc/json-rpc/getblock/
	headerCh, err := best.electrumClient.SubscribeHeaders(b.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error opening channel to read new blocks")
	}

	respChan := make(chan *bitcoin.BlockHeaderAndCoinbase, 1)
	go (func() {
		var pingTicker = time.NewTicker(time.Minute * time.Duration(best.FulcrumPingMin))
		terminating := false
		for !terminating {
			select {
			case header := <-headerCh:
				b.logger.Debugf("Found new block at height %d", header.Height)

				blockHash, err := best.bchClient.GetBlockHash(int(header.Height))
				if err != nil {
					b.logger.Errorf("Error getting block hash: %v", err)
					continue
				}

				// header.Hex is len 160, we expect len 64. both are HEX. So header.Hex is the raw header data?
				//block, err := best.bchClient.GetBlock(blockHash) // doesn't include full TX
				block, err := best.bchClient.GetBlockHeaderAndCoinbase(blockHash)
				if err != nil {
					b.logger.Errorf("Error getting block: %v", err)
					continue
				}
				b.logger.Debugf("Block %d has %d transactions", header.Height, len(block.Tx))
				respChan <- block

			case <-pingTicker.C:
				// ping fulcrum to keep connection alive
				err := best.electrumClient.Ping(ctx)
				b.reconnectElectrumOnError(err)

			case <-ctx.Done():
				terminating = true
			}
		}
	})()

	return respChan, nil
}

func (b *Bch) GetBestAddress() string {
	best := b.Nodes.GetBestBlockNode()
	protocol := "http"
	if best.SSL {
		protocol = "https"
	}
	return protocol + "://" + best.Address + "/rest/"
}

func (b *Bch) loadNodeConfig() error {
	b.Nodes = &Nodes{}
	err := viper.UnmarshalKey("BCH", b.Nodes)
	if err != nil {
		return errors.Wrap(err, "error loading BCH nodes from config")
	}

	for _, node := range b.Nodes.Nodes {
		//node.monitor = b.monitor
		node.stats = &NodeStats{}
	}

	return nil
}
