package extra

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"regexp"
	"unicode"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/pkg/slicev"
	"github.com/vasilisp/lingograph/store"
	"golang.org/x/text/unicode/norm"
)

var sanitize = regexp.MustCompile(`[\x00-\x08\x0B-\x1F\x7F]|\x1B\[[0-9;]*[a-zA-Z]`)

// SanitizeOutput writes sanitized text to the provided writer.
// It removes ASCII control characters and ANSI escape sequences,
// normalizes Unicode text to NFC form, and optionally replaces newlines with
// spaces.
func SanitizeOutput(input string, removeNewlines bool, writer io.Writer) {
	// Remove ASCII control characters and ANSI escape sequences
	cleaned := sanitize.ReplaceAllString(input, "")

	// Create a normalizing writer that writes to the file
	writerNormalizing := norm.NFC.Writer(writer)

	// Process and write runes directly
	for _, r := range cleaned {
		if r == '\n' {
			if removeNewlines {
				writerNormalizing.Write([]byte{' '})
				continue
			}
			writerNormalizing.Write([]byte{'\n'})
		} else if unicode.IsPrint(r) || unicode.IsSpace(r) {
			// Write the rune directly to the normalizing writer
			writerNormalizing.Write([]byte(string(r)))
		}
	}
	writerNormalizing.Close()
}

// SanitizeOutputString returns a sanitized version of the input string.
// It performs the same sanitization as SanitizeOutput but returns the result as
// a string.
func SanitizeOutputString(input string, removeNewlines bool) string {
	writer := bytes.NewBuffer(nil)
	SanitizeOutput(input, removeNewlines, writer)
	return writer.String()
}

// Echoln returns a function that writes messages to a file with a prefix.  The
// returned function can be used as an "echo" callback in pipelines, e.g., for
// writing LLM messages to stdin.
func Echoln(file *os.File, prefix string) func(msg lingograph.Message) {
	return func(msg lingograph.Message) {
		SanitizeOutput(prefix, false, file)
		SanitizeOutput(msg.Content, false, file)
		file.Write([]byte{'\n'})
		file.Sync()
	}
}

// Stdin returns an Actor that reads input from standard input.
// The actor reads a single line of text from stdin and records it as a chat
// message for downstream processing.
func Stdin() lingograph.Actor {
	return lingograph.NewActor(lingograph.User, func(history slicev.RO[lingograph.Message], r store.Store) (string, error) {
		reader := bufio.NewReader(os.Stdin)

		text, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		return text, nil
	})
}
