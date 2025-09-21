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
	ActionRabbitmqPublishFailed = "rabbitmq_publish_failed"

	// RabbitMQ-related actions
	ActionRabbitMQConnecting      = "rabbitmq_connecting"
	ActionRabbitMQDisconnected    = "rabbitmq_disconnected"
	ActionRabbitMQReconnecting    = "rabbitmq_reconnecting"
	ActionRabbitMQReconnected     = "rabbitmq_reconnected"
	ActionRabbitMQReconnectFailed = "rabbitmq_reconnect_failed"
	ActionRabbitMQSetup           = "rabbitmq_setup"
	ActionRabbitMQSetupComplete   = "rabbitmq_setup_complete"
	ActionRabbitMQSetupFailed     = "rabbitmq_setup_failed"
	ActionRabbitMQConsumeStarted  = "rabbitmq_consume_started"
	ActionRabbitMQConsumeFailed   = "rabbitmq_consume_failed"
	ActionRabbitMQConnectFailed   = "rabbitmq_connect_failed"
	ActionRabbitMQAckFailed       = "rabbitmq_ack_failed"
	ActionRabbitMQNackFailed      = "rabbitmq_nack_failed"

	// Kitchen worker actions
	ActionWorkerRegistrationFailed = "worker_registration_failed"
	ActionOrderRejected            = "order_rejected"
	ActionOrderProcessingStarted   = "order_processing_started"
	ActionOrderCompleted           = "order_completed"
	ActionHeartbeatSent            = "heartbeat_sent"
	ActionStatusUpdatePublished    = "status_update_published"
	ActionMessageProcessingFailed  = "message_processing_failed" // Added this missing action

	// Notification subscriber actions
	ActionNotificationReceived = "notification_received"

	// Database and response actions
	ActionDBConnectFailed = "db_connect_failed"
	ActionDBQueryFailed   = "db_query_failed"
	ActionResponseFailed  = "response_failed"
	ActionServiceFailed   = "service_failed"
	ActionRequestReceived = "request_received"
)
