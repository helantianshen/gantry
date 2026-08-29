package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gantry/deploy-platform/internal/metrics"
	"gantry/deploy-platform/internal/mq"
	"gantry/deploy-platform/internal/service"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Worker 把 RabbitMQ delivery 转换为应用服务调用和 Ack/Nack
type Worker struct {
	id       string
	consumer *mq.Consumer
	runner   *service.DeploymentRunner
}

// New 组装消息消费者和部署执行服务，id 用于日志和消费标识
func New(id string, consumer *mq.Consumer, runner *service.DeploymentRunner) *Worker {
	return &Worker{
		id:       id,
		consumer: consumer,
		runner:   runner,
	}
}

// Run 顺序消费 delivery，直到 ctx 取消或 RabbitMQ 消息通道关闭
func (w *Worker) Run(ctx context.Context) error {
	deliveries, err := w.consumer.Consume()
	if err != nil {
		return fmt.Errorf("启动消费失败: %w", err)
	}
	metrics.WorkerActive.Inc()
	defer metrics.WorkerActive.Dec()
	log.Printf("Worker %s 开始消费队列 %s", w.id, mq.QueueName)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %s 收到停止信号, 退出消费循环", w.id)
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("消息通道已关闭")
			}
			w.handleDelivery(ctx, delivery)
		}
	}
}

// handleDelivery 校验消息并将 Service 处理结果映射为 RabbitMQ Ack 或 Nack
func (w *Worker) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	var msg mq.DeployMessage
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		log.Printf("反序列化消息失败: %v", err)
		_ = delivery.Nack(false, false)
		return
	}
	if !validMessage(msg) {
		log.Printf("部署消息字段无效: message_id=%q deployment_id=%d", msg.MessageID, msg.DeploymentID)
		_ = delivery.Nack(false, false)
		return
	}

	switch w.runner.Handle(ctx, msg) {
	case service.ProcessingRetry:
		// 临时故障重新入队，RabbitMQ 后续会再次投递
		_ = delivery.Nack(false, true)
	case service.ProcessingDiscard:
		// 永久无效消息不再进入主队列，由已配置的 DLX 接管
		_ = delivery.Nack(false, false)
	default:
		// 完成或已由其他 Worker 处理的消息可以安全确认
		_ = delivery.Ack(false)
	}
}

// validMessage 检查执行所需标识均有效，Attempt 允许初始值 0
func validMessage(msg mq.DeployMessage) bool {
	return msg.MessageID != "" && msg.DeploymentID > 0 && msg.AppID > 0 && msg.VersionID > 0 && msg.Attempt >= 0
}
