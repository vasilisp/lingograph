package extra

import (
	"bufio"
	"os"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/store"
)

func Stdin() lingograph.Actor {
	return lingograph.NewActor(lingograph.User, func(history []lingograph.Message, r store.Store) (string, error) {
		reader := bufio.NewReader(os.Stdin)

		text, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		return text, nil
	})
}
