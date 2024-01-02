/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

var (
	errNoCommandSpecified = errors.New("no command specified")
	errInvalidArgument    = errors.New("argument provided is invalid")
	errMissingArgument    = errors.New("argument is required but not provided")
)

const zeroDelimiter = "\u0000"
const newLineDelimiter = "\n"

type xargsFlags struct {
	delimiter   string
	maxProcs    int
	maxArgs     int
	replacement string
	exitOnError bool
}

func newXargsFlags() *xargsFlags {
	return &xargsFlags{delimiter: newLineDelimiter, maxProcs: 1, maxArgs: 1}
}

func (f *xargsFlags) parse(args []string) (rest []string, finished bool, err error) {
	if len(args) == 0 {
		return args, false, errNoCommandSpecified
	}

	flag := args[0]
	switch flag {
	case "-0":
		f.delimiter = zeroDelimiter
		return args[1:], false, nil
	case "--exit-on-error":
		f.exitOnError = true
		return args[1:], false, nil
	case "-n", "--max-args":
		f.maxArgs, err = parseNumericArgument(args)
		if err != nil {
			return args, false, fmt.Errorf("-n, --max-args %w", err)
		}
		return args[2:], false, nil
	case "-P", "--max-procs":
		f.maxProcs, err = parseNumericArgument(args)
		if err != nil {
			return args, false, fmt.Errorf("-P, --max-procs %w", err)
		}
		return args[2:], false, err
	case "-I", "-i":
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return args, false, fmt.Errorf("-I, -i %w", errMissingArgument)
		}
		f.replacement = args[1]
		return args[2:], false, err
	default:
		// no recognized flag, so program and arguments must start
		return args, true, nil
	}
}

func parseNumericArgument(args []string) (int, error) {
	if len(args) < 2 || strings.HasPrefix(args[1], "-") {
		return 0, errMissingArgument
	}
	n, err := strconv.Atoi(args[1])
	if err != nil || n < 1 {
		return 0, errInvalidArgument
	}
	return n, nil
}

func (f *xargsFlags) parseFlags(args []string) ([]string, error) {
	finished := false
	var err error
	for !finished {
		args, finished, err = f.parse(args)
		if err != nil {
			return args, err
		}
	}

	if len(f.replacement) != 0 {
		// when a replacement char is provided, parameter -n/--max-args is
		// set to one in original xargs, so we follow that convention
		f.maxArgs = 1
	}

	return args, nil
}

func ExecuteXargs() {
	if len(os.Args) == 1 {
		fmt.Println("Usage: {} xargs [-I <replacement>] [-P <max-procs>] [-n <max-args] <command> [args]\n", os.Args[0])
		return
	}

	flags := newXargsFlags()
	rest, err := flags.parseFlags(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %s\n", err)
		return
	}

	process(rest, flags)
}

func process(args []string, flags *xargsFlags) {
	scanner := bufio.NewScanner(os.Stdin)
	if flags.delimiter == zeroDelimiter {
		scanner.Split(splitByZero)
	}
	argch := make(chan []string, flags.maxProcs)
	outch := make(chan string, flags.maxProcs)

	// TODO implement --exit-on-error
	var wg sync.WaitGroup
	for i := 0; i < flags.maxProcs; i++ {
		wg.Add(1)
		go func() {
			runProgram(argch, outch)
			wg.Done()
		}()
	}

	if flags.maxArgs == 1 {
		go passBySingle(scanner, args, argch, flags.replacement)
	} else {
		go passByMultiple(scanner, args, argch, flags.maxArgs)
	}

	go func() {
		wg.Wait()
		close(outch)
	}()

	for out := range outch {
		if strings.HasSuffix(out, "\n") {
			fmt.Print(out)
		} else {
			fmt.Println(out)
		}
	}
}

// Pass argument read from stdin as a single argument to program by sending to argument channel (argch)
// i.e let cmd to command to be run, then argument channel will be arranged so that command is run like cmd <stdin_arg_1>, cmd <stdin_arg_2>
// args contains command and its command line flags/arguments
func passBySingle(scanner *bufio.Scanner, args []string, argch chan<- []string, replacement string) {
	for scanner.Scan() {
		line := scanner.Text()
		c := make([]string, len(args))
		copy(c, args)
		if len(replacement) == 0 {
			c = append(c, line)
			argch <- c
			continue
		}

		// use replacement
		for i := 0; i < len(args); i++ {
			if strings.Contains(c[i], replacement) {
				c[i] = strings.ReplaceAll(c[i], replacement, line)
			}
		}
		argch <- c
	}
	close(argch)
}

// Pass argument read from stdin as multiple arguments (maxArgs) to program by sending to argument channel (argch)
// i.e let cmd to command to be run, then argument channel will be arranged so that command is run like cmd <stdin_arg_1> <stdin_arg_2> ... <stdin_arg_maxArgs>
// args contains command and its command line flags/arguments
func passByMultiple(scanner *bufio.Scanner, args []string, argch chan<- []string, maxArgs int) {
	passArgs := make([]string, 0, maxArgs)
	for scanner.Scan() {
		line := scanner.Text()
		passArgs = append(passArgs, line)

		if len(passArgs) == maxArgs {
			c := make([]string, len(args), len(args)+maxArgs)
			copy(c, args)
			c = append(c, passArgs...)
			argch <- c
			passArgs = passArgs[:0]
		}
	}

	if len(passArgs) != 0 {
		c := make([]string, len(args), len(args)+len(passArgs))
		copy(c, args)
		c = append(c, passArgs...)
		argch <- c
	}

	close(argch)
}

func splitByZero(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF || len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\u0000'); i >= 0 {
		return i + 1, data[0:i], nil
	}
	// eof, return all
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func runProgram(argch <-chan []string, outch chan<- string) error {
	for {
		commandAndArgs, open := <-argch
		if !open {
			return nil
		}

		command := exec.Command(commandAndArgs[0], commandAndArgs[1:]...)
		command.Stdin, _ = os.Open(os.DevNull)
		var out strings.Builder
		command.Stdout = &out
		err := command.Run()
		if err != nil {
			return err
		}

		outch <- out.String()
	}
}
