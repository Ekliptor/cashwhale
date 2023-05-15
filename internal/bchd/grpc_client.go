package bchd

import (
	"context"
	"crypto/tls"
	"fmt"
	pb "github.com/Ekliptor/cashwhale/internal/bchd/golang"
	"github.com/Ekliptor/cashwhale/internal/log"
	"github.com/Ekliptor/cashwhale/internal/monitoring"
	"github.com/Ekliptor/cashwhale/internal/social"
	"github.com/Ekliptor/cashwhale/pkg/chainhash"
	"github.com/Ekliptor/cashwhale/pkg/notification"
	"github.com/Ekliptor/cashwhale/pkg/price"
	"github.com/Ekliptor/cashwhale/pkg/txcounter"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"io"
	"time"
)

type GRPCClient struct {
	Client pb.BchrpcClient

	counter *txcounter.TxCounter
	logger  log.Logger
	monitor *monitoring.HttpMonitoring
	conn    *grpc.ClientConn
}

func NewGrpcClient(logger log.Logger, monitor *monitoring.HttpMonitoring, counter *txcounter.TxCounter) (grpcClient *GRPCClient, err error) {
	grpcClient = &GRPCClient{
		counter: counter,
		logger: logger.WithFields(log.Fields{
			"module": "bchd_grpc",
		}),
		monitor: monitor,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	target := viper.GetString("BCHD.Address")
	logger.Infof("Connecting to BCHD at: %s", target)
	var opts []grpc.DialOption
	if viper.GetString("BCHD.RootCertFile") != "" {
		creds, err := credentials.NewClientTLSFromFile(viper.GetString("BCHD.RootCertFile"), viper.GetString("BCHD.CaDomain"))
		if err != nil {
			logger.Errorf("Failed to create gRPC TLS credentials %v", err)
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		config := &tls.Config{
			InsecureSkipVerify: viper.GetBool("BCHD.AllowSelfSigned"),
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	}
	opts = append(opts, grpc.WithBlock())
	grpcClient.conn, err = grpc.DialContext(ctx, target, opts...)
	logger.Infof("Connecting to gRPC using TLS")
	if err != nil {
		grpcClient.logger.Errorf("%+v", err)
		return nil, err
	}

	grpcClient.Client = pb.NewBchrpcClient(grpcClient.conn)

	// add dummy tweet so we always have a LastTweet value (in case we never start sending)
	grpcClient.monitor.AddEvent("LastTweet", monitoring.D{
		"msg": "never (connected)",
	})

	grpcClient.checkLastTweetTime()
	return grpcClient, nil
}

func (gc *GRPCClient) Close() error {
	if gc.conn == nil {
		return nil
	}
	return gc.conn.Close()
}

func NewReqContext() context.Context {
	reqCtx := context.Background()
	if viper.GetString("BCHD.AuthenticationToken") != "" {
		reqCtx = metadata.AppendToOutgoingContext(reqCtx, "AuthenticationToken", viper.GetString("BCHD.AuthenticationToken"))
	}
	return reqCtx
}

func (gc *GRPCClient) ReadTransactionStream(reqCtx context.Context, cancel context.CancelFunc, msgBuilder *social.MessageBuilder) error {
	// open the TX stream
	transactionStream, err := gc.Client.SubscribeTransactions(reqCtx, &pb.SubscribeTransactionsRequest{
		Subscribe: &pb.TransactionFilter{
			AllTransactions: true,
		},
		Unsubscribe:    nil,
		IncludeMempool: false,
		IncludeInBlock: true,
		SerializeTx:    false,
	})
	if err != nil {
		gc.logger.Errorf("Error subscribing to bchd TX stream: %+v", err)
		cancel()
		return err
	}
	gc.logger.Infof("Opened TX stream from BCHD")
	for {
		data, err := transactionStream.Recv()
		if err == io.EOF {
			gc.logger.Errorf("BCHD TX stream stopped. This shouldn't happen!")
			cancel()
			break
		} else if err != nil {
			gc.logger.Errorf("Error in BCHD TX stream: %+v", err)
			cancel()
			break
		}

		//gc.logger.Debugf("TX: %+v", data)
		if data.GetType() == pb.TransactionNotification_CONFIRMED {
			tx := data.GetConfirmedTransaction()
			if tx == nil {
				gc.logger.Errorf("Received invalid bchd TX")
				continue
			}
			gc.checkLastTweetTime()

			// check if it's a Coinbase TX
			inputs := tx.GetInputs()
			if len(inputs) == 0 { // can't happen
				gc.logger.Errorf("TX has 0 inputs. block height %d, hash (reversed) %s", tx.GetBlockHeight(), tx.GetHash())
				continue
			}
			inputHash, err := chainhash.NewHash(inputs[0].GetOutpoint().GetHash())
			if err != nil {
				gc.logger.Errorf("Error getting input hash of TX %+v", err)
				continue
			} else if (inputHash.IsEqual(&chainhash.Hash{})) {
				continue // Coinbase TX has no input
			}

			// loop through TX outputs and find big transactions
			fee := getTransactionFee(tx)
			outputs := tx.GetOutputs()
			var amountBCH float64 = 0.0
			for _, out := range outputs {
				// TODO add a filter if outputAddress in [previousInputAddress, ...] and deduct it
				// last address is usually change address
				amountBCH += price.SatoshiToBitcoin(out.GetValue())
			}
			gc.counter.AddTransaction(float32(amountBCH))
			if amountBCH < viper.GetFloat64("Message.WahleThresholdBCH") {
				//if gc.counter.GetTransactionCount() < viper.GetInt("Average.MinTxCount") || amountBCH < float64(gc.counter.GetAverageTransactionSize()) * viper.GetFloat64("Average.AverageTxFactor") {
				if gc.counter.GetTransactionCount() < viper.GetInt("Average.MinTxCount") || amountBCH < float64(gc.counter.GetUpperTransactionSizePercent(float32(viper.GetFloat64("Average.UpperTxPercent")))) {
					continue
				}
			}

			hash, err := chainhash.NewHash(tx.GetHash())
			if err != nil {
				gc.logger.Errorf("Error getting hash of TX %+v", err)
				continue
			}
			txData := &social.TransactionData{
				AmountBchRaw: amountBCH,
				FeeBch:       price.SatoshiToBitcoin(fee),
				Hash:         hash.String(),
			}
			err = msgBuilder.CreateMessage(txData)
			if err == nil {
				err = msgBuilder.SendMessage(txData)
				if err != nil {
					gc.logger.Errorf("Error sending message %+v", err)
				} else if gc.monitor != nil {
					gc.monitor.AddEvent("LastTweet", monitoring.D{
						"msg": txData.Message,
						//"time": time.Now(), // "when" attribute is present in event map
					})
				}
			}
		}
	}

	return nil
}

func (gc *GRPCClient) checkLastTweetTime() {
	lastTweet := gc.monitor.GetEvent("LastTweet")
	if lastTweet == nil {
		gc.logger.Errorf("No LastTweet event found. Monitoring will not work")
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
		gc.logger.Errorf("Error reading notifier config %+v", err)
		return
	}

	sendData := notification.NewNotification(fmt.Sprintf("%s tweets stopped", viper.GetString("App.Name")),
		fmt.Sprintf("Last tweet: %s ago", time.Since(lastTweetTime)))

	for _, notify := range notifier {
		_, err = notification.CreateAndSendNotification(sendData, notify)
		if err != nil {
			gc.logger.Errorf("Error sending 'tweets stopped' notification %+v", err)
			return
		}
	}
}
