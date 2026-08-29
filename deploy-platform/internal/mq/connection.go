package mq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// NewConn 建立 RabbitMQ 连接并创建通道，任一步失败都不保留半初始化资源
// 返回值依次为连接、通道和初始化错误
func NewConn(url string) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("RabbitMQ 连接失败: %w", err)
	}
	channel, err := conn.Channel()
	if err != nil {
		// 通道创建失败时关闭已建立的连接，避免调用方无法回收半成品
		conn.Close()
		return nil, nil, fmt.Errorf("创建通道失败: %w", err)
	}
	return conn, channel, nil
}
