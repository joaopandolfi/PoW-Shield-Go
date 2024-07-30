package config

import (
	"encoding/json"
	"fmt"
	"os"
)

func ConvertTo[T any](v interface{}) (T, error) {
	var result T

	b, err := json.Marshal(v)
	if err != nil {
		return result, fmt.Errorf("marshaling interface: %w", err)
	}

	err = json.Unmarshal(b, &result)
	if err != nil {
		return result, fmt.Errorf("unmarshaling interface: %w", err)
	}

	return result, nil
}

func StrTo[T any](val string) T {
	var result T
	err := json.Unmarshal([]byte(val), &result)
	if err != nil {
		return result
	}

	return result
}

func getEnvOrDefault(key, _default string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return _default
}
