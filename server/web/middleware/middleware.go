package middleware

func Setup() {
	InitWaf()
	InitPow()
	InitRateLimiter()
	InitContract()
}
