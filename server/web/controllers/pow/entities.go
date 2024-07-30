package pow

import "pow-shield-go/models/domain"

type problemResponsePayload struct {
	Prefix     string `json:"prefix"`
	Difficulty int    `json:"difficulty"`
}

func (p *problemResponsePayload) FromDomain(c domain.Challenge) {
	p.Prefix = c.Prefix
	p.Difficulty = c.Difficulty
}

type verifyChallengePayload struct {
	Buffer     string `json:"buffer" validate:"required"`
	Difficulty int    `json:"difficulty" validate:"required"`
	Prefix     string `json:"prefix" validate:"required"`
}
