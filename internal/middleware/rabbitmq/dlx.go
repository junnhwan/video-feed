package rabbitmq

import amqp "github.com/rabbitmq/amqp091-go"

const (
	DLXExchange   = "dlx.events"
	MaxRetryCount = 3
)

func DeclareDLX(ch *amqp.Channel, queueName string) error {
	if ch == nil {
		return nil
	}
	if err := ch.ExchangeDeclare(DLXExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	dlxQueue := queueName + ".dlx"
	if _, err := ch.QueueDeclare(dlxQueue, true, false, false, false, nil); err != nil {
		return err
	}
	return ch.QueueBind(dlxQueue, "#", DLXExchange, false, nil)
}

func GetRetryCount(delivery amqp.Delivery) int {
	deaths, ok := delivery.Headers["x-death"].([]any)
	if !ok || len(deaths) == 0 {
		return 0
	}
	death, ok := deaths[0].(amqp.Table)
	if !ok {
		return 0
	}
	count, ok := death["count"].(int64)
	if !ok {
		return 0
	}
	return int(count)
}
