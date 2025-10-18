// --- common/types.go ---
package common

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
