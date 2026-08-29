package mq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer 使用手动确认和单条预取接收部署消息
type Consumer struct {
	channel *amqp.Channel
	tag     string
}

// NewConsumer 声明消费拓扑并为 workerID 创建单条预取的手动确认消费者
func NewConsumer(channel *amqp.Channel, workerID string) (*Consumer, error) {
	if err := declareTopology(channel); err != nil {
		return nil, fmt.Errorf("MQ 拓扑声明失败: %w", err)
	}
	if err := channel.Qos(1, 0, false); err != nil {
		return nil, fmt.Errorf("Qos 设置失败: %w", err)
	}
	return &Consumer{
		channel: channel,
		tag:     workerID,
	}, nil
}

// Consume 启动消费并返回消息通道，消息必须由 Worker 显式 Ack 或 Nack
func (c *Consumer) Consume() (<-chan amqp.Delivery, error) {
	return c.channel.Consume(
		QueueName,
		c.tag,
		false,
		false,
		false,
		false,
		nil)
}

// Close 取消当前 consumer tag 并停止接收新消息
func (c *Consumer) Close() error {
	return c.channel.Cancel(c.tag, false)
}
