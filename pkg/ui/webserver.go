package ui

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eulerguard/pkg/config"
)

func RunWebServer(opts config.Options, port int, assets embed.FS) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("must run as root (current euid=%d)", os.Geteuid())
	}

	app := NewApp(opts)
	close(app.ready)

	go func() {
		if err := app.Run(); err != nil {
			log.Printf("Tracer error: %v", err)
		}
	}()

	mux := http.NewServeMux()
	registerAPI(mux, app)
	registerStatic(mux, assets)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		server.Shutdown(context.Background())
	}()

	log.Printf("========================================")
	log.Printf("  EulerGuard Web UI: http://localhost:%d", port)
	log.Printf("========================================")

	return server.ListenAndServe()
}

func registerStatic(mux *http.ServeMux, assets embed.FS) {
	frontendFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Printf("Warning: frontend assets not found: %v", err)
		return
	}
	mux.Handle("/", http.FileServer(http.FS(frontendFS)))
}

func registerAPI(mux *http.ServeMux, app *App) {
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		s := app.GetSystemStats()
		fmt.Fprintf(w, `{"processCount":%d,"containerCount":%d,"eventsPerSec":%.2f,"alertCount":%d,"probeStatus":"%s"}`,
			s.ProcessCount, s.ContainerCount, s.EventsPerSec, s.AlertCount, s.ProbeStatus)
	})

	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		alerts := app.GetAlerts()
		if len(alerts) == 0 {
			w.Write([]byte("[]"))
			return
		}
		w.Write([]byte("["))
		for i, a := range alerts {
			if i > 0 {
				w.Write([]byte(","))
			}
			fmt.Fprintf(w, `{"id":"%s","timestamp":%d,"severity":"%s","ruleName":"%s","description":"%s","pid":%d,"processName":"%s","inContainer":%t}`,
				a.ID, a.Timestamp, a.Severity, a.RuleName, a.Description, a.PID, a.ProcessName, a.InContainer)
		}
		w.Write([]byte("]"))
	})

	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				exec, file, net := app.Stats().Rates()
				fmt.Fprintf(w, "data: {\"exec\":%d,\"file\":%d,\"network\":%d}\n\n", exec, file, net)
				flusher.Flush()
			}
		}
	})
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}
