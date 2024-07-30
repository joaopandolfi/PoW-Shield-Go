package domain

import (
	"fmt"
	"strconv"
	"strings"
)

const CHALLENGE_STATUS_TO_SOLVE = "to-solve"
const CHALLENGE_STATUS_ERROR_COUNT = "error:"

type Challenge struct {
	Requester  string
	Prefix     string
	Difficulty int
	Status     string
}

func (d *Challenge) Key() string {
	return d.Requester
}

func (d *Challenge) ParsePreviousState(state string) {
	if strings.Contains(state, CHALLENGE_STATUS_ERROR_COUNT) {
		splitSate := strings.Split(state, ":")
		diff, _ := strconv.Atoi(splitSate[1])
		d.Difficulty += diff
	}
}

func (d *Challenge) IncreaseDifficulty(state string) string {
	if strings.Contains(state, CHALLENGE_STATUS_ERROR_COUNT) {
		splitSate := strings.Split(state, ":")
		diff, _ := strconv.Atoi(splitSate[1])
		d.Difficulty += diff
	}

	return fmt.Sprintf("%s%d", CHALLENGE_STATUS_ERROR_COUNT, d.Difficulty)
}
