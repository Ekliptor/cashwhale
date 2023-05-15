package notification

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"testing"
)

func TestSendNotification(t *testing.T) {
	loadTestConfig()

	notifier := make([]*NotificationReceiver, 0, 5)

	err := viper.UnmarshalKey("Notify", &notifier)
	if err != nil {
		t.Fatalf("Error reading notifier config %+v", err)
	}

	sendData := NewNotification(fmt.Sprintf("%s test notification", viper.GetString("App.Name")), "test message")

	for _, notify := range notifier {
		_, err = CreateAndSendNotification(sendData, notify)
		if err != nil {
			t.Fatalf("Error sending test notification %+v", err)
			return
		}
	}
}

func loadTestConfig() {
	viper.SetConfigName("config")
	viper.AddConfigPath("../..")
	viper.SetDefault("Log.Level", "debug")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("unable to read config: %v\n", err)
		os.Exit(1)
	}
}
