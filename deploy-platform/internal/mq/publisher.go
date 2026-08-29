package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher 发送持久化部署消息并等待 broker confirm
type Publisher struct {
	channel *amqp.Channel
}

// NewPublisher 声明发布拓扑并开启 publisher confirm 模式
func NewPublisher(channel *amqp.Channel) (*Publisher, error) {
	if err := declareTopology(channel); err != nil {
		return nil, fmt.Errorf("MQ 拓扑声明失败: %w", err)
	}
	if err := channel.Confirm(false); err != nil {
		return nil, fmt.Errorf("开启 publisher confirm 失败: %w", err)
	}
	return &Publisher{
		channel,
	}, nil
}

// Publish 发送持久化部署消息并等待 broker confirm
// 返回 nil 只表示 RabbitMQ 已接管消息，不表示 Worker 已完成任务
func (p *Publisher) Publish(ctx context.Context, msg DeployMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("消息序列化失败: %w", err)
	}
	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		ExchangeName,
		RoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    msg.MessageID,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
	if err != nil {
		return err
	}

	// confirm 只表示 broker 接管消息，不表示 Worker 已经处理
	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("等待 publisher confirm 失败: %w", err)
	}
	if !acked {
		return errors.New("RabbitMQ 拒绝消息")
	}
	return nil
}
