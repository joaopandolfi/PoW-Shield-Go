package domain

import "regexp"

type Waf struct {
	ID       int    `json:"id"`
	Reg      string `json:"reg"`
	Type     int    `json:"type"`
	Cmt      string `json:"cmt"`
	TypeName string `json:"-"`
	Regex    *regexp.Regexp
}
