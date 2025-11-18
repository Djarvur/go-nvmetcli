package nvmet

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
)

var errUnknownCommand = errors.New("unknown command")

// Shell provides an interactive shell for managing NVMe targets.
type Shell struct {
	root   *Root
	rl     *readline.Instance
	prompt string
	path   string
	exit   bool
}

// NewShell creates a new shell instance.
func NewShell(historyFile string) (*Shell, error) {
	var historyPath string
	if historyFile != "" {
		historyPath = historyFile
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err //nolint:wrapcheck // os.UserHomeDir error is clear enough
		}

		historyPath = filepath.Join(home, ".nvmetcli_history")
	}

	//nolint:exhaustruct,varnamelen,lll // readline.Config has many optional fields, we set only required ones; rl is a standard abbreviation for readline
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "/ # ",
		HistoryFile:     historyPath,
		AutoComplete:    nil,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return nil, err //nolint:wrapcheck // readline.NewEx error is clear enough
	}

	root, err := NewRoot()
	if err != nil {
		rl.Close()

		return nil, err
	}

	shell := &Shell{
		root:   root,
		rl:     rl,
		prompt: "/ # ",
		path:   "/",
		exit:   false,
	}

	return shell, nil
}

// RunInteractive runs the interactive shell loop.
func (s *Shell) RunInteractive() error {
	for !s.exit {
		line, err := s.rl.Readline()
		if err != nil {
			if errors.Is(err, readline.ErrInterrupt) {
				continue
			}

			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if err := s.execute(line); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}

	return nil
}

//nolint:gocyclo,cyclop,funlen // function complexity is necessary for command parsing
func (s *Shell) execute(line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	//nolint:goconst // "exit" is a command string, not a constant
	case "exit", "quit":
		s.exit = true

		return nil

	//nolint:goconst // "help" is a command string, not a constant
	case "help":
		s.printHelp()

		return nil

	//nolint:goconst // "save" is a command string, not a constant
	case "save":
		savefile := ""
		if len(args) > 0 {
			savefile = args[0]
		}

		return s.root.SaveToFile(savefile)

	//nolint:goconst // "restore" is a command string, not a constant
	case "restore":
		clearExisting := false

		savefile := ""
		if len(args) > 0 {
			savefile = args[0]
			//nolint:goconst // "clear_existing" is a command argument string, not a constant
			if len(args) > 1 && args[1] == "clear_existing" {
				clearExisting = true
			}
		}

		errors, err := s.root.RestoreFromFile(savefile, clearExisting, false)
		if err != nil {
			return err
		}

		if len(errors) > 0 {
			fmt.Fprintf(os.Stderr, "Configuration restored with %d errors:\n", len(errors))

			for _, e := range errors {
				fmt.Fprintf(os.Stderr, "  %s\n", e)
			}
		}

		return nil

	//nolint:goconst // "clear" is a command string, not a constant
	case "clear":
		return s.root.ClearExisting()

	case "ls", "list":
		return s.listCurrent()

	case "cd":
		return s.changeDirectory(args)

	case "pwd":
		//nolint:forbidigo // CLI output to stdout is acceptable
		fmt.Println(s.path)

		return nil

	default:
		return fmt.Errorf("unknown command: %s (type 'help' for help): %w", cmd, errUnknownCommand)
	}
}

func (s *Shell) printHelp() {
	helpText := `
Available commands:
  help              - Show this help message
  exit, quit        - Exit the shell
  save [file]       - Save current configuration to file
  restore [file]    - Restore configuration from file
  clear             - Clear all existing configuration
  ls, list          - List current directory contents
  cd <path>         - Change directory
  pwd               - Print current working directory

For more detailed help, see the documentation.
`
	//nolint:forbidigo // CLI output to stdout is acceptable
	fmt.Print(helpText)
}

func (s *Shell) listCurrent() error {
	//nolint:forbidigo // CLI output to stdout is acceptable
	fmt.Println(s.path + ":")
	// Simplified listing - would need to implement full tree navigation
	return nil
}

func (s *Shell) changeDirectory(args []string) error {
	if len(args) == 0 {
		s.path = "/"
		s.rl.SetPrompt("/ # ")

		return nil
	}

	newPath := args[0]
	if strings.HasPrefix(newPath, "/") {
		s.path = newPath
	} else {
		s.path = filepath.Join(s.path, newPath)
	}

	s.prompt = s.path + " # "
	s.rl.SetPrompt(s.prompt)

	return nil
}

// Close closes the shell and releases resources.
func (s *Shell) Close() error {
	if s.rl != nil {
		return s.rl.Close() //nolint:wrapcheck // readline.Close error is clear enough
	}

	return nil
}

// SimpleShell provides a basic shell interface using bufio.
type SimpleShell struct {
	root *Root
	exit bool
}

// NewSimpleShell creates a simple shell using bufio.
func NewSimpleShell() (*SimpleShell, error) {
	root, err := NewRoot()
	if err != nil {
		return nil, err
	}

	return &SimpleShell{
		root: root,
		exit: false,
	}, nil
}

// Run runs the simple shell.
func (s *SimpleShell) Run() error {
	scanner := bufio.NewScanner(os.Stdin)

	//nolint:forbidigo // CLI output to stdout is acceptable
	fmt.Print("/ # ")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			//nolint:forbidigo // CLI output to stdout is acceptable
			fmt.Print("/ # ")

			continue
		}

		if err := s.execute(line); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

		if s.exit {
			break
		}

		//nolint:forbidigo // CLI output to stdout is acceptable
		fmt.Print("/ # ")
	}

	return scanner.Err() //nolint:wrapcheck // scanner.Err error is clear enough
}

//nolint:gocyclo,cyclop,funlen // function complexity is necessary for command parsing
func (s *SimpleShell) execute(line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "exit", "quit":
		s.exit = true

		return nil

	case "help":
		s.printHelp()

		return nil

	case "save":
		savefile := ""
		if len(args) > 0 {
			savefile = args[0]
		}

		if err := s.root.SaveToFile(savefile); err != nil {
			return err
		}

		//nolint:forbidigo // CLI output to stdout is acceptable
		fmt.Println("Configuration saved")

		return nil

	case "restore":
		clearExisting := false

		savefile := ""
		if len(args) > 0 {
			savefile = args[0]

			if len(args) > 1 && args[1] == "clear_existing" {
				clearExisting = true
			}
		}

		errors, err := s.root.RestoreFromFile(savefile, clearExisting, false)
		if err != nil {
			return err
		}

		if len(errors) > 0 {
			fmt.Fprintf(os.Stderr, "Configuration restored with %d errors:\n", len(errors))

			for _, e := range errors {
				fmt.Fprintf(os.Stderr, "  %s\n", e)
			}
		} else {
			//nolint:forbidigo // CLI output to stdout is acceptable
			fmt.Println("Configuration restored successfully")
		}

		return nil

	case "clear":
		if err := s.root.ClearExisting(); err != nil {
			return err
		}

		//nolint:forbidigo // CLI output to stdout is acceptable
		fmt.Println("Configuration cleared")

		return nil

	default:
		return fmt.Errorf("unknown command: %s (type 'help' for help): %w", cmd, errUnknownCommand)
	}
}

func (s *SimpleShell) printHelp() {
	helpText := `
Available commands:
  help              - Show this help message
  exit, quit        - Exit the shell
  save [file]       - Save current configuration to file
  restore [file]    - Restore configuration from file
  clear             - Clear all existing configuration

For more detailed help, see the documentation.
`
	//nolint:forbidigo // CLI output to stdout is acceptable
	fmt.Print(helpText)
}
