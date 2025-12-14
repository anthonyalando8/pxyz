package response

import (
	"encoding/json"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ✅ Global proto marshaler with enum strings
var protoMarshaler = protojson.MarshalOptions{
	UseEnumNumbers:  false, // Use string names instead of numbers
	EmitUnpopulated: false, // Don't emit zero values
	UseProtoNames:   false, // Use JSON names (camelCase)
}

// JSON sends a success response, automatically handling proto messages
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := APIResponse{
		Status:  "success",
		Data:   convertData(data), // ✅ Auto-convert proto if needed
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// Error sends an error response
func Error(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := APIResponse{
		Status:  "error",
		Message: msg,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// ✅ convertData checks if data is a proto message and converts it
func convertData(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	// Check if it's a proto message
	if msg, ok := data.(proto.Message); ok {
		// Convert proto to JSON-friendly map
		jsonBytes, err := protoMarshaler.Marshal(msg)
		if err != nil {
			// If conversion fails, return original data
			return data
		}

		// Unmarshal to map for proper JSON encoding
		var result map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			return data
		}
		return result
	}

	// Check if it's a slice of proto messages
	if slice, ok := data.([]proto.Message); ok {
		converted := make([]interface{}, len(slice))
		for i, item := range slice {
			converted[i] = convertData(item)
		}
		return converted
	}

	// Return as-is for non-proto data
	return data
}

// ✅ JSONProto explicitly sends a proto message (alternative method)
func JSONProto(w http.ResponseWriter, status int, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Marshal proto with enum strings
	jsonBytes, err := protoMarshaler.Marshal(msg)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to marshal response")
		return
	}

	// Unmarshal to APIResponse structure
	var data interface{}
	if err := json. Unmarshal(jsonBytes, &data); err != nil {
		Error(w, http.StatusInternalServerError, "Failed to process response")
		return
	}

	resp := APIResponse{
		Status:  "success",
		Data:   data,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// ✅ JSONRaw sends raw proto JSON (without APIResponse wrapper)
func JSONRaw(w http.ResponseWriter, status int, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	jsonBytes, err := protoMarshaler.Marshal(msg)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to marshal response")
		return
	}

	w.Write(jsonBytes)
}

// MarshalToJSON - standalone helper (kept for backward compatibility)
func MarshalToJSON(msg proto.Message) ([]byte, error) {
	return protoMarshaler.Marshal(msg)
}