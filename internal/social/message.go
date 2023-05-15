package social

import (
	"bytes"
	"fmt"
	"github.com/Ekliptor/cashwhale/internal/log"
	"github.com/Ekliptor/cashwhale/pkg/price"
	"github.com/spf13/viper"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"text/template"
)

type MessageBuilder struct {
	logger  log.Logger
	twitter *TwitterClient
	// TODO add memo and more
}

func NewMessageBuilder(logger log.Logger) *MessageBuilder {
	builder := &MessageBuilder{
		logger: logger.WithFields(
			log.Fields{
				"module": "message",
			},
		),
	}
	if viper.GetBool("Twitter.Enable") {
		builder.twitter = NewTwitterClient(logger)
	}
	return builder
}

type TransactionData struct {
	//RawTXs []*pb.Transaction_Output `json:"txs"`
	AmountBchRaw float64 `json:"amount_bch_raw"`

	Amount     string  `json:"amount"`
	Symbol     string  `json:"symbol"`
	Currency   string  `json:"currency"`
	FeeBch     float64 `json:"fee"`
	FiatFee    string  `json:"fee"`
	FiatAmount string  `json:"fiat_amount"`
	FiatSymbol string  `json:"fiat_symbol"`
	Hash       string  `json:"hash"`
	TxLink     string  `json:"tx_link"`

	Message string `json:"message"`
}

// Prepares a social media message from RawTXs.
func (m *MessageBuilder) CreateMessage(tx *TransactionData) error {
	// TODO crawl rich list addresses and name them in tweets: https://bitinfocharts.com/top-100-richest-bitcoin%20cash-addresses.html

	// get current price // TODO cache?
	price, err := price.GetBitcoinCashRate(viper.GetString("Message.FiatCurrency"))
	if err != nil {
		m.logger.Errorf("Error getting BCH rate %+v", err)
		return err
	}

	// fill template vars
	pr := message.NewPrinter(language.English)
	tx.Amount = pr.Sprintf("%.0f", tx.AmountBchRaw)
	tx.Symbol = "BCH" // TODO add SLP support
	tx.Currency = "BitcoinCash"
	tx.FiatAmount = pr.Sprintf("%.0f", tx.AmountBchRaw*float64(price))

	fiatFee := tx.FeeBch * float64(price)
	if fiatFee < 0.0001 {
		fiatFee = 0.0001
	}
	tx.FiatFee = pr.Sprintf("%.4f", fiatFee)

	tx.FiatSymbol = viper.GetString("Message.FiatCurrency")
	tx.TxLink = fmt.Sprintf(viper.GetString("Message.BlockExplorer"), tx.Hash)

	// create message
	tmpl, err := template.New("message").Parse(viper.GetString("Message.Text"))
	if err != nil {
		m.logger.Errorf("Error creating message from template %s %+v", viper.GetString("Message.Text"), err)
		return err
	}
	var data bytes.Buffer
	err = tmpl.Execute(&data, tx)
	if err != nil {
		m.logger.Errorf("Error executing template message %+v", err)
		return err
	}
	tx.Message = data.String()
	m.logger.Debugf("Msg: %s", tx.Message)
	return nil
}

// Sends message. Call this after CreateMessage().
func (m *MessageBuilder) SendMessage(tx *TransactionData) error {
	if viper.GetBool("Twitter.Enable") {
		_, err := m.twitter.SendTweet(tx.Message)
		if err != nil {
			//m.logger.Errorf("Error tweeting %+v", err)
			return err
		}
	}

	return nil
}
