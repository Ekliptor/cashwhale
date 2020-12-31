package bchd

import pb "github.com/Ekliptor/cashwhale/internal/bchd/golang"

func getInputValue(tx *pb.Transaction) int64 {
	var total int64 = 0
	inputs := tx.GetInputs()
	for _, in := range inputs {
		total += in.GetValue()
	}
	return total
}

func getOutputValue(tx *pb.Transaction) int64 {
	var total int64 = 0
	outputs := tx.GetOutputs()
	for _, out := range outputs {
		total += out.GetValue()
	}
	return total
}

func getTransactionFee(tx *pb.Transaction) int64 {
	// https://en.bitcoin.it/wiki/Transaction#Input
	inValue := getInputValue(tx)
	outValue := getOutputValue(tx)
	return inValue - outValue
}
