package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
)

const (
	PORT = 3333

	STREAM_BUFFER_SIZE = 1024
)

func parseBody[Body any](body io.ReadCloser) (Body, error) {
	var result Body
	if err := json.NewDecoder(body).Decode(&result); err != nil && err != io.EOF {
		return result, fmt.Errorf("error reading body: %w", err)
	}
	return result, nil
}

func runCommand(args []string, w http.ResponseWriter, contentType string) {
	cmd := exec.Command(args[0], args[1:]...)

	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			http.Error(w, fmt.Sprintf("error running command (%v): %v", cmd, err), http.StatusInternalServerError)
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(cmdOutput)
}

func runStreamCommand(args []string, w http.ResponseWriter) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "error creating flusher", http.StatusInternalServerError)
		return
	}

	cmd := exec.Command(args[0], args[1:]...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("error running command (%v): %v", cmd, err), http.StatusInternalServerError)
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("error running command (%v): %v", cmd, err), http.StatusInternalServerError)
		return
	}
	defer stdoutPipe.Close()
	defer stderrPipe.Close()
	flush := func(pipe io.ReadCloser) {
		buf := make([]byte, STREAM_BUFFER_SIZE)

		for {
			n, err := pipe.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, fmt.Sprintf("error running command (%v): %v", cmd, err), http.StatusInternalServerError)
				return
			}
			if n > 0 {
				w.Write(buf[:n])
				flusher.Flush()
			}
		}
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("error running command (%v): %v", cmd, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	go flush(stdoutPipe)
	go flush(stderrPipe)
	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			http.Error(w, fmt.Sprintf("error running command (%v): %v", cmd, err), http.StatusInternalServerError)
			return
		}
	}
}

func main() {
	http.Handle("GET /", http.FileServer(http.Dir("./static")))

	http.HandleFunc("POST /",
		func(w http.ResponseWriter, r *http.Request) {
			body, err := parseBody[struct {
				Args   []string
				Stream bool
				Json   bool
			}](r.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("error reading body: %v", err), http.StatusBadRequest)
				return
			}
			if len(body.Args) == 0 {
				http.Error(w, "bad request: args should not be empty", http.StatusBadRequest)
				return
			}
			slog.Info(r.URL.Path, "body", body)

			if body.Stream {
				runStreamCommand(body.Args, w)
			} else {
				var contentType string
				if body.Json {
					contentType = "application/json"
				} else {
					contentType = "text/plain; charset=UTF-8"
				}

				runCommand(body.Args, w, contentType)
			}
		},
	)

	slog.Info(fmt.Sprintf("listening at http://localhost:%d ...\n", PORT))
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
}
