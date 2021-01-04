package bchd

import (
	"context"
	"crypto/tls"
	pb "github.com/Ekliptor/cashwhale/internal/bchd/golang"
	"github.com/Ekliptor/cashwhale/internal/log"
	"github.com/Ekliptor/cashwhale/internal/monitoring"
	"github.com/Ekliptor/cashwhale/internal/social"
	"github.com/Ekliptor/cashwhale/pkg/chainhash"
	"github.com/Ekliptor/cashwhale/pkg/price"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"io"
	"time"
)

type GRPCClient struct {
	Client pb.BchrpcClient

	logger  log.Logger
	monitor *monitoring.HttpMonitoring
	conn    *grpc.ClientConn
}

func NewGrpcClient(logger log.Logger, monitor *monitoring.HttpMonitoring) (grpcClient *GRPCClient, err error) {
	grpcClient = &GRPCClient{
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
	return grpcClient, nil
}

func (gc *GRPCClient) Close() error {
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
			if amountBCH < viper.GetFloat64("Message.WahleThresholdBCH") {
				continue
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
					gc.monitor.AddEvent("LastTweet", monitoring.EventMap{
						"msg": txData.Message,
					})
				}
			}
		}
	}

	return nil
}
