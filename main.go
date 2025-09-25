package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/autumnkelsey/gorganize/formatters"
	"github.com/daixiang0/gci/pkg/log"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	debug     bool                  // for unit testing
	formatter *formatters.Formatter = formatters.NewFormatter()
	stdin     bool
)

func main() {
	cmd := &cobra.Command{
		Use:   "gorganize [flags] [path ...]",
		Short: "gorganize formats .go files.",
		Long:  `Formats .go files based on the AIFI software team's coding conventions`,
		RunE:  run,
	}

	fs := cmd.Flags()
	fs.BoolVar(&stdin, "stdin", false, color.GreenString("Use standard input for piping source files"))

	log.InitLogger()

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "gorganize failed: %s", err.Error())
		os.Exit(1)
	}
}

func formatFiles(args []string) error {
	var paths []string
	if debug {
		paths = []string{"./formatters/aifi.go"}
	} else if len(args) == 0 {
		if abs, err := filepath.Abs("."); err != nil {
			return err
		} else {
			paths = []string{abs}
		}
	} else {
		for _, arg := range args {
			if abs, err := filepath.Abs(strings.ReplaceAll(arg, "...", "")); err != nil {
				return err
			} else {
				paths = append(paths, abs)
			}
		}
	}

	for _, path := range paths {
		if err := filepath.Walk(path, func(path string, f fs.FileInfo, err error) error {
			if err != nil {
				return err
			} else if f.IsDir() || strings.HasPrefix(f.Name(), ".") || !strings.HasSuffix(f.Name(), ".go") {
				return nil // not a Go file
			}

			var in *os.File
			defer in.Close()
			if in, err = os.Open(path); err != nil {
				return err
			} else if input, err := io.ReadAll(in); err != nil {
				return err
			} else if output, err := formatter.Format(path, input); err != nil {
				return err
			} else if bytes.Equal(input, output) {
				return nil
			} else {
				var perms os.FileMode
				if fi, err := os.Stat(path); err == nil {
					perms = fi.Mode() & os.ModePerm
				}
				return os.WriteFile(path, output, perms)
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

func formatStdin() error {
	if bytes, err := io.ReadAll(os.Stdin); err != nil {
		return err
	} else if bytes, err = formatter.Format("<standard input>", bytes); err != nil {
		return err
	} else {
		_, err = os.Stdout.Write(bytes)
		return err
	}
}

func run(_ *cobra.Command, args []string) error {
	if stdin {
		return formatStdin()
	}
	return formatFiles(args)
}
