package price

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
)

type BitcoinComRate struct {
	Price int `json:"price"`
	Stamp uint64 `json:"stamp"`
}

func GetBitcoinCashRate(fiatCurrency string) (float32, error) {
	url := viper.GetString(fmt.Sprintf("Price.API.%s", fiatCurrency))
	if len(url) == 0 {
		return 0.0, errors.New(fmt.Sprintf("not supported fiat currency %s", fiatCurrency))
	}
	resp, err := http.Get(url)
	if err != nil {
		return 0.0, err
	}
	defer resp.Body.Close()

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0.0, err
	}
	var data BitcoinComRate
	err = json.Unmarshal(body, &data)
	if err != nil {
		return 0.0, err
	}
	if data.Price <= 0 {
		return 0.0, errors.New(fmt.Sprintf("invalid response from %s", url))
	}
	return float32(data.Price / 100.0), nil
}

func SatoshiToBitcoin(sats int64) float64 {
	return float64(sats / 100000000.0)
}
