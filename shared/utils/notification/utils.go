package utils

func NormalizePayload(input map[string]interface{}) map[string]interface{} {
    fixed := make(map[string]interface{}, len(input))
    for k, v := range input {
        switch vv := v.(type) {
        case []string:
            arr := make([]interface{}, len(vv))
            for i, s := range vv {
                arr[i] = s
            }
            fixed[k] = arr
        default:
            fixed[k] = v
        }
    }
    return fixed
}