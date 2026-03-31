package kafka

import (
	"fmt"
	"log"

	"github.com/IBM/sarama"
)

type Producer struct {
	producer sarama.SyncProducer
	topic    string
}

func NewProducer(brokers []string, topic string) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	log.Printf("Kafka producer connected to %v, topic: %s", brokers, topic)
	return &Producer{producer: producer, topic: topic}, nil
}

func (p *Producer) SendMessage(key, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
	_, _, err := p.producer.SendMessage(msg)
	return err
}

func (p *Producer) Close() error {
	return p.producer.Close()
}
