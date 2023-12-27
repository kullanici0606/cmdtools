/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

// duCmd represents the du command
var duCmd = &cobra.Command{
	Use:   "du",
	Short: "Disk usage",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		executeDu(args)
	},
}

var humanReadable bool

func init() {
	rootCmd.AddCommand(duCmd)
	duCmd.Flags().BoolVarP(&humanReadable, "human-readable", "H", false, "print sizes in human readable format (e.g., 1K 234M 2G)")
}

func executeDu(args []string) {
	root := "."
	switch len(args) {
	case 0:
		// noop
	case 1:
		root = args[0]
	}

	total, ok := diskUsageWalkDir(root)
	if ok {
		if humanReadable {
			fmt.Printf("%s %s\n", humanize.Bytes(uint64(total)), root)
			return
		}

		fmt.Printf("%d %s\n", total, root)
	}
}

type dirInfo struct {
	parent  string
	entries []fs.DirEntry
}

type list[T any] struct {
	elem *T
	next *list[T]
}

type queue[T any] struct {
	head *list[T]
	tail *list[T]
}

func (q *queue[T]) push(t *T) {
	if q.head == nil {
		q.head = &list[T]{elem: t, next: nil}
		q.tail = q.head
		return
	}
	q.tail.next = &list[T]{elem: t, next: nil}
	q.tail = q.tail.next
}

func (q *queue[T]) pop() *T {
	t := q.head.elem
	q.head = q.head.next
	return t
}

func (q *queue[T]) isEmpty() bool {
	return q.head == nil
}

// Calculate disk usage by pipelining i.e one goroutine traverse directories
// and enqueues each entry it finds, another goroutine deques entries from queue and calculates file size
func diskUsagePipelined(folder string) (int64, error) {
	f, err := os.Open(folder)
	if err != nil {
		return 0, err
	}

	ch := make(chan string, 10)
	var total int64

	go func() {
		for {
			path, open := <-ch
			if !open {
				break
			}

			info, err := os.Stat(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Cannot stat file '%s': %s\n", path, err)
				continue
			}
			total += info.Size()
		}
	}()

	entries, err := f.ReadDir(-1)
	if err != nil {
		close(ch)
		return 0, err
	}

	dirq := new(queue[dirInfo])

	dirq.push(&dirInfo{parent: folder, entries: entries})

	for !dirq.isEmpty() {
		dir := dirq.pop()
		for _, entry := range dir.entries {
			if (entry.Type().Type() & os.ModeSymlink) != 0 {
				continue
			}

			child := filepath.Join(dir.parent, entry.Name())
			if entry.IsDir() {
				f, err := os.Open(child)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Cannot open directory '%s': %s\n", child, err)
					continue
				}
				subentries, err := f.ReadDir(-1)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Cannot list directory '%s': %s\n", child, err)
					continue
				}
				dirq.push(&dirInfo{parent: child, entries: subentries})
			}

			ch <- child
		}
	}

	close(ch)
	return total, nil
}

// Calculate disk usage using WalkDir
func diskUsageWalkDir(folder string) (int64, bool) {
	var total int64
	filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read '%s': %s\n", path, err)
			return nil
		}

		if (d.Type().Type() & os.ModeSymlink) != 0 {
			return nil
		}

		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot stat file '%s': %s\n", path, err)
			return nil
		}
		total += info.Size()

		return nil
	})

	return total, true
}

// Calculates folder size recursively
func diskUsage(folder string) (int64, bool) {
	f, err := os.Open(folder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open directory '%s': %s\n", folder, err)
		return 0, false
	}

	entries, err := f.ReadDir(-1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot list directory '%s': %s\n", folder, err)
		return 0, false
	}

	var total int64
	for _, entry := range entries {
		if (entry.Type().Type() & os.ModeSymlink) != 0 {
			continue
		}

		if entry.IsDir() {
			subdir := filepath.Join(folder, entry.Name())
			subtotal, ok := diskUsage(subdir)
			if ok {
				total += subtotal
			}
		}

		info, err := entry.Info()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot stat file '%s': %s\n", filepath.Join(folder, entry.Name()), err)
			continue
		}
		total += info.Size()
	}

	return total, true
}
