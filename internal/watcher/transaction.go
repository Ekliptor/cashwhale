package watcher

import "github.com/prompt-cash/go-bitcoin"

func getInputValue(tx *bitcoin.RawTransaction) float32 {
	var total float32 = 0
	inputs := tx.Vin
	for _, in := range inputs {
		total += in.Prevout.Value
	}
	return total
}

func getOutputValue(tx *bitcoin.RawTransaction) float32 {
	var total float32 = 0
	outputs := tx.Vout
	for _, out := range outputs {
		total += float32(out.Value)
	}
	return total
}

func getTransactionFeeBCH(tx *bitcoin.RawTransaction) float32 {
	// https://en.bitcoin.it/wiki/Transaction#Input
	inValue := getInputValue(tx)
	outValue := getOutputValue(tx)
	return inValue - outValue
}

func getTransactionFee(tx *bitcoin.RawTransaction) int64 {
	return int64(getTransactionFeeBCH(tx) * 100000000.0)
}
