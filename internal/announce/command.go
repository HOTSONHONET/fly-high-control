package announce

import (
	"fmt"
	"strconv"
	"strings"
)

const MaxRepeat = 10

type Command struct {
	Repeat  int
	Target  string
	Message string
}

func Parse(text string, allowedTargets map[string]struct{}) (Command, error) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) < 3 {
		return Command{}, fmt.Errorf("usage: /announce <num-times> <target> <message>")
	}

	repeat, err := strconv.Atoi(fields[0])
	if err != nil {
		return Command{}, fmt.Errorf("num-times must be a whole number")
	}
	if repeat < 1 || repeat > MaxRepeat {
		return Command{}, fmt.Errorf("num-times must be between 1 and %d", MaxRepeat)
	}

	target := strings.ToLower(fields[1])
	if target != "all" {
		if _, ok := allowedTargets[target]; !ok {
			return Command{}, fmt.Errorf("unknown target %q", target)
		}
	}

	prefix := fields[0] + " " + fields[1]
	message := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), prefix))
	if message == "" {
		return Command{}, fmt.Errorf("announcement message cannot be empty")
	}

	return Command{Repeat: repeat, Target: target, Message: message}, nil
}
