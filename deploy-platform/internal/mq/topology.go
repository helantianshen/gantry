package mq

import (
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// declareTopology 幂等声明主交换机、主队列以及死信交换机和死信队列
func declareTopology(channel *amqp.Channel) error {
	if err := channel.ExchangeDeclare(
		DLXName,
		"direct",
		true,
		false,
		false,
		false,
		nil); err != nil {
		return fmt.Errorf("声明 DLX 交换机失败: %w", err)
	}
	if _, err := channel.QueueDeclare(
		DLQName,
		true,
		false,
		false,
		false,
		nil); err != nil {
		return fmt.Errorf("声明 DLQ 失败: %w", err)
	}
	if err := channel.QueueBind(
		DLQName,
		RoutingKey,
		DLXName,
		false,
		nil); err != nil {
		return fmt.Errorf("绑定 DLQ 失败: %w", err)
	}

	if err := channel.ExchangeDeclare(
		ExchangeName,
		"direct",
		true,
		false,
		false,
		false,
		nil); err != nil {
		return fmt.Errorf("声明主交换机失败: %w", err)
	}
	args := amqp.Table{
		// 主队列拒绝且不重入的消息交给死信交换机
		"x-dead-letter-exchange": DLXName,
	}
	if _, err := channel.QueueDeclare(
		QueueName,
		true,
		false,
		false,
		false,
		args); err != nil {
		return fmt.Errorf("声明主队列失败: %w", err)
	}
	if err := channel.QueueBind(
		QueueName,
		RoutingKey,
		ExchangeName,
		false,
		nil); err != nil {
		return fmt.Errorf("绑定主队列失败: %w", err)
	}

	log.Println("MQ 拓扑就绪")
	return nil
}
