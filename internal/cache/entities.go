package cache

import "time"

type stored struct {
	value   interface{}
	validAt time.Time
}
