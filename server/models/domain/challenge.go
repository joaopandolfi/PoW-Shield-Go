package domain

import (
	"fmt"
	"pow-shield-go/config"
	"strconv"
	"strings"
)

const CHALLENGE_STATUS_TO_SOLVE = "to-solve:"
const CHALLENGE_STATUS_ERROR_COUNT = "error:"
const CHALLENGE_STATUS_VERIFIED = "verified:"

type Challenge struct {
	Requester  string
	Prefix     string
	Difficulty int
	Status     string
}

func NewChallenge() Challenge {
	return Challenge{
		Difficulty: config.Get().Pow.DefaultPrefixSize,
		Status:     fmt.Sprintf("%s%d", CHALLENGE_STATUS_TO_SOLVE, config.Get().Pow.DefaultPrefixSize),
	}
}

func (d *Challenge) Key() string {
	return d.Requester
}

func (d *Challenge) ParsePreviousState(state string) {
	d.Status = state
	if strings.Contains(state, CHALLENGE_STATUS_ERROR_COUNT) || strings.Contains(state, CHALLENGE_STATUS_TO_SOLVE) {
		d.ParseState(0)
	}
}

func (d *Challenge) ParseState(punishment int) {
	if d.Status == "" {
		d.Status = fmt.Sprintf("%s%d", CHALLENGE_STATUS_TO_SOLVE, d.Difficulty)
	}
	d.applyPunishment(d.Status, punishment)
}

func (d *Challenge) IncreaseDifficulty(state string, punishment int) string {
	if !strings.Contains(state, CHALLENGE_STATUS_ERROR_COUNT) {
		d.Status = strings.ReplaceAll(state, CHALLENGE_STATUS_TO_SOLVE, CHALLENGE_STATUS_ERROR_COUNT)
		d.Status = strings.ReplaceAll(d.Status, CHALLENGE_STATUS_VERIFIED, CHALLENGE_STATUS_ERROR_COUNT)
	}
	return d.applyPunishment(d.Status, punishment)
}

func (d *Challenge) RegisterSuccess(nonce string, punishment int) string {
	if strings.Contains(d.Status, CHALLENGE_STATUS_VERIFIED) {
		s := fmt.Sprintf("%s:%s", d.applyPunishment(d.Status, punishment), nonce)
		return s
	}

	d.ParseState(0)
	d.Status = fmt.Sprintf("%s%d:%s", CHALLENGE_STATUS_VERIFIED, d.Difficulty, nonce)
	return d.Status
}

func VerifyChallengeState(state, nonce string) bool {
	splitSate := strings.Split(state, ":")
	if len(splitSate) < 3 {
		return false
	}

	return splitSate[2] == nonce
}

// ==== Privates

func (d *Challenge) applyPunishment(state string, punishment int) string {
	splitSate := strings.Split(state, ":")
	if len(splitSate) == 0 {
		return state
	}
	lastCount, _ := strconv.Atoi(splitSate[1])
	d.Difficulty = lastCount + punishment
	d.Status = fmt.Sprintf("%s:%d", splitSate[0], d.Difficulty)
	return d.Status
}
