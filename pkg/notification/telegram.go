package notification

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/url"
	"regexp"
)

// ensure we always implement Notifier (compile error otherwise)
var _ Notifier = (*Telegram)(nil)

const telegramApiUrl = "https://api.telegram.org/bot%s/sendMessage?%s"

type Telegram struct {
	config TelegramConfig
}

type TelegramConfig struct {
	Token   string // received by talking to @BotFather
	Channel string // channel ID or user ID
}

func NewTelegram(config TelegramConfig) (*Telegram, error) {
	if len(config.Token) == 0 || len(config.Channel) == 0 {
		return nil, errors.New("Telegram receiver (token) and channel ID must be set to send Telegram notifications")
	}
	isNumeric, err := regexp.Match("^-?[0-9]+$", []byte(config.Channel))
	if err != nil {
		return nil, err
	}
	if isNumeric == false && config.Channel[:1] != "@" {
		return nil, errors.New("Telegram channel ID must start with @ or be a numeric channel ID from: curl -X POST https://api.telegram.org/bot[BOT_API_KEY]/getUpdates")
	}
	return &Telegram{
		config: config,
	}, nil
}

type TelegramResponse struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

func (t *Telegram) SendNotification(notification *Notification) error {
	httpAgent := getHttpAgent()
	notification.prepare()

	data := url.Values{
		"chat_id": []string{t.config.Channel},
		"text":    []string{notification.GetMessengerText()},
	}

	urlStr := fmt.Sprintf(telegramApiUrl, t.config.Token, data.Encode())
	resp, err := httpAgent.Get(urlStr)
	if err != nil {
		return errors.Wrap(err, "error sending Telegram message")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "error reading Telegram response")
	}

	var apiRes TelegramResponse
	err = json.Unmarshal(body, &apiRes)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling Telegram JSON")
	} else if !apiRes.Ok {
		return errors.New(fmt.Sprintf("Failed to send Telegram message. Code %d - %s", apiRes.ErrorCode, apiRes.Description))
	}

	return nil
}
