package price

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	fmt.Println("setting up test environment")
	viper.SetConfigName("config")
	viper.AddConfigPath("../../")
	viper.SetDefault("LogLevel", "debug")
	viper.AutomaticEnv()
	viper.ReadInConfig()

	code := m.Run()

	//fmt.Println("cleanup")

	os.Exit(code)
}

func TestPriceAPI(t *testing.T) {
	rate, err := GetBitcoinCashRate("USD")
	if err != nil {
		t.Fatalf("Error in price API %+v", err)
	}
	t.Logf("Received price: %.2f", rate)
}
