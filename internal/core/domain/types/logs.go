package types

const (
	ActionWorkerRegistered = "worker_registered"
	ActionGracefulShutdown = "graceful_shutdown"

	ActionServiceStarted        = "service_started"
	ActionDBConnected           = "db_connected"
	ActionRabbitMQConnected     = "rabbitmq_connected"
	ActionOrderReceived         = "order_received"
	ActionOrderPublished        = "order_published"
	ActionValidationFailed      = "validation_failed"
	ActionDBTransactionFailed   = "db_transaction_failed"
	ActionRabbitMQPublishFailed = "rabbitmq_publish_failed"
)
