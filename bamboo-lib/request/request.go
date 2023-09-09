package request

type ApplicationRequest struct {
	RequestID  string `json:"requestId"`
	TraceID    string `json:"traceId"`
	MessageID  string `json:"messageId"`
	ReceiverID string `json:"receiverId"`
	Data       []byte `json:"data"`
	Version    int    `json:"version"`
}

type HealthCheckRequest struct {
}
