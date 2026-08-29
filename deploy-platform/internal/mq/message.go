package mq

const (
	ExchangeName = "deploy.exchange"
	QueueName    = "deploy.queue"
	DLXName      = "deploy.dlx"
	DLQName      = "deploy.dlq"
	RoutingKey   = "deploy.run"
)

// DeployMessage 是 API、RabbitMQ 和 Worker 之间传递的发布任务快照
type DeployMessage struct {
	MessageID    string `json:"message_id"`
	DeploymentID int64  `json:"deployment_id"`
	AppID        int64  `json:"app_id"`
	VersionID    int64  `json:"version_id"`
	Attempt      int    `json:"attempt"`
}
