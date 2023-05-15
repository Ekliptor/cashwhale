package chaintools

import (
	"context"
	"github.com/checksum0/go-electrum/electrum"
	"log"
	"testing"
)

func TestElectrum(t *testing.T) {

	client, err := electrum.NewClientTCP(context.Background(), "bch.prompt.cash:60001")

	if err != nil {
		log.Fatal(err)
	}

	serverVer, protocolVer, err := client.ServerVersion(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Server version: %s [Protocol %s]", serverVer, protocolVer)

	tools, err := NewChainTools()
	addr1, err := tools.NewToOldAddress("qpkjwn82pz3uj2jdzhv43tmaej2ftzyp8qe0er6ld4")
	scripthash, _ := electrum.AddressToElectrumScriptHash(addr1)
	unspent, err := client.ListUnspent(context.Background(), scripthash)

	res, err := client.GetTransaction(context.Background(), unspent[0].Hash)

	_ = res
	_ = unspent

	// Asking the server for the balance of address 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
	// 8b01df4e368ea28f8dc0423bcf7a4923e3a12d307c875e47a0cfbf90b5c39161
	// We must use scripthash of the address now as explained in ElectrumX docs
	scripthash, _ = electrum.AddressToElectrumScriptHash("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	balance, err := client.GetBalance(context.Background(), scripthash)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Address confirmed balance:   %+v", balance.Confirmed)
	log.Printf("Address unconfirmed balance: %+v", balance.Unconfirmed)

	/*go func() {
		for {
			if err := client.Ping(context.Background()); err != nil {
				log.Fatal(err)
			}
			time.Sleep(60 * time.Second)
		}
	}()*/
}
