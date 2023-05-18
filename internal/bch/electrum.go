package bch

import (
	"context"
	"github.com/checksum0/go-electrum/electrum"
)

func (b *Bch) ListUnspent(ctx context.Context, newAddress string) ([]*electrum.ListUnspentResult, error) {
	addr, err := b.tools.NewToOldAddress(newAddress)
	if err != nil {
		return nil, err
	}
	scripthash, err := electrum.AddressToElectrumScriptHash(addr)
	if err != nil {
		return nil, err
	}

	best := b.Nodes.GetBestBlockNode()
	out, err := best.GetElectrumClient().ListUnspent(ctx, scripthash)
	if err == electrum.ErrServerShutdown {
		b.reconnectElectrumOnError(err)
		return b.ListUnspent(ctx, newAddress)
	}
	return out, err
}

func (b *Bch) GetTransaction(ctx context.Context, txHash string) (*electrum.GetTransactionResult, error) {
	best := b.Nodes.GetBestBlockNode()
	res, err := best.GetElectrumClient().GetTransaction(ctx, txHash)
	if err == electrum.ErrServerShutdown {
		b.reconnectElectrumOnError(err)
		return b.GetTransaction(ctx, txHash)
	}

	return res, err
}

func (b *Bch) reconnectElectrumOnError(err error) {
	if err == nil || err != electrum.ErrServerShutdown {
		return
	}

	b.logger.Errorf("Fulcrum connection closed, reconnecting: %v", err)

	// easy way: simple re-connect all
	for _, node := range b.Nodes.Nodes {
		node.electrumClient, err = electrum.NewClientTCP(b.ctx, node.Fulcrum)
		if err != nil {
			b.logger.Errorf("Error re-connecting electrum client: %v", err)
			// we will try to re-connect again on next failed ping
		}
	}
}
