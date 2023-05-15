package notification

import (
	"fmt"
	"github.com/spf13/viper"
)

type Notification struct {
	Title               string
	Text                string
	RequireConfirmation bool
}

func NewNotification(title string, text string) *Notification {
	return &Notification{
		Title:               title,
		Text:                filterInvalidChars(text),
		RequireConfirmation: false,
	}
}

// Return the messenger text message containing title + message text to be used for Telegram, etc...
func (n *Notification) GetMessengerText() string {
	message := n.Title
	if len(n.Text) != 0 {
		message += ":\r\n" + n.Text
	}
	return message
}

func (n *Notification) prepare() {
	prefix := fmt.Sprintf("%s: ", viper.GetString("App.Name"))
	n.Title = prefix + n.Title
	if len(n.Text) == 0 {
		n.Text = "empty text" // some services (Pushover) can't send empty messages
	}
}

func filterInvalidChars(text string) string {
	//text = strings.ReplaceAll(text, "@", "-")
	return text
}
