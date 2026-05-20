package rabbitmq

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"video-feed/internal/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	Conn *amqp.Connection
	Ch   *amqp.Channel
}

func NewRabbitMQ(cfg config.RabbitMQConfig) (*RabbitMQ, error) {
	url := "amqp://" + cfg.Username + ":" + cfg.Password + "@" + cfg.Host + ":" + strconv.Itoa(cfg.Port) + "/"
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &RabbitMQ{Conn: conn, Ch: ch}, nil
}

func (r *RabbitMQ) Close() error {
	if r == nil {
		return nil
	}
	if r.Ch != nil {
		if err := r.Ch.Close(); err != nil {
			return err
		}
	}
	if r.Conn != nil {
		return r.Conn.Close()
	}
	return nil
}

func (r *RabbitMQ) DeclareTopic(exchange string, queue string, bindingKey string) error {
	if r == nil || r.Ch == nil {
		return errors.New("rabbitmq is not initialized")
	}
	if exchange == "" || queue == "" || bindingKey == "" {
		return errors.New("exchange, queue and binding_key are required")
	}
	if err := r.Ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	q, err := r.Ch.QueueDeclare(queue, true, false, false, false, amqp.Table{"x-dead-letter-exchange": DLXExchange})
	if err != nil {
		return err
	}
	if err := r.Ch.QueueBind(q.Name, bindingKey, exchange, false, nil); err != nil {
		return err
	}
	return DeclareDLX(r.Ch, queue)
}

func (r *RabbitMQ) PublishJSON(ctx context.Context, exchange string, routingKey string, payload any) error {
	if r == nil || r.Ch == nil {
		return errors.New("rabbitmq is not initialized")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return r.Ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
}

func newEventID(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate event id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
