package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Djarvur/go-nvmetcli/internal/nvmet"
)

var errConfigurationRestoredWithErrors = errors.New("configuration restored with errors")

func usage() {
	fmt.Fprintf(os.Stderr, "syntax: %s save [file_to_save_to]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "        %s restore [file_to_restore_from] [clear_existing]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "        %s clear\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "        %s [interactive mode]\n", os.Args[0])
	os.Exit(1)
}

func save(toFile string) error {
	root, err := nvmet.NewRoot()
	if err != nil {
		return err //nolint:wrapcheck // main function propagates errors to stderr, no need to wrap
	}

	if err := root.SaveToFile(toFile); err != nil {
		return err //nolint:wrapcheck // main function propagates errors to stderr, no need to wrap
	}

	//nolint:forbidigo // CLI output to stdout is acceptable
	fmt.Printf("Configuration saved to %s\n", toFile)

	return nil
}

func restore(fromFile string, clearExisting bool) error {
	root, err := nvmet.NewRoot()
	if err != nil {
		return err //nolint:wrapcheck // main function propagates errors to stderr, no need to wrap
	}

	var pathErr *os.PathError

	restoreErrors, err := root.RestoreFromFile(fromFile, clearExisting, false)
	if err != nil {
		if errors.As(err, &pathErr) {
			// Not an error if the restore file is not present
			//nolint:forbidigo // CLI output to stdout is acceptable
			fmt.Printf("No saved config file at %s, ok, exiting\n", fromFile)

			return nil
		}

		return err //nolint:wrapcheck // main function propagates errors to stderr, no need to wrap
	}

	if len(restoreErrors) > 0 {
		for _, e := range restoreErrors {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}

		return fmt.Errorf("configuration restored with %d errors: %w", len(restoreErrors), errConfigurationRestoredWithErrors)
	}

	//nolint:forbidigo // CLI output to stdout is acceptable
	fmt.Println("Configuration restored successfully")

	return nil
}

func clearConfig() error {
	root, err := nvmet.NewRoot()
	if err != nil {
		return err //nolint:wrapcheck // main function propagates errors to stderr, no need to wrap
	}

	if err := root.ClearExisting(); err != nil {
		return err //nolint:wrapcheck // main function propagates errors to stderr, no need to wrap
	}

	//nolint:forbidigo // CLI output to stdout is acceptable
	fmt.Println("Configuration cleared")

	return nil
}

//nolint:gocyclo,cyclop,funlen // main function has natural complexity due to command-line argument parsing
func main() {
	// Check if running as root (required for configfs access)
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "%s: must run as root.\n", os.Args[0])
		os.Exit(1)
	}

	//nolint:mnd // 3 is a standard CLI argument count limit
	if len(os.Args) > 3 {
		usage()
	}

	// Handle non-interactive commands
	if len(os.Args) == 2 || len(os.Args) == 3 { //nolint:nestif,lll // nested if structure is clear and readable for CLI argument handling; 2 and 3 are standard CLI argument positions
		if os.Args[1] == "--help" || os.Args[1] == "-h" {
			usage()
		}

		var savefile string
		//nolint:mnd // 3 is standard CLI argument position for optional filename
		if len(os.Args) == 3 {
			savefile = os.Args[2]
		}

		switch os.Args[1] {
		case "save":
			if err := save(savefile); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			return

		case "restore":
			clearExisting := false
			//nolint:mnd // 3 is standard CLI argument position for optional filename
			if len(os.Args) == 3 && os.Args[2] == "clear_existing" {
				clearExisting = true
			} else if len(os.Args) == 3 {
				savefile = os.Args[2]
			}

			if err := restore(savefile, clearExisting); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			return

		case "clear":
			if err := clearConfig(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			return
		}
	}

	// Interactive mode
	shell, err := nvmet.NewSimpleShell()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := shell.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
