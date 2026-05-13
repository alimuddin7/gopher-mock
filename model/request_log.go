package model

import "time"

// RequestLog captures a single API request/response cycle.
type RequestLog struct {
	ID         string            `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	StatusCode int               `json:"statusCode"`
	DurationMs float64           `json:"durationMs"`
	ReqHeaders map[string]string `json:"reqHeaders"`
	ReqBody    interface{}       `json:"reqBody,omitempty"`
	ResBody    interface{}       `json:"resBody,omitempty"`
	Matched    bool              `json:"matched"`
	Error      string            `json:"error,omitempty"`
}
