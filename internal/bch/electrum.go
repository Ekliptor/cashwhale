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
	return out, err
}

func (b *Bch) GetTransaction(ctx context.Context, txHash string) (*electrum.GetTransactionResult, error) {
	best := b.Nodes.GetBestBlockNode()
	return best.GetElectrumClient().GetTransaction(ctx, txHash)
}
