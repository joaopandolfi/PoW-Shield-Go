package middleware

func Setup() {
	InitWaf()
	InitPow()
	InitRateLimiter()
}
