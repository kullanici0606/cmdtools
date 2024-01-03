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

	"golang.org/x/sync/errgroup"
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

type cmdRunner interface {
	runAsync(argch <-chan []string)
	waitAsync()
}

type regularRunner struct {
	wg    sync.WaitGroup
	outch chan<- string
	errch chan<- string
}

func (r *regularRunner) runAsync(argch <-chan []string) {
	r.wg.Add(1)
	go func() {
		err := r.runProgram(argch)
		if err != nil {
			fmt.Printf("error: %s", err)
		}
		r.wg.Done()
	}()
}

func (r *regularRunner) waitAsync() {
	r.wg.Wait()
	close(r.outch)
}

func (r *regularRunner) runProgram(argch <-chan []string) error {
	for {
		commandAndArgs, open := <-argch
		if !open {
			return nil
		}

		stdout, stderr := runProgram(commandAndArgs)
		if len(stderr) != 0 {
			r.errch <- stderr
			continue
		}

		r.outch <- stdout
	}
}

type runnerOnExit struct {
	errg  *errgroup.Group
	errch chan string
	outch chan<- string
}

func (r *runnerOnExit) runAsync(argch <-chan []string) {
	r.errg.Go(func() error {
		return r.runProgram(argch)
	})
}

func (r *runnerOnExit) waitAsync() {
	if err := r.errg.Wait(); err != nil {
		r.errch <- fmt.Sprint(err)
	}

	close(r.errch)
	close(r.outch)
}

func (r *runnerOnExit) runProgram(argch <-chan []string) error {
	for {
		commandAndArgs, open := <-argch
		if !open {
			return nil
		}

		stdout, stderr := runProgram(commandAndArgs)
		if len(stderr) != 0 {
			r.errch <- stderr
			return fmt.Errorf("error: %s", stderr)
		}

		r.outch <- stdout
	}
}

func runProgram(commandAndArgs []string) (stdout string, stderr string) {
	command := exec.Command(commandAndArgs[0], commandAndArgs[1:]...)
	command.Stdin, _ = os.Open(os.DevNull)
	var out strings.Builder
	command.Stdout = &out
	var errout strings.Builder
	command.Stderr = &errout
	err := command.Run()
	if err != nil {
		return "", errout.String()
	}

	return out.String(), ""
}

func process(args []string, flags *xargsFlags) {
	scanner := bufio.NewScanner(os.Stdin)
	if flags.delimiter == zeroDelimiter {
		scanner.Split(splitByZero)
	}
	argch := make(chan []string, flags.maxProcs)
	outch := make(chan string, flags.maxProcs)
	errch := make(chan string, flags.maxProcs)

	var runner cmdRunner

	if flags.exitOnError {
		runner = &runnerOnExit{
			errg: new(errgroup.Group), outch: outch, errch: errch,
		}
	} else {
		runner = &regularRunner{
			outch: outch, wg: sync.WaitGroup{}, errch: errch,
		}
	}

	for i := 0; i < flags.maxProcs; i++ {
		runner.runAsync(argch)
	}

	exitch := make(chan struct{})

	if flags.maxArgs == 1 {
		go passBySingle(scanner, args, argch, exitch, flags.replacement)
	} else {
		go passByMultiple(scanner, args, argch, exitch, flags.maxArgs)
	}

	go runner.waitAsync()

loop:
	for {
		select {
		case err, open := <-errch:
			if !open {
				close(exitch)
				break loop
			}

			fmt.Fprintf(os.Stderr, "%s\n", strings.TrimSuffix(err, "\n"))

		case out, open := <-outch:
			if !open {
				close(exitch)
				break loop
			}
			fmt.Printf("%s\n", strings.TrimSuffix(out, "\n"))
		}
	}

}

// Pass argument read from stdin as a single argument to program by sending to argument channel (argch)
// i.e let cmd to command to be run, then argument channel will be arranged so that command is run like cmd <stdin_arg_1>, cmd <stdin_arg_2>
// args contains command and its command line flags/arguments
func passBySingle(scanner *bufio.Scanner, args []string, argch chan<- []string, exitch <-chan struct{}, replacement string) {
	for scanner.Scan() {
		select {
		case <-exitch:
			return
		default:
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
	}
	close(argch)
}

// Pass argument read from stdin as multiple arguments (maxArgs) to program by sending to argument channel (argch)
// i.e let cmd to command to be run, then argument channel will be arranged so that command is run like cmd <stdin_arg_1> <stdin_arg_2> ... <stdin_arg_maxArgs>
// args contains command and its command line flags/arguments
func passByMultiple(scanner *bufio.Scanner, args []string, argch chan<- []string, exitch <-chan struct{}, maxArgs int) {
	passArgs := make([]string, 0, maxArgs)
	for scanner.Scan() {
		select {
		case <-exitch:
			return
		default:
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
