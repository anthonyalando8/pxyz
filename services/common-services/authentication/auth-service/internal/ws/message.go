package ws

type Message struct {
	Type   string      `json:"type"`
	UserID string       `json:"user_id,omitempty"`
	DeviceID string    `json:"device_id,omitempty"`
	Data   interface{} `json:"data"`
}
