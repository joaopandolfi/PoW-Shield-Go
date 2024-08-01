package domain

import (
	"time"
)

type Cookie struct {
	Value string

	Path       string
	Domain     string
	Expires    time.Time
	RawExpires string

	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'
	// MaxAge>0 means Max-Age attribute present and given in seconds
	MaxAge   int
	Secure   bool
	HttpOnly bool
}

func (d *Cookie) ToSession() *Session {
	var session Session
	err := session.Unrap(d.Value)
	if err != nil {
		return nil
	}

	return &session
}
