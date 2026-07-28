package platform

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// NewRabbitConn dials RabbitMQ and returns the connection. The caller is
// responsible for closing it on shutdown. A single connection is multiplexed
// into lightweight "channels" — you open a channel per goroutine that publishes
// or consumes, but share one TCP connection per process.
func NewRabbitConn(url string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	return conn, nil
}
