/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// webserverCmd represents the webserver command
var webserverCmd = &cobra.Command{
	Use:   "webserver",
	Short: "Simple HTTP server for filesystem",
	Run: func(cmd *cobra.Command, args []string) {
		serve(args)
	},
}

var bind string
var directory string
var logfile string

func init() {
	rootCmd.AddCommand(webserverCmd)
	webserverCmd.Flags().StringVarP(&bind, "bind", "b", "", "Specify alternate bind address [default: all interfaces]")
	webserverCmd.Flags().StringVarP(&directory, "directory", "d", ".", "Specify alternative directory [default:current directory]")
	webserverCmd.Flags().StringVarP(&logfile, "logfile", "l", "", "Specify a log file, otherwise logs are suprassed")
}

func serve(args []string) {
	port := "8000"
	if len(args) == 1 {
		_, err := strconv.Atoi(args[0])
		if err != nil {
			// initial errors are printed to console to inform user parameters errors
			fmt.Fprintf(os.Stderr, "Invalid port number: %s %s", args[0], err)
			return
		}

		port = args[0]
	}

	addr := bind + ":" + port
	dir, err := filepath.Abs(directory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid directory: %s %s", directory, err)
		return
	}

	fmt.Printf("Serving HTTP on: %s for directory: %s\n", addr, dir)

	setDefaultLogger()

	d := newDirectoryServer(dir)
	http.Handle("/", d)
	err = http.ListenAndServe(addr, nil)

	if err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error %s\n", err)
	}
}

func setDefaultLogger() {
	logpath := logfile
	level := slog.LevelInfo
	if len(logfile) == 0 {
		logpath = os.DevNull
		level = slog.LevelError
	}
	f, err := os.OpenFile(logpath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open log file: %s %s\n", logfile, err)
		return
	}

	logger := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

}

const htmlTemplate = `
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
<title>Directory listing for /</title>
</head>
<body>
<h1>Directory listing for %s</h1>
<hr>
<ul>
%s
</ul>
<hr>
</body>
</html>
`

const hrefTemplate = `<li><a href="%s">%s</a></li>`

type directoryServer struct {
	directory string
	fs        afero.Fs
}

type loggingResponseWriter struct {
	http.ResponseWriter
	r       *http.Request
	reqTime time.Time
}

func (l *loggingResponseWriter) WriteHeader(statusCode int) {
	l.ResponseWriter.WriteHeader(statusCode)

	addr := l.r.RemoteAddr
	cols := strings.Split(addr, ":")
	if len(cols) == 2 {
		addr = cols[0] // remove port
	}
	// 127.0.0.1 - - [27/12/2023 10:08:46] "GET /cmd/ HTTP/1.1" 200 -
	fmt.Printf("%s - - [%s] \"%s %s %s\" %d -\n", addr, l.reqTime.Format("02/01/2006 15:04:05"), l.r.Method, l.r.URL.Path, l.r.Proto, statusCode)
}

func newDirectoryServer(dir string) *directoryServer {
	return newDirectoryServerWithFs(dir, afero.NewOsFs())
}

func newDirectoryServerWithFs(dir string, fs afero.Fs) *directoryServer {
	// append file separator to make directory escape prevention logic simpler
	dir = dir + string(filepath.Separator)
	return &directoryServer{directory: dir, fs: fs}
}

func (d *directoryServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w = &loggingResponseWriter{w, r, time.Now()}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	path := filepath.Join(d.directory, r.URL.Path)
	path = filepath.Clean(path)

	if r.URL.Path != "/" && !strings.HasPrefix(path, d.directory) {
		slog.Warn("Requested path is outside of the served directory", "path", path)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	info, err := d.fs.Stat(path)
	if err != nil {
		slog.Warn("Cannot stat path", "path", path, "error", err)
		if errors.Is(err, os.ErrNotExist) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		body := fmt.Sprintf("Error: %s", err)
		w.Write([]byte(body))
		return
	}

	if info.IsDir() {
		d.serveDirectory(w, r, path)
		return
	}

	d.serveFile(w, r, path, info.Size())
}

func (d *directoryServer) serveDirectory(w http.ResponseWriter, r *http.Request, path string) {
	entries, err := afero.ReadDir(d.fs, path)
	if err != nil {
		slog.Warn("Cannot list directory", "path", path, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		body := fmt.Sprintf("Error: %s", err)
		w.Write([]byte(body))
		return
	}

	links := make([]string, 0, len(entries))
	for _, d := range entries {
		name := d.Name()
		if d.IsDir() {
			name += "/"
		}
		l := fmt.Sprintf(hrefTemplate, name, name)
		links = append(links, l)
	}

	body := fmt.Sprintf(htmlTemplate, r.URL.Path, strings.Join(links, "\n"))
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(body))
}

func (d *directoryServer) serveFile(w http.ResponseWriter, r *http.Request, path string, fileSize int64) {
	f, err := d.fs.Open(path)
	if err != nil {
		slog.Warn("Cannot open file", "path", path, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		body := fmt.Sprintf("Error: %s", err)
		w.Write([]byte(body))
		return
	}
	defer f.Close()

	w.Header().Add("Content-Type", "application/octet-stream")
	w.Header().Add("Content-Length", strconv.FormatInt(fileSize, 10))
	w.WriteHeader(http.StatusOK)

	buffer := make([]byte, 512)
	for {
		n, err := f.Read(buffer)
		if err == io.EOF {
			break
		}
		w.Write(buffer[:n])
	}
}
