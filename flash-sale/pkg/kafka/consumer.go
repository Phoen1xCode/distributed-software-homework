package kafka

import (
	"context"
	"fmt"
	"log"

	"github.com/IBM/sarama"
)

type MessageHandler func(key, value []byte) error

type Consumer struct {
	group   sarama.ConsumerGroup
	topic   string
	handler MessageHandler
}

func NewConsumer(brokers []string, groupID, topic string, handler MessageHandler) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer group: %w", err)
	}

	log.Printf("Kafka consumer group '%s' connected, topic: %s", groupID, topic)
	return &Consumer{group: group, topic: topic, handler: handler}, nil
}

func (c *Consumer) Start(ctx context.Context) {
	h := &consumerGroupHandler{handler: c.handler}
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, h); err != nil {
			log.Printf("[ERROR] Kafka consume error: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (c *Consumer) Close() error {
	return c.group.Close()
}

type consumerGroupHandler struct {
	handler MessageHandler
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := h.handler(msg.Key, msg.Value); err != nil {
			log.Printf("[ERROR] Failed to process message: %v", err)
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
