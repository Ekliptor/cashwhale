package txcounter

import (
	"container/heap"
	"context"
	"encoding/gob"
	"github.com/Ekliptor/cashwhale/internal/log"
	"github.com/Ekliptor/cashwhale/internal/monitoring"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"os"
	"time"
)

// A struct counting the average TX size (in BCH) over a specified
// time period (such as 24h).
// This gives a dynamic threshold to identify whales.
type TxCounter struct {
	config             TxCounterConfig
	transactionHistory []*TxCounterTransaction
	avgTxSize          float32

	ctx     context.Context
	logger  log.Logger
	monitor *monitoring.HttpMonitoring
}

type TxCounterConfig struct {
	AverageTime time.Duration // the time to go back and include TX for average size calculation
}

type TxCounterTransaction struct {
	SizeBch float32
	When    time.Time
}

func NewTxCounter(config *TxCounterConfig, ctx context.Context, logger log.Logger, monitor *monitoring.HttpMonitoring) (*TxCounter, error) {
	if config == nil {
		config = &TxCounterConfig{
			AverageTime: 24 * time.Hour,
		}
	}
	counter := &TxCounter{
		config:             *config,
		transactionHistory: make([]*TxCounterTransaction, 0, 10000),
		ctx:                ctx,
		logger:             logger,
		monitor:            monitor,
	}
	if err := counter.readTransactionsFile(); err != nil {
		return nil, err
	}
	return counter, nil
}

func (counter *TxCounter) AddTransaction(sizeBch float32) {
	counter.transactionHistory = append(counter.transactionHistory, &TxCounterTransaction{
		SizeBch: sizeBch,
		When:    time.Now(),
	})
	counter.calcAverageTransactionSize()
}

func (counter *TxCounter) GetAverageTransactionSize() float32 {
	return counter.avgTxSize
}

func (counter *TxCounter) GetUpperTransactionSizePercent(percent float32) float32 {
	// TODO add map with percent number as key and invalidate on AddTransaction()
	txCount := int(float32(len(counter.transactionHistory)) / 100.0 * percent)
	if txCount == 0 {
		txCount = 1
	}

	// add all TX to a heap
	transactionHeap := &TxHeap{}
	heap.Init(transactionHeap)
	for _, tx := range counter.transactionHistory {
		heap.Push(transactionHeap, tx)
	}

	// get the sum of top n TX and compute average
	var sum float32 = 0.0
	for i := 0; i < txCount && transactionHeap.Len() > 0; i++ {
		tx := heap.Pop(transactionHeap).(*TxCounterTransaction)
		sum += tx.SizeBch
	}
	return sum / float32(txCount)
}

func (counter *TxCounter) GetTransactionCount() int {
	return len(counter.transactionHistory)
}

func (counter *TxCounter) ScheduleCleanupTransactions() error {
	// start the cleanup timer
	tickerInterval := time.Duration(viper.GetInt("Average.AverageTxCleanupTimeMin")) * time.Minute
	var ticker = time.NewTicker(tickerInterval)
	terminating := false
	for !terminating {
		select {
		case _ = <-ticker.C:
			counter.logger.Infof("Start cleaning up old transaction data...")
			err := counter.cleanupOldTransactions()
			if err != nil {
				counter.logger.Errorf("Error cleaning up old transaction data %+v", err)
			}
			err = counter.WriteTransactionsFile()
			if err != nil {
				counter.logger.Errorf("Error writing transaction data to disk %+v", err)
			}

			counter.logger.Infof("Successfully cleaned up old transaction data.")

		case <-counter.ctx.Done():
			terminating = true
			break
		}
	}

	return nil
}

func (counter *TxCounter) WriteTransactionsFile() error {
	// create a new file
	file, err := os.Create(viper.GetString("Average.TxHistoryFile"))
	if err != nil {
		return errors.Wrap(err, "error opening file to write")
	}
	defer file.Close()

	// then write all to disk
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(counter.transactionHistory); err != nil {
		return errors.Wrap(err, "error encoding transaction data")
	}

	counter.logger.Infof("Successfully stored transaction history with %d transactions.", len(counter.transactionHistory))
	return nil
}

func (counter *TxCounter) cleanupOldTransactions() error {
	// create a new smaller slice and copy TX over
	capacity := int(float32(len(counter.transactionHistory))*0.9 + 1)
	transactions := make([]*TxCounterTransaction, 0, capacity)
	expiry := time.Now().Add(-1 * time.Duration(viper.GetInt("Average.TransactionAverageTimeH")) * time.Hour)
	for _, tx := range counter.transactionHistory {
		if tx.When.After(expiry) {
			transactions = append(transactions, tx)
		}
	}

	counter.transactionHistory = transactions
	return nil
}

func (counter *TxCounter) calcAverageTransactionSize() {
	size := len(counter.transactionHistory)
	if size == 0 {
		counter.avgTxSize = 0.0
		return
	}
	var sum float32 = 0.0
	for _, tx := range counter.transactionHistory {
		sum += tx.SizeBch
	}
	counter.avgTxSize = sum / float32(size)

	// monitoring
	counter.monitor.AddEvent("TxCount", size)
	counter.monitor.AddEvent("TxAvgBch", counter.avgTxSize)
	counter.monitor.AddEvent("TxUpperPercentBch", counter.GetUpperTransactionSizePercent(float32(viper.GetFloat64("Average.UpperTxPercent"))))
}

func (counter *TxCounter) readTransactionsFile() error {
	file, err := os.Open(viper.GetString("Average.TxHistoryFile"))
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "error opening existing transactions file")
		}
		return nil
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)

	//transactions := make([]*TxCounterTransaction, 0, 10000)
	if err := decoder.Decode(&counter.transactionHistory); err != nil {
		return errors.Wrap(err, "error decoding previously stored transaction data")
	}

	counter.logger.Infof("Loaded transaction history containing %d transactions.", len(counter.transactionHistory))
	counter.calcAverageTransactionSize()
	return nil
}
