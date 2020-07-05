package bchd

import (
	"context"
	"crypto/tls"
	"github.com/Ekliptor/cashwhale/internal/bchd/chainhash"
	pb "github.com/Ekliptor/cashwhale/internal/bchd/golang"
	"github.com/Ekliptor/cashwhale/internal/log"
	"github.com/Ekliptor/cashwhale/internal/social"
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

	logger log.Logger
	conn   *grpc.ClientConn
}

func NewGrpcClient(logger log.Logger) (grpcClient *GRPCClient, err error) {
	grpcClient = &GRPCClient{
		logger: logger.WithFields(log.Fields{
			"module": "bchd_grpc",
		}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	target := viper.GetString("BCHD.Address")
	logger.Infof("Connecting to BCHD at: %s", target)
	var opts []grpc.DialOption
	if viper.GetString("BCHD.RootCertFile") != "" {
		creds, err := credentials.NewClientTLSFromFile(viper.GetString("BCHD.RootCertFile"), viper.GetString("BCHD.CaDomain"))
		if err != nil {
			logger.Fatalf("Failed to create gRPC TLS credentials %v", err)
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
		grpcClient.logger.Fatalf("%+v", err)
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

func (gc *GRPCClient) ReadTransactionStream(reqCtx context.Context, msgBuilder *social.MessageBuilder) error {
	// open the TX stream
	transactionStream, err := gc.Client.SubscribeTransactions(reqCtx, &pb.SubscribeTransactionsRequest{
		Subscribe:      &pb.TransactionFilter{
			AllTransactions:  true,
		},
		Unsubscribe:    nil,
		IncludeMempool: false,
		IncludeInBlock: true,
		SerializeTx:    false,
	})
	if err != nil {
		gc.logger.Fatalf("Error subscribing to bchd TX stream: %+v", err)
		return err
	}
	gc.logger.Infof("Opened TX stream from BCHD")
	for {
		data, err := transactionStream.Recv()
		if err == io.EOF {
			gc.logger.Errorf("BCHD TX stream stopped. This shouldn't happen!")
			break
		} else if err != nil {
			gc.logger.Errorf("Error in BCHD TX stream: %+v", err)
			break
		}
		gc.logger.Debugf("TX: %+v", data)
		if data.GetType() == pb.TransactionNotification_CONFIRMED {
			tx := data.GetConfirmedTransaction()
			if tx == nil {
				gc.logger.Errorf("Received invalid bchd TX")
			} else {
				gc.logger.Debugf("TX: %+v", tx.GetOutputs())

				// loop through TX outputs and find big transactions
				outputs := tx.GetOutputs()
				for _, out := range outputs {
					amountBCH := price.SatoshiToBitcoin(out.Value)
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
						//Hash: hex.EncodeToString(tx.GetHash()),
						//Hash: hex.EncodeToString(tx.GetBlockHash()),
						Hash: hash.String(),
					}
					err = msgBuilder.CreateMessage(txData)
					if err == nil {
						msgBuilder.SendMessage(txData)
					}
				}
			}
		}
	}

	return nil
}

func (gc *GRPCClient) getTotalValueFromTx() {

}