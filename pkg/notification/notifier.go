package notification

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"net/http"
	"time"
)

type Notifier interface {
	SendNotification(notification *Notification) error
}

func CreateAndSendNotification(sendData *Notification, notify *NotificationReceiver) (Notifier, error) {
	switch notify.Method {
	case "pushover":
		push, err := NewPushover(PushoverConfig{
			AppToken: notify.AppToken,
			Receiver: notify.Receiver,
		})
		if err != nil {
			return nil, err
		}
		err = push.SendNotification(sendData)
		if err != nil {
			return nil, err
		}
		return push, nil

	case "telegram":
		tele, err := NewTelegram(TelegramConfig{
			Token:   notify.Token,
			Channel: notify.Channel,
		})
		if err != nil {
			return nil, err
		}
		err = tele.SendNotification(sendData)
		if err != nil {
			return nil, err
		}
		return tele, nil

	case "email":
		email, err := NewEmail(EmailConfig{
			SmtpHost:        notify.SmtpHost,
			SmtpPort:        notify.SmtpPort,
			AllowSelfSigned: notify.AllowSelfSigned,
			FromAddress:     notify.FromAddress,
			FromPassword:    notify.FromPassword,
			RecAddress:      notify.RecAddress,
		})
		if err != nil {
			return nil, err
		}
		err = email.SendNotification(sendData)
		if err != nil {
			return nil, err
		}
		return email, nil

	default:
		return nil, errors.New(fmt.Sprintf("unknown notification Method in config: %s", notify.Method))
	}
}

type NotificationMethod string

const (
	NOTIFICATION_EMAIL    = "email"
	NOTIFICATION_PUSHOVER = "pushover"
	NOTIFICATION_TELEGRAM = "telegram"
)

type NotificationReceiver struct {
	Method NotificationMethod `mapstructure:"Method"`

	// Email
	SmtpHost        string `mapstructure:"SmtpHost"`
	SmtpPort        int    `mapstructure:"SmtpPort"`
	AllowSelfSigned bool   `mapstructure:"AllowSelfSigned"`
	FromAddress     string `mapstructure:"FromAddress"`
	FromPassword    string `mapstructure:"FromPassword"`
	RecAddress      string `mapstructure:"RecAddress"`

	// Pushover
	AppToken string `mapstructure:"AppToken"`
	Receiver string `mapstructure:"Receiver"`

	// Telegram
	Token   string `mapstructure:"Token"`
	Channel string `mapstructure:"Channel"`
}

func getHttpAgent() *http.Client {
	return &http.Client{
		Timeout: time.Duration(viper.GetInt("HTTP.RequestTimeoutSec")) * time.Second,
	}
}
