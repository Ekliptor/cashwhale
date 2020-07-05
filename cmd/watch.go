package cmd

import (
	"context"
	"github.com/Ekliptor/cashwhale/internal/bchd"
	"github.com/Ekliptor/cashwhale/internal/log"
	"github.com/Ekliptor/cashwhale/internal/social"
	"github.com/spf13/cobra"
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
		go listenExitCommand(logger, cancel)

		// start all main workers in separate goroutines
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			watchTransactions(ctx, logger)
		}()

		wg.Wait()
		return nil
	},
}

func watchTransactions(ctx context.Context, logger log.Logger) {
	client, err := bchd.NewGrpcClient(logger)
	if err != nil {
		logger.Fatalf("Error creating bchd gRPC client: %+v", err)
	}
	//defer client.Close()

	msgBuilder := social.NewMessageBuilder(logger)
	reqCtx := bchd.NewReqContext()
	go client.ReadTransactionStream(reqCtx, msgBuilder) // TODO if it ends here on 1st call we might not retry (race condition)
	terminating := false
	for !terminating {
		select {
		case <- reqCtx.Done():
			// something went wrong with our BCHD TX stream, retry
			logger.Errorf("Error in gRPC connection, retrying...")
			client.Close()

			time.Sleep(10 * time.Second)
			client, err = bchd.NewGrpcClient(logger)
			if err != nil {
				logger.Fatalf("Error creating bchd gRPC client: %+v", err)
			}
			reqCtx = bchd.NewReqContext()
			go client.ReadTransactionStream(reqCtx, msgBuilder)

		case <- ctx.Done():
			//cancel()
			client.Close()
			terminating = true
		}
	}
}