package cmd

import (
	"context"
	"github.com/Ekliptor/cashwhale/internal/bch"
	"github.com/Ekliptor/cashwhale/internal/log"
	monitoring "github.com/Ekliptor/cashwhale/internal/monitoring"
	"github.com/Ekliptor/cashwhale/internal/social"
	"github.com/Ekliptor/cashwhale/internal/watcher"
	"github.com/Ekliptor/cashwhale/pkg/txcounter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sync"
	"time"
)

func init() {
	rootCmd.AddCommand(WatchCmd)
}

var WatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for Whales on the BitcoinCash network",
	Long: `This command will watch for large amounts of BCH being transferred on-chain.
		It will then tweet about these transactions.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		logger, err := getLogger()
		if err != nil {
			return err
		}

		// Create the app context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go listenExitCommand(logger, cancel)
		monitor := createMonitoringClient(ctx, logger)

		// start all main workers in separate goroutines
		var wg sync.WaitGroup

		if monitor != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				monitor.ListenHttp(ctx)
			}()
		}

		// create the avg TX size counter
		counter, err := txcounter.NewTxCounter(&txcounter.TxCounterConfig{
			AverageTime: time.Duration(viper.GetInt("Average.TransactionAverageTimeH")) * time.Hour,
		}, ctx, logger, monitor)
		if err != nil {
			logger.Fatalf("Error creating TX counter: %+v", err)
		}
		go counter.ScheduleCleanupTransactions()
		// TODO ctx.Done() should wait for file write

		// create the gRPC watch client
		wg.Add(1)
		go func() {
			defer wg.Done()
			watchTransactionsRest(counter, ctx, logger, monitor)
		}()

		wg.Wait()

		// cleanup on shutdown
		err = counter.WriteTransactionsFile()
		if err != nil {
			logger.Errorf("Error writing transaction data to disk on shutdown %+v", err)
		}

		return nil
	},
}

func watchTransactionsRest(counter *txcounter.TxCounter, ctx context.Context, logger log.Logger, monitor *monitoring.HttpMonitoring) {
	msgBuilder := social.NewMessageBuilder(logger)
	bch, err := bch.NewBch(ctx, logger)
	if err != nil {
		logger.Fatalf("Error creating BCH client: %+v", err)
	}
	bch.StartNodeTimer()

	watch, err := watcher.NewWatcher(logger, monitor, counter, msgBuilder)
	if err != nil {
		logger.Fatalf("Error creating watcher: %+v", err)
	}

	blockCh, err := bch.WatchNewBlocks(ctx)
	if err != nil {
		logger.Fatalf("Error watching new blocks: %+v", err)
	}

	terminating := false
	for !terminating {
		select {
		case block := <-blockCh:
			for _, tx := range block.Tx {
				watch.CheckTransaction(&tx)
			}
			watch.CheckLastTweetTime()

		case <-ctx.Done():
			terminating = true
		}
	}
}

/*
func watchTransactions(counter *txcounter.TxCounter, ctx context.Context, logger log.Logger, monitor *monitoring.HttpMonitoring) {
	client, err := bchd.NewGrpcClient(logger, monitor, counter)
	if err != nil {
		logger.Fatalf("Error creating bchd gRPC client: %+v", err)
	}
	//defer client.Close()

	msgBuilder := social.NewMessageBuilder(logger)
	reqCtx, cancel := context.WithCancel(bchd.NewReqContext())
	go client.ReadTransactionStream(reqCtx, cancel, msgBuilder) // TODO if it ends here on 1st call we might not retry (race condition)
	terminating := false
	for !terminating {
		select {
		case <-reqCtx.Done():
			// something went wrong with our BCHD TX stream, retry
			logger.Errorf("Error in gRPC connection, retrying...")
			time.Sleep(10 * time.Second)

			if client != nil {
				client.Close()
			}
			client, err = bchd.NewGrpcClient(logger, monitor, counter)
			if err != nil {
				logger.Errorf("Error creating bchd gRPC client: %+v", err)
				break
			}
			reqCtx, cancel = context.WithCancel(bchd.NewReqContext())
			go client.ReadTransactionStream(reqCtx, cancel, msgBuilder)

		case <-ctx.Done():
			//cancel()
			client.Close()
			terminating = true
		}
	}
}
*/

func createMonitoringClient(ctx context.Context, logger log.Logger) *monitoring.HttpMonitoring {
	if viper.GetBool("Monitoring.Enable") == false {
		return nil
	}

	monitor, err := monitoring.NewHttpMonitoring(monitoring.HttpMonitoringConfig{
		HttpListenAddress: viper.GetString("Monitoring.Address"),
		Events: []string{
			"LastTweet", "TxCount", "TxAvgBch", "TxUpperPercentBch",
		},
	}, logger)
	if err != nil {
		logger.Fatalf("Error starting monitoring: %+v", err)
	}

	return monitor
}
