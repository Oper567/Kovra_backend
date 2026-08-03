package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/lucepay-dev/lucepay/backend/notification-service/internal/usecase"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	uc     *usecase.NotificationUsecase
	logger *slog.Logger
}

func NewConsumer(url string, uc *usecase.NotificationUsecase, logger *slog.Logger) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	err = ch.ExchangeDeclare(
		"lucepay.events", // name
		"topic",        // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return &Consumer{
		conn:   conn,
		ch:     ch,
		uc:     uc,
		logger: logger,
	}, nil
}

func (c *Consumer) Start() error {
	q, err := c.ch.QueueDeclare(
		"notifications_queue",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// Bind to topics we care about
	topics := []string{"wallet.credited", "wallet.debited", "engagement.streak", "engagement.reward"}
	for _, topic := range topics {
		if err := c.ch.QueueBind(q.Name, topic, "lucepay.events", false, nil); err != nil {
			return err
		}
	}

	msgs, err := c.ch.Consume(
		q.Name,
		"",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for d := range msgs {
			c.handleEvent(d)
		}
	}()

	return nil
}

func (c *Consumer) handleEvent(d amqp.Delivery) {
	defer d.Ack(false)

	var payload struct {
		UserID string `json:"user_id"`
		Amount string `json:"amount,omitempty"`
		Desc   string `json:"description,omitempty"`
	}

	if err := json.Unmarshal(d.Body, &payload); err != nil {
		c.logger.Error("failed to unmarshal event", slog.String("error", err.Error()))
		return
	}

	ctx := context.Background()
	switch d.RoutingKey {
	case "wallet.credited":
		title := "Money Received! 💰"
		body := fmt.Sprintf("Your wallet was credited with ₦%s. %s", payload.Amount, payload.Desc)
		c.uc.SendPushToUser(ctx, payload.UserID, title, body, "wallet", nil)
	case "wallet.debited":
		title := "Payment Successful 💸"
		body := fmt.Sprintf("You spent ₦%s. %s", payload.Amount, payload.Desc)
		c.uc.SendPushToUser(ctx, payload.UserID, title, body, "wallet", nil)
	case "engagement.streak":
		title := "Streak Milestone! 🔥"
		body := "You've hit a new login streak! Keep it up to earn more cashback."
		c.uc.SendPushToUser(ctx, payload.UserID, title, body, "engagement", nil)
	default:
		c.logger.Info("unhandled routing key", slog.String("key", d.RoutingKey))
	}
}

func (c *Consumer) Close() {
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
