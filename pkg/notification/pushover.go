package notification

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/url"
)

// ensure we always implement Notifier (compile error otherwise)
var _ Notifier = (*Pushover)(nil)

const pushoverApiUrl = "https://api.pushover.net/1/messages.json"

type Pushover struct {
	config PushoverConfig
}

type PushoverConfig struct {
	AppToken string
	Receiver string
}

func NewPushover(config PushoverConfig) (*Pushover, error) {
	if len(config.Receiver) < 10 {
		return nil, errors.New("invalid Pushover receiver token")
	}
	return &Pushover{
		config: config,
	}, nil
}

type PusoverResponse struct {
	Status    int    `json:"status"`
	RequestId string `json:"request"` // UUID
}

func (p *Pushover) SendNotification(notification *Notification) error {
	httpAgent := getHttpAgent()
	notification.prepare()

	data := url.Values{
		"token":   []string{p.config.AppToken},
		"user":    []string{p.config.Receiver},
		"message": []string{notification.Text},
		"title":   []string{notification.Title},
	}
	if notification.RequireConfirmation {
		data["priority"] = []string{"2"}
		data["expire"] = []string{"900"} // 15min
		data["retry"] = []string{"60"}
	}

	resp, err := httpAgent.PostForm(pushoverApiUrl, data)
	if err != nil {
		return errors.Wrap(err, "error sending Pushover message")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "error reading Pushover response")
	}

	var apiRes PusoverResponse
	err = json.Unmarshal(body, &apiRes)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling Pushover JSON")
	} else if apiRes.Status != 1 {
		return errors.New(fmt.Sprintf("Failed to send pushover message. Status %d", apiRes.Status))
	}

	return nil
}
