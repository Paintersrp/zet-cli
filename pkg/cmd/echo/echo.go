package echo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"github.com/Paintersrp/an/internal/config"
	"github.com/Paintersrp/an/internal/note"
	"github.com/Paintersrp/an/internal/state"
	"github.com/Paintersrp/an/pkg/shared/flags"
)

func NewCmdEcho(s *state.State) *cobra.Command {
	var auto bool

	cmd := &cobra.Command{
		Use:     "echo [message] --name [pin-name] --template [template-name] --auto --paste",
		Aliases: []string{"e"},
		Short:   "Append a message to the pinned file or from the clipboard if --paste is set.",
		Long: heredoc.Doc(`
			The echo command appends a message to the pinned file or from the clipboard if --paste is set.
			If no file is pinned, it returns an error.
			If the --auto flag is set, an autogenerated file will be used instead of a pinned file.
			If the --paste flag is set, the current clipboard content is used as the message.

			Examples:
			  an echo "Add this to the default pinned file."
			  an echo "Add this to named pin (messages-pin) file." --name messages-pin
			  an echo "Add this to autogenerated file." --auto
			  an echo -a --paste  // Add clipboard contents to autogenerated file
			  an e -a -p --template feature // Add clipboard contents to autogenerated file with template
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args, s, auto)
		},
	}

	cmd.Flags().
		BoolVarP(&auto, "auto", "a", false, "Generate an autogenerated note rather than using a pin")

	flags.AddPaste(cmd)
	flags.AddTemplate(cmd, "echo")
	flags.AddName(cmd, "Named pin to target")

	return cmd
}

func run(
	cmd *cobra.Command,
	args []string,
	s *state.State,
	auto bool,
) error {
	tmpl := flags.HandleTemplate(cmd)
	paste, err := flags.HandlePaste(cmd)
	if err != nil {
		return err
	}

	name, err := flags.HandleName(cmd)
	if err != nil {
		return err
	}

	message, err := prepareMessage(args, paste)
	if err != nil {
		return err
	}

	title, targetPin, err := determineTargetPin(name, auto, s.Config)
	if err != nil {
		return err
	}

	if auto {
		err = createAutoGeneratedNote(title, targetPin, message, tmpl, s)
	} else {
		err = appendMessageToPin(targetPin, message)
	}

	if err != nil {
		return err
	}
	printResult(auto, name, targetPin)
	return nil
}

func prepareMessage(args []string, paste bool) (string, error) {
	if paste {
		msg, err := clipboard.ReadAll()
		if err != nil {
			return "", fmt.Errorf("error reading from clipboard: %s", err)
		}
		if msg == "" {
			return "", errors.New("clipboard is empty")
		}
		return msg, nil
	}

	if len(args) == 0 {
		return "", errors.New("no message provided and --paste not set")
	}
	return strings.Join(args, " "), nil
}

func determineTargetPin(
	name string,
	auto bool,
	c *config.Config,
) (string, string, error) {
	if auto {
		title, targetPin := generateFileName(c)
		return title, targetPin, nil
	}

	if name != "" {
		pin := c.NamedPins[name]
		if pin == "" {
			return "", "", fmt.Errorf("no file pinned for the name '%s'", name)
		}
		return "", pin, nil
	}

	if c.PinnedFile == "" {
		return "", "", errors.New("no file pinned")
	}
	return "", c.PinnedFile, nil
}

func generateFileName(cfg *config.Config) (string, string) {
	baseDir := filepath.Join(cfg.VaultDir, "echoes")

	err := os.MkdirAll(baseDir, 0o755)
	if err != nil {
		fmt.Println("Error creating directory:", err)
	}

	ctx := extractContext()
	d := time.Now().Format("01022006_150405")
	fileName := fmt.Sprintf("echo_%s_%s", d, ctx)

	return fileName, filepath.Join(baseDir, fileName+".md")
}

func extractContext() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "default"
	}
	return filepath.Base(cwd)
}

// TODO: target pin?
func createAutoGeneratedNote(
	title, targetPin, message, tmpl string,
	s *state.State,
) error {
	n := note.NewZettelkastenNote(
		s.Vault,
		"echoes",
		title,
		[]string{"echo"},
		nil,
		"",
	)

	conflict := n.HandleConflicts()
	if conflict != nil {
		return fmt.Errorf("%s", conflict)
	}

	_, err := n.Create(tmpl, s.Templater, message)
	if err != nil {
		return fmt.Errorf("error creating note file: %s", err)
	}

	// return appendMessageToPin(targetPin, message)
	return nil
}

func appendMessageToPin(targetPin, message string) error {
	file, err := os.OpenFile(targetPin, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("error opening file: %s", err)
	}
	defer file.Close()

	_, err = file.WriteString(message)
	if err != nil {
		return fmt.Errorf("error writing to file: %s", err)
	}
	return nil
}

func printResult(auto bool, name, targetPin string) {
	if auto {
		fmt.Printf("Message appended to the autogenerated file %s.\n", targetPin)
	} else if name != "" {
		fmt.Printf("Message appended to the named pinned file %s.\n", name)
	} else {
		fmt.Println("Message appended to the pinned file.")
	}
}
