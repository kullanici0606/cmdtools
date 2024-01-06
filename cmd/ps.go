/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tklauser/go-sysconf"
)

// psCmd represents the ps command
var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "Subset of ps command",
	Run: func(cmd *cobra.Command, args []string) {
		execPs(args)
	},
}

func init() {
	rootCmd.AddCommand(psCmd)
	// psCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

const procfsRoot = "/proc/"

type pidDir []fs.DirEntry

func (p pidDir) compare(i, j int) bool {
	p1, err := strconv.Atoi(p[i].Name())
	if err != nil {
		fmt.Println(err)
		// pids are always integers, a parse error means, there is something really wrong here
		panic("incorrect pid: " + p[i].Name())
	}

	p2, err := strconv.Atoi(p[j].Name())
	if err != nil {
		fmt.Println(err)
		// pids are always integers, a parse error means, there is something really wrong here
		panic("incorrect pid: " + p[i].Name())
	}

	return p1 < p2
}

func execPs(args []string) {
	f, err := os.Open(procfsRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read procfs %s\n", err)
		return
	}

	dirs, err := f.ReadDir(-1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read procfs %s\n", err)
		return
	}

	usernames, err := readUserNames()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read user name pid mappings %s\n", err)
	}

	var pids pidDir
	for _, d := range dirs {
		if _, err := strconv.Atoi(d.Name()); err == nil {
			pids = append(pids, d)
		}
	}

	sort.Slice(pids, pids.compare)
	fmt.Printf("%-8s %s\t%s\t%-17s\t%-8s\t%s\n", "UID", "PID", "PPID", "STIME", "TIME", "NAME")

	for _, entry := range pids {
		err := psProcess(entry.Name(), usernames)
		if errors.Is(err, os.ErrPermission) {
			continue
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read details of %s %s\n", entry.Name(), err)
		}
	}
}

func readUserNames() (map[string]string, error) {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parsePasswdFile(f), nil
}

func parsePasswdFile(f io.Reader) map[string]string {
	scanner := bufio.NewScanner(f)
	usernames := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		// even though in some systems, comments are not allowd in passwd file, we try to be on the safe side
		if line[0] == '#' {
			continue
		}
		cols := strings.Split(line, ":")
		if len(cols[0]) > 8 {
			// truncate long user name as ps does
			usernames[cols[2]] = cols[0][:7] + "+"
		} else {
			usernames[cols[2]] = cols[0]
		}

	}

	return usernames
}

func psProcess(pid string, usernames map[string]string) error {
	status, err := readProcStatus(pid)
	if err != nil {
		return err
	}

	clktck, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	if err != nil {
		return err
	}

	cputime, starttime, err := readProcStat(pid, clktck)
	if err != nil {
		return err
	}

	bootTime, err := readBootTime()
	if err != nil {
		return err
	}

	startTime := bootTime.Add(time.Duration(starttime) * time.Second)
	formattedTime := startTime.Format("2006-01-02 15:04:05")

	fmt.Printf("%-8s %s\t%s\t%s\t%s\t%s\n", usernames[status.uid], status.pid, status.parentPid, formattedTime, formatCpuTime(cputime), status.name)

	return nil
}

func readBootTime() (time.Time, error) {
	uptime := filepath.Join(procfsRoot, "uptime")
	f, err := os.Open(uptime)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()
	return parseBootTime(f, time.Now())
}

func parseBootTime(r io.Reader, now time.Time) (time.Time, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return time.Time{}, err
	}

	// /proc/uptime
	//          This file contains two numbers (values in seconds): the
	//          uptime of the system (including time spent in suspend) and
	//          the amount of time spent in the idle process.
	content := string(b)
	cols := strings.Split(content, " ")
	seconds, err := strconv.ParseFloat(cols[0], 64)
	if err != nil {
		return time.Time{}, err
	}

	diff := -1 * time.Duration(seconds) * time.Second

	return now.Add(diff), nil
}

type procStatus struct {
	name      string
	pid       string
	parentPid string
	uid       string
	gid       string
}

func (p *procStatus) String() string {
	return fmt.Sprintf("name: %s, pid: %s, ppid: %s, uid: %s, gid: %s", p.name, p.pid, p.parentPid, p.uid, p.gid)
}

func readProcStatus(pid string) (*procStatus, error) {
	statusFile := filepath.Join(procfsRoot, pid, "status")
	f, err := os.Open(statusFile)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	return parseProcStatus(f)
}

func parseProcStatus(f io.Reader) (*procStatus, error) {
	scanner := bufio.NewScanner(f)
	status := procStatus{}
	for scanner.Scan() {
		cols := strings.Split(scanner.Text(), "\t")
		switch cols[0] {
		case "Name:":
			status.name = cols[1]
		case "Pid:":
			status.pid = cols[1]
		case "PPid:":
			status.parentPid = cols[1]
		case "Uid:":
			// Real, effective, saved set, and filesystem UIDs
			// TODO which one to use?
			status.uid = cols[1]
		case "Gid:":
			// Real, effective, saved set, and filesystem UIDs
			// TODO which one to use?
			status.gid = cols[1]
		default:
			//do nothing
		}
	}

	return &status, nil
}

func readProcStat(pid string, clktck int64) (cputime int64, startTime int64, err error) {
	statusFile := filepath.Join(procfsRoot, pid, "stat")
	f, err := os.Open(statusFile)
	if err != nil {
		return 0, 0, err
	}

	defer f.Close()

	return parseProcStat(f, clktck)
}

func parseProcStat(f io.Reader, clktck int64) (cputime int64, startTime int64, err error) {
	b, err := io.ReadAll(f)
	if err != nil {
		return 0, 0, err
	}

	content := string(b)
	cols := strings.Split(content, " ")
	// 14) utime  %lu Amount of time that this process has been scheduled
	// 			      in user mode, measured in clock ticks (divide by
	//                sysconf(_SC_CLK_TCK)).
	utime, err := strconv.ParseInt(cols[13], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("utime %w", err)
	}

	// (15) stime  %lu Amount of time that this process has been scheduled
	// 				   in kernel mode, measured in clock ticks (divide by
	// 				   sysconf(_SC_CLK_TCK)).

	stime, err := strconv.ParseInt(cols[14], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("stime %w", err)
	}

	cputime = (stime + utime) / clktck

	// (22) starttime  %llu The time the process started after system boot.
	//                 		Before Linux 2.6, this value was expressed in
	//                 		jiffies.  Since Linux 2.6, the value is expressed
	//                 		in clock ticks (divide by sysconf(_SC_CLK_TCK)).
	startTime, err = strconv.ParseInt(cols[21], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("starttime %w", err)
	}

	startTime /= clktck

	return cputime, startTime, nil
}

func formatCpuTime(cputime int64) string {
	hours := cputime / 3600
	if hours > 0 {
		cputime = cputime % 3600
	}

	minutes := cputime / 60
	seconds := cputime % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
