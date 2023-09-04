package lib

type ApplicationRequest struct {
	DestinationTopic string `json:"destinationTopic"`
	Body             string `json:"body"`
	RetryIntervalSec int    `json:"retryIntervalSec"`
	MaxRetry         int    `json:"maxRetry"`
}

type HealthCheckRequest struct {
}
