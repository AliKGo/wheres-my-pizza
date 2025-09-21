package models

import "errors"

var (
	ErrorValidationFailed      = errors.New("validation_failed")
	ErrorDbTransactionFailed   = errors.New("db_transaction_failed")
	ErrorRabbitmqPublishFailed = errors.New("rabbitmq_publish_failed")
)
