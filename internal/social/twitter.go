package social

import (
	"github.com/Ekliptor/cashwhale/internal/log"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/spf13/viper"
)

type TwitterClient struct {
	Client *twitter.Client

	logger log.Logger
}

func NewTwitterClient(logger log.Logger) *TwitterClient {
	config := oauth1.NewConfig(viper.GetString("Twitter.ConsumerKey"), viper.GetString("Twitter.ConsumerSecret"))
	token := oauth1.NewToken(viper.GetString("Twitter.AccessToken"), viper.GetString("Twitter.AccessSecret"))
	httpClient := config.Client(oauth1.NoContext, token)

	// Twitter client
	client := twitter.NewClient(httpClient)
	return &TwitterClient{
		Client: client,
		logger: logger.WithFields(
			log.Fields{
				"module": "twitter",
			},
		),
	}
}

func (c *TwitterClient) SendTweet(msg string) (*twitter.Tweet, error) {
	c.logger.Debugf("Sending tweet: %s", msg)
	tweet, _, err := c.Client.Statuses.Update(msg, nil)
	if err != nil {
		c.logger.Errorf("Error sending tweet %+v", err)
		return nil, err
	}
	c.logger.Infof("Successfully sent tweet with ID: %s", tweet.IDStr)
	return tweet, err
}
