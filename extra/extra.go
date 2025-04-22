package extra

import (
	"bufio"
	"os"
	"regexp"
	"unicode"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/store"
	"golang.org/x/text/unicode/norm"
)

var sanitize = regexp.MustCompile(`[\x00-\x08\x0B-\x1F\x7F]|\x1B\[[0-9;]*[a-zA-Z]`)

func SanitizeOutput(input string, removeNewlines bool, file *os.File) {
	// Remove ASCII control characters and ANSI escape sequences
	cleaned := sanitize.ReplaceAllString(input, "")

	// Create a normalizing writer that writes to the file
	writer := norm.NFC.Writer(file)

	// Process and write runes directly
	for _, r := range cleaned {
		if r == '\n' {
			if removeNewlines {
				writer.Write([]byte{' '})
				continue
			}
			writer.Write([]byte{'\n'})
		} else if unicode.IsPrint(r) || unicode.IsSpace(r) {
			// Write the rune directly to the normalizing writer
			writer.Write([]byte(string(r)))
		}
	}

	// Ensure everything is flushed
	writer.Close()
}

func Echoln(file *os.File, prefix string) func(msg lingograph.Message) {
	return func(msg lingograph.Message) {
		SanitizeOutput(prefix, false, file)
		SanitizeOutput(msg.Content, false, file)
		file.Write([]byte{'\n'})
		file.Sync()
	}
}

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
