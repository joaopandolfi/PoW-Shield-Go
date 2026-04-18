package pow

import "time"

const defaultCacheDuration = time.Minute * 10
const challengeCacheKeyPrefix = "session:"

func CacheKey(sessionID, prefix string) string {
	if prefix == "" {
		return challengeCacheKeyPrefix + sessionID
	}

	return challengeCacheKeyPrefix + sessionID + ":" + prefix
}
