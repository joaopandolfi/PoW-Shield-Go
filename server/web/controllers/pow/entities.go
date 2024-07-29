package pow

type problemResponsePayload struct {
	Prefix string `json:"prefix"`
}

type verifyChallengePayload struct {
	Buffer     string `json:"buffer" validate:"required"`
	Difficulty int    `json:"difficulty" validate:"required"`
	Prefix     string `json:"difficulty" validate:"required"`
}
