package pow

import (
	"encoding/json"
	"pow-shield-go/models/domain"
	"pow-shield-go/services/utils"
)

type problemResponsePayload struct {
	Prefix     string `json:"prefix"`
	Difficulty int    `json:"difficulty"`
	ID         string `json:"id"`
	Token      string `json:"token"`
}

func (p *problemResponsePayload) FromDomain(c domain.Challenge) {
	p.Prefix = c.Prefix
	p.Difficulty = c.Difficulty
	p.ID = c.Key()
	b, _ := json.Marshal(p)
	p.Token = utils.ToBase64(b)
}

type verifyChallengePayload struct {
	Buffer     string `json:"buffer" validate:"required"`
	Difficulty int    `json:"difficulty" validate:"required"`
	Prefix     string `json:"prefix" validate:"required"`
}
