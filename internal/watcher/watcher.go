package watcher

import (
	"fmt"
	"github.com/Ekliptor/cashwhale/internal/log"
	"github.com/Ekliptor/cashwhale/internal/monitoring"
	"github.com/Ekliptor/cashwhale/internal/social"
	"github.com/Ekliptor/cashwhale/pkg/notification"
	"github.com/Ekliptor/cashwhale/pkg/txcounter"
	"github.com/prompt-cash/go-bitcoin"
	"github.com/spf13/viper"
	"time"
)

type Watcher struct {
	counter    *txcounter.TxCounter
	monitor    *monitoring.HttpMonitoring
	msgBuilder *social.MessageBuilder
	logger     log.Logger
}

func NewWatcher(logger log.Logger, monitor *monitoring.HttpMonitoring, counter *txcounter.TxCounter, msgBuilder *social.MessageBuilder) (*Watcher, error) {
	watcher := &Watcher{
		counter:    counter,
		monitor:    monitor,
		msgBuilder: msgBuilder,
		logger:     logger,
	}

	// add dummy tweet so we always have a LastTweet value (in case we never start sending)
	watcher.monitor.AddEvent("LastTweet", monitoring.D{
		"msg": "never (connected)",
	})
	watcher.CheckLastTweetTime()

	return watcher, nil
}

// CheckTransaction will see if it's a big transaction to tweet about.
func (w *Watcher) CheckTransaction(tx *bitcoin.RawTransaction) {
	// check if it's a Coinbase TX
	inputs := tx.Vin
	if len(inputs) == 0 { // can't happen
		w.logger.Errorf("TX has 0 inputs. block height %d, hash (reversed) %s", tx.BlockHeight, tx.Hash)
		return
	}
	/*
		inputHash, err := chainhash.NewHash(inputs[0].GetOutpoint().GetHash())
		if err != nil {
			gc.logger.Errorf("Error getting input hash of TX %+v", err)
			continue
		} else if (inputHash.IsEqual(&chainhash.Hash{})) {
			continue // Coinbase TX has no input
		}
	*/

	// loop through TX outputs and find big transactions
	//fee := getTransactionFee(tx)
	fee := tx.Fee

	outputs := tx.Vout
	var amountBCH float64 = 0.0
	for _, out := range outputs {
		// TODO add a filter if outputAddress in [previousInputAddress, ...] and deduct it
		// last address is usually change address
		//amountBCH += price.SatoshiToBitcoin(out.GetValue())
		amountBCH += out.Value
	}
	w.counter.AddTransaction(float32(amountBCH))
	if amountBCH < viper.GetFloat64("Message.WahleThresholdBCH") {
		//if gc.counter.GetTransactionCount() < viper.GetInt("Average.MinTxCount") || amountBCH < float64(gc.counter.GetAverageTransactionSize()) * viper.GetFloat64("Average.AverageTxFactor") {
		if w.counter.GetTransactionCount() < viper.GetInt("Average.MinTxCount") || amountBCH < float64(w.counter.GetUpperTransactionSizePercent(float32(viper.GetFloat64("Average.UpperTxPercent")))) {
			return
		}
	}

	txData := &social.TransactionData{
		AmountBchRaw: amountBCH,
		//FeeBch:       price.SatoshiToBitcoin(fee),
		FeeBch: float64(fee),
		Hash:   tx.Hash,
	}
	err := w.msgBuilder.CreateMessage(txData)
	if err == nil {
		err = w.msgBuilder.SendMessage(txData)
		if err != nil {
			w.logger.Errorf("Error sending message %+v", err)
		} else if w.monitor != nil {
			w.monitor.AddEvent("LastTweet", monitoring.D{
				"msg": txData.Message,
				//"time": time.Now(), // "when" attribute is present in event map
			})
		}
	}
}

func (w *Watcher) CheckLastTweetTime() {
	lastTweet := w.monitor.GetEvent("LastTweet")
	if lastTweet == nil {
		w.logger.Errorf("No LastTweet event found. Monitoring will not work")
		return
	}

	lastTweetTime := time.Unix(lastTweet.When, 0)
	threshold := time.Duration(viper.GetInt("Monitoring.TweetThresholdH"))
	if lastTweetTime.Add(threshold * time.Hour).After(time.Now()) {
		return
	}

	// TODO move config loading to a better place. Errors won't happen often though
	notifier := make([]*notification.NotificationReceiver, 0, 5)
	err := viper.UnmarshalKey("Notify", &notifier)
	if err != nil {
		w.logger.Errorf("Error reading notifier config %+v", err)
		return
	}

	sendData := notification.NewNotification(fmt.Sprintf("%s tweets stopped", viper.GetString("App.Name")),
		fmt.Sprintf("Last tweet: %s ago", time.Since(lastTweetTime)))

	for _, notify := range notifier {
		_, err = notification.CreateAndSendNotification(sendData, notify)
		if err != nil {
			w.logger.Errorf("Error sending 'tweets stopped' notification %+v", err)
			return
		}
	}
}
