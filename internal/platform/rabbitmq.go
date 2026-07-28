package platform

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

// NewRabbitConn dials RabbitMQ and returns the connection, retrying until the
// broker is reachable or ctx is cancelled. The caller is responsible for closing
// it on shutdown. A single connection is multiplexed into lightweight "channels"
// — you open a channel per goroutine that publishes or consumes, but share one
// TCP connection per process.
func NewRabbitConn(ctx context.Context, url string) (*amqp.Connection, error) {
	var conn *amqp.Connection
	err := retryConnect(ctx, "rabbitmq", func(context.Context) error {
		c, err := amqp.Dial(url)
		if err != nil {
			return err
		}
		conn = c
		return nil
	})
	return conn, err
}
