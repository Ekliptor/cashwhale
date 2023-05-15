package bch

import (
	"github.com/checksum0/go-electrum/electrum"
	"github.com/prompt-cash/go-bitcoin"
	"time"
)

type Nodes struct {
	Nodes []*Node
}

func (nodes *Nodes) GetBestBlockNode() *Node {
	var best *Node
	for _, node := range nodes.Nodes {
		if best == nil || best.stats == nil || node.stats.BlockHeight.BlockNumber > best.stats.BlockHeight.BlockNumber {
			best = node
		}
	}

	return best
}

type Node struct {
	// config
	Address  string `mapstructure:"Address"`
	User     string `mapstructure:"User"`
	Password string `mapstructure:"Password"`
	SSL      bool   `mapstructure:"SSL"`

	Fulcrum        string `mapstructure:"Fulcrum"`
	FulcrumPingMin int    `mapstructure:"FulcrumPingMin"`

	bchClient      *bitcoin.Bitcoind
	electrumClient *electrum.Client

	stats *NodeStats
}

// Node stats available via HTTP API as JSON
type NodeStats struct {
	Connected       time.Time           `json:"connected"`
	BlockHeight     BestBlockHeight     `json:"block_height"`
	LostConnections []*NodeConnectError `json:"lost_connections"`
	LastNotified    time.Time           `json:"last_notified"`
}

type BestBlockHeight struct {
	BlockNumber uint32    `json:"block_number"`
	BlockTime   time.Time `json:"block_time"`
	Received    time.Time `json:"received"` // when WE saw this node got this block
}

type NodeConnectError struct {
	When  time.Time `json:"when"`
	Error error     `json:"error"`
}

func (n *Node) SetConnected() {
	if n.stats.Connected.IsZero() {
		n.stats.Connected = time.Now()
	}
	//n.updateStats()
}

func (n *Node) SetBlockHeight(blockHeight uint32, timestamp time.Time) {
	n.stats.BlockHeight = BestBlockHeight{
		BlockNumber: blockHeight,
		BlockTime:   timestamp,
		Received:    time.Now(),
	}
	//n.updateStats()
}

func (n *Node) GetBchClient() *bitcoin.Bitcoind {
	return n.bchClient
}

func (n *Node) GetElectrumClient() *electrum.Client {
	return n.electrumClient
}
