package cache

import (
	"time"

	"github.com/joaopandolfi/blackwhale/utils"
)

type cacheInjectable interface {
	inject(c Cache)
}

// lateInitCache is ised to inject cache on struct after a signal
func lateInitCache(c cacheInjectable) {
	if err := recover(); err != nil {
		utils.Debug("[CACHE][Async loading] waiting for ready cache signal")
		wait := make(chan bool, 1)
		AddInitializedListenner(wait)
		go func() {
			ticker := time.NewTicker(time.Second * 40)
			defer ticker.Stop()

			select {
			case <-wait:
				c.inject(Get())
				utils.Debug("[CACHE][Async loading] cache initialized")
				close(wait)
			case <-ticker.C:
				utils.CriticalError("[CACHE][Async loading] Wait cache time reached")
				panic("Wait cache time reached")
			}
		}()
	}
}
