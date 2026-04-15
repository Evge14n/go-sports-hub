package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

const SubjectEventsRaw = "events.raw"

type Client struct {
	conn *nats.Conn
}

func NewClient(url string) (*Client, error) {
	opts := []nats.Option{
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1),
		nats.PingInterval(20 * time.Second),
		nats.MaxPingsOutstanding(3),
	}

	conn, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) Close() {
	c.conn.Drain()
}

func (c *Client) IsConnected() bool {
	return c.conn.IsConnected()
}

func (c *Client) Publish(subject string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	if err := c.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}
	return nil
}

// SubscribeRaw subscribes with direct access to the nats.Msg for custom decoding.
func (c *Client) SubscribeRaw(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	sub, err := c.conn.Subscribe(subject, handler)
	if err != nil {
		return nil, fmt.Errorf("nats subscribe: %w", err)
	}
	return sub, nil
}

// SubscribeChan subscribes to a subject and delivers decoded messages via a typed channel.
func SubscribeChan[T any](ctx context.Context, c *Client, subject string) (<-chan T, error) {
	out := make(chan T, 128)

	sub, err := c.conn.Subscribe(subject, func(msg *nats.Msg) {
		var v T
		if err := json.Unmarshal(msg.Data, &v); err != nil {
			return
		}
		select {
		case out <- v:
		default:
		}
	})
	if err != nil {
		return nil, fmt.Errorf("nats subscribe chan: %w", err)
	}

	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
		close(out)
	}()

	return out, nil
}
