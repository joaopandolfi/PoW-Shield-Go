package domain

type Session struct {
	Authorized bool
	Difficulty int
	Prefix     string
	Buffer     string
}
