// internal/provider/mpesa/b2b_callback.go (NEW FILE)
package mpesa

import (
	"encoding/json"
	"fmt"
)

// ParseB2BCallbackFlexible parses M-Pesa B2B callback with flexible ResultParameter handling
func (m *MpesaProvider) ParseB2BCallbackFlexible(payload []byte) (*CallbackResult, error) {
	// Parse into generic map first
	var rawCallback map[string]interface{}
	if err := json.Unmarshal(payload, &rawCallback); err != nil {
		return nil, fmt.Errorf("failed to parse callback: %w", err)
	}

	// Extract Result
	resultMap, ok := rawCallback["Result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Result field not found or invalid")
	}

	// Build callback result
	result := &CallbackResult{
		RawData: make(map[string]interface{}),
	}

	// Extract basic fields
	if val, ok := resultMap["ResultCode"].(float64); ok {
		result.ResultCode = fmt.Sprintf("%.0f", val)
		result.Success = int(val) == 0
	}
	
	if val, ok := resultMap["ResultDesc"].(string); ok {
		result.ResultDescription = val
	}
	
	if val, ok := resultMap["ConversationID"].(string); ok {
		result.ConversationID = val
	}
	
	if val, ok := resultMap["TransactionID"].(string); ok {
		result.TransactionID = val
		result.ProviderTxID = val
	}

	// Extract ResultParameters
	if resultParams, ok := resultMap["ResultParameters"].(map[string]interface{}); ok {
		if resultParam, ok := resultParams["ResultParameter"]; ok {
			// Handle array format
			if paramArray, ok := resultParam.([]interface{}); ok {
				for _, item := range paramArray {
					if paramItem, ok := item.(map[string]interface{}); ok {
						key, _ := paramItem["Key"].(string)
						value := paramItem["Value"]
						
						if key != "" {
							result.RawData[key] = value
							
							// Extract specific fields
							switch key {
							case "TransactionAmount":
								if val, ok := value.(float64); ok {
									result.  Amount = val
								}
							case "TransactionReceipt": 
								if val, ok := value.(string); ok {
									result.ProviderTxID = val
								}
							}
						}
					}
				}
			} else if paramMap, ok := resultParam.(map[string]interface{}); ok {
				// Handle object format
				result.RawData = paramMap
				
				// Extract specific fields
				if val, ok := paramMap["TransactionAmount"].(float64); ok {
					result.Amount = val
				}
				if val, ok := paramMap["TransactionReceipt"].(string); ok {
					result.ProviderTxID = val
				}
			}
		}
	}

	return result, nil
}