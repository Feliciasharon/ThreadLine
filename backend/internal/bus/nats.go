package bus

import (
	"time"

	"github.com/nats-io/nats.go"
)

type Client struct {
	Conn *nats.Conn
}

func Connect(url string) (*Client, error) {
	if url == "" {
		url = nats.DefaultURL
	}
	nc, err := nats.Connect(
		url,
		nats.UserCredentials("/etc/secrets/creds.creds"),
		nats.Timeout(3*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(500*time.Millisecond),
	)
	if err != nil {
		return nil, err
	}
	return &Client{Conn: nc}, nil
}

func (c *Client) Close() { c.Conn.Close() }

