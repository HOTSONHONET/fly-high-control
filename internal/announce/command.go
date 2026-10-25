package announce

import (
	"fmt"
	"strings"
)

type Command struct {
	Target  string
	Message string
}

func Parse(text string, allowedTargets map[string]struct{}) (Command, error) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) < 2 {
		return Command{}, fmt.Errorf("usage: /announce <target> <message>")
	}

	target := strings.ToLower(fields[0])
	if target != "all" {
		if _, ok := allowedTargets[target]; !ok {
			return Command{}, fmt.Errorf("unknown target %q", target)
		}
	}

	message := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), fields[0]))
	if message == "" {
		return Command{}, fmt.Errorf("announcement message cannot be empty")
	}

	return Command{Target: target, Message: message}, nil
}
