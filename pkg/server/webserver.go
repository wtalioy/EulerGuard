package server

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"aegis/pkg/ai"
	"aegis/pkg/api"
	"aegis/pkg/config"
	"aegis/pkg/events"
	"aegis/pkg/storage"
	"aegis/pkg/types"
	"aegis/pkg/utils"
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
		ai.StopOllamaRuntime()
		server.Shutdown(context.Background())
	}()

	log.Printf("========================================")
	log.Printf("Aegis Web UI: http://localhost:%d", port)
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
		fmt.Fprintf(w, `{"processCount":%d,"workloadCount":%d,"eventsPerSec":%.2f,"alertCount":%d,"probeStatus":"%s"}`,
			s.ProcessCount, s.WorkloadCount, s.EventsPerSec, s.AlertCount, s.ProbeStatus)
	})

	// Phase 1: Get rate statistics
	mux.HandleFunc("/api/stats/rates", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		execRate, fileRate, connectRate := app.Stats().Rates()
		json.NewEncoder(w).Encode(map[string]float64{
			"execRate":    float64(execRate),
			"fileRate":    float64(fileRate),
			"connectRate": float64(connectRate),
		})
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
			fmt.Fprintf(w, `{"id":"%s","timestamp":%d,"severity":"%s","ruleName":"%s","description":"%s","pid":%d,"processName":"%s","cgroupId":"%s","action":"%s","blocked":%t}`,
				a.ID, a.Timestamp, a.Severity, a.RuleName, a.Description, a.PID, a.ProcessName, a.CgroupID, a.Action, a.Blocked)
		}
		w.Write([]byte("]"))
	})

	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)

		if r.Method == "GET" && r.URL.Query().Get("stream") == "true" {
			// SSE stream (existing behavior)
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
		}

		// GET /api/events - List events with pagination
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")

			// Parse query parameters
			limit := 50
			if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
				if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
					limit = l
				}
			}

			eventType := r.URL.Query().Get("type")
			processName := r.URL.Query().Get("process")

			if app.core == nil || app.core.Storage == nil {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"events": []interface{}{},
					"total":  0,
					"page":   1,
				})
				return
			}

			// Get latest events
			eventList, err := app.core.Storage.Latest(limit)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Filter by type and process if specified
			filtered := make([]*storage.Event, 0)
			for _, ev := range eventList {
				if eventType != "" && string(ev.Type) != eventType {
					continue
				}
				if processName != "" {
					// Check process name in event data
					matched := false
					switch ev.Type {
					case events.EventTypeExec:
						if execEv, ok := ev.Data.(*events.ExecEvent); ok {
							if utils.ExtractCString(execEv.Hdr.Comm[:]) == processName {
								matched = true
							}
						}
					case events.EventTypeFileOpen:
						if fileEv, ok := ev.Data.(*events.FileOpenEvent); ok {
							if utils.ExtractCString(fileEv.Hdr.Comm[:]) == processName {
								matched = true
							}
						}
					case events.EventTypeConnect:
						if connEv, ok := ev.Data.(*events.ConnectEvent); ok {
							if utils.ExtractCString(connEv.Hdr.Comm[:]) == processName {
								matched = true
							}
						}
					}
					if !matched {
						continue
					}
				}
				filtered = append(filtered, ev)
			}

			// Convert to frontend format
			frontendEvents := make([]interface{}, 0, len(filtered))
			for _, ev := range filtered {
				switch ev.Type {
				case events.EventTypeExec:
					if execEv, ok := ev.Data.(*events.ExecEvent); ok {
						frontendEvents = append(frontendEvents, ExecToFrontend(*execEv))
					}
				case events.EventTypeFileOpen:
					if fileEv, ok := ev.Data.(*events.FileOpenEvent); ok {
						filename := utils.ExtractCString(fileEv.Filename[:])
						frontendEvents = append(frontendEvents, FileToFrontend(*fileEv, filename))
					}
				case events.EventTypeConnect:
					if connEv, ok := ev.Data.(*events.ConnectEvent); ok {
						ip := utils.ExtractIP(connEv)
						addr := fmt.Sprintf("%s:%d", ip, connEv.Port)
						processName := utils.ExtractCString(connEv.Hdr.Comm[:])
						frontendEvents = append(frontendEvents, ConnectToFrontend(*connEv, addr, processName))
					}
				}
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"events": frontendEvents,
				"total":  len(frontendEvents),
				"page":   1,
			})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// Phase 1: Get event by ID (using index)
	mux.HandleFunc("/api/events/", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/events/"), "/")
		if len(pathParts) == 0 {
			http.Error(w, "Invalid event ID", http.StatusBadRequest)
			return
		}

		if pathParts[0] == "range" {
			// GET /api/events/range - Time range query
			startStr := r.URL.Query().Get("start")
			endStr := r.URL.Query().Get("end")

			if startStr == "" || endStr == "" {
				http.Error(w, "start and end parameters required", http.StatusBadRequest)
				return
			}

			start, err := time.Parse(time.RFC3339, startStr)
			if err != nil {
				http.Error(w, "Invalid start time format (use RFC3339)", http.StatusBadRequest)
				return
			}

			end, err := time.Parse(time.RFC3339, endStr)
			if err != nil {
				http.Error(w, "Invalid end time format (use RFC3339)", http.StatusBadRequest)
				return
			}

			if app.core == nil || app.core.Storage == nil {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"events": []interface{}{},
				})
				return
			}

			eventList, err := app.core.Storage.Query(start, end)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Convert to frontend format
			frontendEvents := make([]interface{}, 0, len(eventList))
			for _, ev := range eventList {
				switch ev.Type {
				case events.EventTypeExec:
					if execEv, ok := ev.Data.(*events.ExecEvent); ok {
						frontendEvents = append(frontendEvents, ExecToFrontend(*execEv))
					}
				case events.EventTypeFileOpen:
					if fileEv, ok := ev.Data.(*events.FileOpenEvent); ok {
						filename := utils.ExtractCString(fileEv.Filename[:])
						frontendEvents = append(frontendEvents, FileToFrontend(*fileEv, filename))
					}
				case events.EventTypeConnect:
					if connEv, ok := ev.Data.(*events.ConnectEvent); ok {
						ip := utils.ExtractIP(connEv)
						addr := fmt.Sprintf("%s:%d", ip, connEv.Port)
						processName := utils.ExtractCString(connEv.Hdr.Comm[:])
						frontendEvents = append(frontendEvents, ConnectToFrontend(*connEv, addr, processName))
					}
				}
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"events": frontendEvents,
			})
			return
		}

		// GET /api/events/{id} - Get event by ID
		eventID := pathParts[0]

		if app.core == nil || app.core.Storage == nil {
			http.Error(w, "Storage not available", http.StatusServiceUnavailable)
			return
		}

		// Search through recent events to find matching ID
		// Event ID is generated as hash of timestamp + event data
		eventList, err := app.core.Storage.Latest(10000) // Search last 10k events
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var foundEvent *storage.Event
		for _, ev := range eventList {
			// Generate ID for this event
			id := generateEventID(ev)
			if id == eventID {
				foundEvent = ev
				break
			}
		}

		if foundEvent == nil {
			http.Error(w, "Event not found", http.StatusNotFound)
			return
		}

		// Convert to frontend format
		var frontendEvent interface{}
		switch foundEvent.Type {
		case events.EventTypeExec:
			if execEv, ok := foundEvent.Data.(*events.ExecEvent); ok {
				frontendEvent = ExecToFrontend(*execEv)
			}
		case events.EventTypeFileOpen:
			if fileEv, ok := foundEvent.Data.(*events.FileOpenEvent); ok {
				filename := utils.ExtractCString(fileEv.Filename[:])
				frontendEvent = FileToFrontend(*fileEv, filename)
			}
		case events.EventTypeConnect:
			if connEv, ok := foundEvent.Data.(*events.ConnectEvent); ok {
				ip := utils.ExtractIP(connEv)
				addr := fmt.Sprintf("%s:%d", ip, connEv.Port)
				processName := utils.ExtractCString(connEv.Hdr.Comm[:])
				frontendEvent = ConnectToFrontend(*connEv, addr, processName)
			}
		}

		if frontendEvent == nil {
			http.Error(w, "Event format not supported", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(frontendEvent)
	})

	// Phase 1: Process profile endpoints
	mux.HandleFunc("/api/process/", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract PID from path: /api/process/{pid}/...
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/process/"), "/")
		if len(pathParts) == 0 {
			http.Error(w, "Invalid PID", http.StatusBadRequest)
			return
		}

		pid, err := strconv.ParseUint(pathParts[0], 10, 32)
		if err != nil {
			http.Error(w, "Invalid PID", http.StatusBadRequest)
			return
		}

		if app.core == nil || app.core.ProfileReg == nil {
			http.Error(w, "Profile registry not available", http.StatusServiceUnavailable)
			return
		}

		// Handle sub-paths
		if len(pathParts) > 1 {
			subPath := pathParts[1]

			if subPath == "profile" {
				// GET /api/process/{pid}/profile
				profile, ok := app.core.ProfileReg.GetProfile(uint32(pid))
				if !ok {
					http.Error(w, "Process profile not found", http.StatusNotFound)
					return
				}

				// Get ancestors for genealogy
				ancestors := app.GetAncestors(uint32(pid))
				genealogy := make([]uint32, 0, len(ancestors))
				for _, anc := range ancestors {
					genealogy = append(genealogy, anc.PID)
				}

				json.NewEncoder(w).Encode(map[string]interface{}{
					"pid": uint32(pid),
					"static": map[string]interface{}{
						"startTime":   profile.Static.StartTime.UnixMilli(),
						"commandLine": profile.Static.CommandLine,
						"genealogy":   genealogy,
					},
					"dynamic": map[string]interface{}{
						"fileOpenCount":   profile.Dynamic.FileOpenCount,
						"netConnectCount": profile.Dynamic.NetConnectCount,
						"execCount":       profile.Dynamic.ExecCount,
						"lastFileOpen":    profile.Dynamic.LastFileOpen.UnixMilli(),
						"lastConnect":     profile.Dynamic.LastConnect.UnixMilli(),
						"lastExec":        profile.Dynamic.LastExec.UnixMilli(),
					},
					"baseline": profile.Baseline,
				})
				return
			}

			if subPath == "tree" {
				// GET /api/process/{pid}/tree
				ancestors := app.GetAncestors(uint32(pid))
				ancestorPIDs := make([]uint32, 0, len(ancestors))
				for _, anc := range ancestors {
					ancestorPIDs = append(ancestorPIDs, anc.PID)
				}

				// Get children (would need ProcessTree.GetChildren method)
				children := []uint32{} // Placeholder

				json.NewEncoder(w).Encode(map[string]interface{}{
					"ancestors": ancestorPIDs,
					"children":  children,
				})
				return
			}

			if subPath == "events" {
				// GET /api/process/{pid}/events
				limit := 100
				if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
					if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
						limit = l
					}
				}

				if app.core == nil || app.core.Storage == nil {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"events": []interface{}{},
					})
					return
				}

				// Query events by PID
				eventList := app.core.Storage.QueryByPID(uint32(pid))
				if len(eventList) > limit {
					eventList = eventList[len(eventList)-limit:]
				}

				// Convert to frontend format
				frontendEvents := make([]interface{}, 0, len(eventList))
				for _, ev := range eventList {
					switch ev.Type {
					case events.EventTypeExec:
						if execEv, ok := ev.Data.(*events.ExecEvent); ok {
							frontendEvents = append(frontendEvents, ExecToFrontend(*execEv))
						}
					case events.EventTypeFileOpen:
						if fileEv, ok := ev.Data.(*events.FileOpenEvent); ok {
							filename := utils.ExtractCString(fileEv.Filename[:])
							frontendEvents = append(frontendEvents, FileToFrontend(*fileEv, filename))
						}
					case events.EventTypeConnect:
						if connEv, ok := ev.Data.(*events.ConnectEvent); ok {
							ip := utils.ExtractIP(connEv)
							addr := fmt.Sprintf("%s:%d", ip, connEv.Port)
							processName := utils.ExtractCString(connEv.Hdr.Comm[:])
							frontendEvents = append(frontendEvents, ConnectToFrontend(*connEv, addr, processName))
						}
					}
				}

				json.NewEncoder(w).Encode(map[string]interface{}{
					"events": frontendEvents,
				})
				return
			}
		}

		http.Error(w, "Invalid endpoint", http.StatusBadRequest)
	})

	mux.HandleFunc("/api/rules", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		if r.Method == "GET" {
			rules := app.GetRules()
			data, _ := json.Marshal(rules)
			w.Write(data)
			return
		}

		if r.Method == "POST" {
			// Create rule
			var req struct {
				Rule types.Rule `json:"rule"`
				Mode string     `json:"mode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			// Set state if provided (map legacy "mode" to "state")
			if req.Mode != "" {
				switch req.Mode {
				case "draft":
					req.Rule.State = types.RuleStateDraft
				case "testing":
					req.Rule.State = types.RuleStateTesting
				case "production":
					req.Rule.State = types.RuleStateProduction
				}
			} else if req.Rule.State == "" {
				// Default to testing if no state is set
				req.Rule.State = types.RuleStateTesting
			}

			// Load existing rules and append new rule
			allRules := app.getRulesInternal()

			// Set timestamps for new rule
			now := time.Now()
			if req.Rule.CreatedAt.IsZero() {
				req.Rule.CreatedAt = now
			}
			// Set DeployedAt if deploying to testing or production
			if req.Rule.State == types.RuleStateTesting || req.Rule.State == types.RuleStateProduction {
				if req.Rule.DeployedAt == nil {
					req.Rule.DeployedAt = &now
				}
			}

			allRules = append(allRules, req.Rule)

			// Save and reload
			if err := app.saveAndReloadRules(allRules); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"rule":    req.Rule,
			})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// Phase 2: Update rule
	mux.HandleFunc("/api/rules/", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "PUT, DELETE, POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		// Extract rule name from path
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/rules/"), "/")
		if len(pathParts) == 0 {
			http.Error(w, "Invalid rule name", http.StatusBadRequest)
			return
		}
		ruleName := pathParts[0]

		// Note: promote and demote are handled via /api/rules/validation/{name}/promote and /api/rules/validation/{name}/demote

		// Update rule
		if r.Method == "PUT" {
			var req struct {
				Rule types.Rule `json:"rule"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			// Get rules and update
			rule, allRules := app.findRuleByName(ruleName)
			if rule == nil {
				http.Error(w, "Rule not found", http.StatusNotFound)
				return
			}

			// Preserve existing metadata fields
			existingCreatedAt := rule.CreatedAt
			existingDeployedAt := rule.DeployedAt
			existingPromotedAt := rule.PromotedAt
			existingLastReviewedAt := rule.LastReviewedAt
			existingReviewNotes := rule.ReviewNotes

			// Preserve the rule name (must match URL parameter)
			// Update the rule fields
			// Note: Name is preserved from URL parameter, not from request body
			rule.Description = req.Rule.Description
			rule.Action = req.Rule.Action
			rule.Severity = req.Rule.Severity
			rule.Match = req.Rule.Match
			rule.Type = req.Rule.Type
			rule.State = req.Rule.State

			// Preserve metadata if not provided
			if existingCreatedAt.IsZero() && !req.Rule.CreatedAt.IsZero() {
				rule.CreatedAt = req.Rule.CreatedAt
			} else if !existingCreatedAt.IsZero() {
				rule.CreatedAt = existingCreatedAt
			}

			if existingDeployedAt != nil {
				rule.DeployedAt = existingDeployedAt
			} else if req.Rule.DeployedAt != nil {
				rule.DeployedAt = req.Rule.DeployedAt
			}

			if existingPromotedAt != nil {
				rule.PromotedAt = existingPromotedAt
			} else if req.Rule.PromotedAt != nil {
				rule.PromotedAt = req.Rule.PromotedAt
			}

			if existingLastReviewedAt != nil {
				rule.LastReviewedAt = existingLastReviewedAt
			} else if req.Rule.LastReviewedAt != nil {
				rule.LastReviewedAt = req.Rule.LastReviewedAt
			}

			if existingReviewNotes != "" {
				rule.ReviewNotes = existingReviewNotes
			} else {
				rule.ReviewNotes = req.Rule.ReviewNotes
			}

			// Update DeployedAt if state changed to testing or production
			if (rule.State == types.RuleStateTesting || rule.State == types.RuleStateProduction) && rule.DeployedAt == nil {
				now := time.Now()
				rule.DeployedAt = &now
			}

			if err := app.saveAndReloadRules(allRules); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"rule":    *rule,
			})
			return
		}

		// Delete rule
		if r.Method == "DELETE" {
			// Get rules and delete
			rule, allRules := app.findRuleByName(ruleName)
			if rule == nil {
				http.Error(w, "Rule not found", http.StatusNotFound)
				return
			}
			// Remove rule from slice
			for i, r := range allRules {
				if r.Name == ruleName {
					allRules = append(allRules[:i], allRules[i+1:]...)
					break
				}
			}

			if err := app.saveAndReloadRules(allRules); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/workloads", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if strings.HasPrefix(r.URL.Path, "/api/workloads/") {
			idStr := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
			workload := app.GetWorkload(idStr)
			if workload == nil {
				http.Error(w, "Workload not found", http.StatusNotFound)
				return
			}
			data, _ := json.Marshal(workload)
			w.Write(data)
			return
		}

		workloads := app.GetWorkloads()
		data, _ := json.Marshal(workloads)
		w.Write(data)
	})

	mux.HandleFunc("/api/workloads/", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		idStr := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
		workload := app.GetWorkload(idStr)
		if workload == nil {
			http.Error(w, "Workload not found", http.StatusNotFound)
			return
		}
		data, _ := json.Marshal(workload)
		w.Write(data)
	})

	mux.HandleFunc("/api/probes/stats", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		stats := app.GetProbeStats()
		data, _ := json.Marshal(stats)
		w.Write(data)
	})

	mux.HandleFunc("/api/learning/status", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		status := app.GetLearningStatus()
		data, _ := json.Marshal(status)
		w.Write(data)
	})

	mux.HandleFunc("/api/learning/start", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Duration int `json:"duration"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := app.StartLearning(req.Duration); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/learning/stop", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rules, err := app.StopLearning()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(rules)
		w.Write(data)
	})

	mux.HandleFunc("/api/learning/apply", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Indices []int `json:"indices"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := app.ApplyWhitelistRules(req.Indices); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/ai/status", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		var status ai.StatusDTO
		if app.AIService() != nil {
			status = app.AIService().GetStatus()
		} else {
			status = ai.StatusDTO{Status: "unavailable"}
		}

		json.NewEncoder(w).Encode(status)
	})

	mux.HandleFunc("/api/ai/diagnose", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()

		result, err := app.Diagnose(ctx)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if result == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": "AI service not available"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("/api/ai/chat", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Message   string `json:"message"`
			SessionID string `json:"sessionId"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Message == "" {
			http.Error(w, "Message is required", http.StatusBadRequest)
			return
		}

		if req.SessionID == "" {
			req.SessionID = generateSessionID()
		}

		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()

		result, err := app.Chat(ctx, req.SessionID, req.Message)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if result == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": "AI service not available"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("/api/ai/chat/stream", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Message   string `json:"message"`
			SessionID string `json:"sessionId"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Message == "" {
			http.Error(w, "Message is required", http.StatusBadRequest)
			return
		}

		if req.SessionID == "" {
			req.SessionID = generateSessionID()
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()

		tokenChan, err := app.ChatStream(ctx, req.SessionID, req.Message)
		if err != nil {
			data, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return
		}

		if tokenChan == nil {
			data, _ := json.Marshal(map[string]string{"error": "AI service not available"})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return
		}

		for token := range tokenChan {
			data, err := json.Marshal(token)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	})

	mux.HandleFunc("/api/ai/chat/history", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		sessionID := r.URL.Query().Get("sessionId")
		if sessionID == "" {
			json.NewEncoder(w).Encode([]ai.Message{})
			return
		}

		history := app.GetChatHistory(sessionID)
		if history == nil {
			history = []ai.Message{}
		}

		json.NewEncoder(w).Encode(history)
	})

	// Phase 3: AI Intent Parsing
	mux.HandleFunc("/api/ai/intent", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Input   string             `json:"input"`
			Context *ai.RequestContext `json:"context"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Context == nil {
			req.Context = &ai.RequestContext{}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		intent, err := app.AIService().ParseIntent(ctx, req.Input, req.Context)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(intent)
	})

	// Phase 3: AI Rule Generation
	mux.HandleFunc("/api/ai/generate-rule", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ai.RuleGenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()

		response, err := app.AIService().GenerateRule(ctx, &req, app.core.RuleEngine, app.core.Storage)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Phase 3: AI Event Explanation
	mux.HandleFunc("/api/ai/explain", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Accept both camelCase and snake_case payloads
		var raw map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		var req ai.ExplainRequest
		if v, ok := raw["eventId"].(string); ok {
			req.EventID = v
		} else if v, ok := raw["event_id"].(string); ok {
			req.EventID = v
		}
		if v, ok := raw["question"].(string); ok {
			req.Question = v
		}
		if v, ok := raw["eventData"]; ok {
			req.EventData = v
		} else if v, ok := raw["event_data"]; ok {
			req.EventData = v
		}

		// Build storage.Event from frontend shape
		var event *storage.Event
		if req.EventData != nil {
			event = &storage.Event{Timestamp: time.Now(), Data: req.EventData}
			// Try to infer type from eventData.type
			if m, ok := req.EventData.(map[string]interface{}); ok {
				// Timestamp may be at top-level or under header.timestamp (ms)
				if ts, ok := m["timestamp"].(float64); ok {
					event.Timestamp = time.UnixMilli(int64(ts))
				} else if hdr, ok := m["header"].(map[string]interface{}); ok {
					if ts, ok := hdr["timestamp"].(float64); ok {
						event.Timestamp = time.UnixMilli(int64(ts))
					}
				}
				// Type is normalized as lowercase string
				if t, ok := m["type"].(string); ok {
					switch t {
					case "exec":
						event.Type = events.EventTypeExec
					case "file":
						event.Type = events.EventTypeFileOpen
					case "connect":
						event.Type = events.EventTypeConnect
					}
				}
			}
		}

		if event == nil {
			http.Error(w, "Event not found", http.StatusNotFound)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		response, err := app.AIService().ExplainEvent(ctx, &req, event, app.core.RuleEngine, app.core.Storage, app.core.ProfileReg)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Phase 3: AI Context Analysis
	mux.HandleFunc("/api/ai/analyze", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ai.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		response, err := app.AIService().Analyze(ctx, &req, app.core.ProfileReg, app.core.WorkloadReg, app.core.RuleEngine)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Phase 3: Semantic Query
	mux.HandleFunc("/api/query", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req api.QueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		queryService := api.NewSemanticQueryService(app.AIService(), app.core.Storage)
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		response, err := queryService.Query(ctx, &req)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Convert storage events to frontend events
		frontendEvents := make([]interface{}, 0, len(response.Events))
		for _, ev := range response.Events {
			if ev == nil {
				continue
			}
			switch ev.Type {
			case events.EventTypeExec:
				// Handle both pointer and value types
				var execEv *events.ExecEvent
				if ptr, ok := ev.Data.(*events.ExecEvent); ok {
					execEv = ptr
				} else if val, ok := ev.Data.(events.ExecEvent); ok {
					execEv = &val
				}
				if execEv != nil {
					frontendEvents = append(frontendEvents, ExecToFrontend(*execEv))
				}
			case events.EventTypeFileOpen:
				// Handle both pointer and value types
				var fileEv *events.FileOpenEvent
				if ptr, ok := ev.Data.(*events.FileOpenEvent); ok {
					fileEv = ptr
				} else if val, ok := ev.Data.(events.FileOpenEvent); ok {
					fileEv = &val
				}
				if fileEv != nil {
					filename := utils.ExtractCString(fileEv.Filename[:])
					frontendEvents = append(frontendEvents, FileToFrontend(*fileEv, filename))
				}
			case events.EventTypeConnect:
				// Handle both pointer and value types
				var connEv *events.ConnectEvent
				if ptr, ok := ev.Data.(*events.ConnectEvent); ok {
					connEv = ptr
				} else if val, ok := ev.Data.(events.ConnectEvent); ok {
					connEv = &val
				}
				if connEv != nil {
					ip := utils.ExtractIP(connEv)
					addr := fmt.Sprintf("%s:%d", ip, connEv.Port)
					processName := utils.ExtractCString(connEv.Hdr.Comm[:])
					frontendEvents = append(frontendEvents, ConnectToFrontend(*connEv, addr, processName))
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"events":      frontendEvents,
			"total":       response.Total,
			"page":        response.Page,
			"limit":       response.Limit,
			"total_pages": response.TotalPages,
			"type_counts": response.TypeCounts,
		})
	})

	// Phase 3: Sentinel WebSocket Stream
	mux.HandleFunc("/api/ai/sentinel/stream", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		// Subscribe to Sentinel insights
		sentinel := app.Sentinel()
		if sentinel == nil {
			// Sentinel not available, send heartbeat only
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-r.Context().Done():
					return
				case <-ticker.C:
					fmt.Fprintf(w, "data: {\"type\":\"heartbeat\"}\n\n")
					flusher.Flush()
				}
			}
		}

		// Subscribe to insights
		insightCh := sentinel.Subscribe()
		defer func() {
			// Close the channel and let Sentinel handle cleanup
			// Note: We can't directly unsubscribe with receive-only channel
			// Sentinel will handle cleanup when channel is closed
		}()

		// Send existing insights first
		existingInsights := sentinel.GetInsights(10)
		for _, insight := range existingInsights {
			data, _ := json.Marshal(insight)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		// Send heartbeat every 30 seconds
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case insight := <-insightCh:
				// New insight received
				data, _ := json.Marshal(insight)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-ticker.C:
				// Send heartbeat
				fmt.Fprintf(w, "data: {\"type\":\"heartbeat\"}\n\n")
				flusher.Flush()
			}
		}
	})

	// Phase 3: Get Sentinel Insights
	mux.HandleFunc("/api/ai/sentinel/insights", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		limit := 50
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		// Get insights from Sentinel
		sentinel := app.Sentinel()
		var insights []*ai.Insight
		if sentinel != nil {
			insights = sentinel.GetInsights(limit)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"insights": insights,
			"total":    len(insights),
		})
	})

	mux.HandleFunc("/api/ai/chat/clear", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.SessionID != "" {
			app.ClearChat(req.SessionID)
		}

		w.WriteHeader(http.StatusOK)
	})

	// Phase 3: AI Review endpoint
	mux.HandleFunc("/api/ai/review", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Rule types.Rule `json:"rule"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if app.AIService() == nil || !app.AIService().IsEnabled() {
			http.Error(w, "AI service not available", http.StatusServiceUnavailable)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		// Use Analyze endpoint to review the rule
		analyzeReq := ai.AnalyzeRequest{
			Type: "rule",
			ID:   req.Rule.Name,
		}

		response, err := app.AIService().Analyze(ctx, &analyzeReq, app.core.ProfileReg, app.core.WorkloadReg, app.core.RuleEngine)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(response)
	})

	// Phase 3: Sentinel Action endpoint
	mux.HandleFunc("/api/ai/sentinel/action", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			InsightID string                 `json:"insight_id"`
			ActionID  string                 `json:"action_id"`
			Params    map[string]interface{} `json:"params"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Execute action based on action_id
		switch req.ActionID {
		case "promote":
			// Promote testing rule
			ruleName, ok := req.Params["rule_name"].(string)
			if !ok {
				http.Error(w, "rule_name parameter required for promote action", http.StatusBadRequest)
				return
			}

			if err := app.PromoteRule(ruleName); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("Rule %s promoted successfully", ruleName),
			})

		case "dismiss":
			// Dismiss insight (no-op for now, just acknowledge)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Insight dismissed",
			})

		case "investigate":
			// Investigate event (could trigger deeper analysis)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Investigation started",
			})

		default:
			http.Error(w, fmt.Sprintf("Unknown action: %s", req.ActionID), http.StatusBadRequest)
			return
		}
	})

	// Settings API
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "GET" {
			// Return current settings
			opts := app.Options()
			settings := map[string]interface{}{
				"ai": map[string]interface{}{
					"mode": opts.AI.Mode,
					"ollama": map[string]interface{}{
						"endpoint": opts.AI.Ollama.Endpoint,
						"model":    opts.AI.Ollama.Model,
						"timeout":  opts.AI.Ollama.Timeout,
					},
					"openai": map[string]interface{}{
						"endpoint": opts.AI.OpenAI.Endpoint,
						"apiKey":   opts.AI.OpenAI.APIKey,
						"model":    opts.AI.OpenAI.Model,
						"timeout":  opts.AI.OpenAI.Timeout,
					},
				},
				"testing": map[string]interface{}{},
				"promotion": map[string]interface{}{
					"minObservationMinutes": opts.PromotionMinObservationMinutes,
					"minHits":               opts.PromotionMinHits,
				},
			}
			json.NewEncoder(w).Encode(settings)
			return
		}

		if r.Method == "PUT" || r.Method == "POST" {
			// Update settings
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			// Update app.opts (in-memory)
			opts := app.Options()
			if aiRaw, ok := req["ai"].(map[string]interface{}); ok {
				if v, ok := aiRaw["mode"].(string); ok && (v == "ollama" || v == "openai") {
					opts.AI.Mode = v
				}
				if ollamaRaw, ok := aiRaw["ollama"].(map[string]interface{}); ok {
					if v, ok := ollamaRaw["endpoint"].(string); ok {
						opts.AI.Ollama.Endpoint = v
					}
					if v, ok := ollamaRaw["model"].(string); ok {
						opts.AI.Ollama.Model = v
					}
					if v, ok := ollamaRaw["timeout"].(float64); ok {
						opts.AI.Ollama.Timeout = int(v)
					}
				}
				if openaiRaw, ok := aiRaw["openai"].(map[string]interface{}); ok {
					if v, ok := openaiRaw["endpoint"].(string); ok {
						opts.AI.OpenAI.Endpoint = v
					}
					if v, ok := openaiRaw["apiKey"].(string); ok {
						opts.AI.OpenAI.APIKey = v
					}
					if v, ok := openaiRaw["model"].(string); ok {
						opts.AI.OpenAI.Model = v
					}
					if v, ok := openaiRaw["timeout"].(float64); ok {
						opts.AI.OpenAI.Timeout = int(v)
					}
				}
			}

			// TODO: Save to config.yaml file (requires file write permissions)
			// For now, just update in-memory settings
			// Note: Changes will be lost on restart unless saved to file

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// NEW: Validation endpoints
	mux.HandleFunc("/api/rules/validation/", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		// Extract rule name from path
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/rules/validation/"), "/")
		if len(pathParts) == 0 {
			http.Error(w, "Invalid rule name", http.StatusBadRequest)
			return
		}

		// Handle sub-paths
		if len(pathParts) > 1 {
			subPath := pathParts[1]

			// POST /api/rules/validation/{name}/promote
			if subPath == "promote" && r.Method == "POST" {
				app.PromoteRuleHandler(w, r)
				return
			}

			// POST /api/rules/validation/{name}/demote
			if subPath == "demote" && r.Method == "POST" {
				app.DemoteRule(w, r)
				return
			}

			// Invalid sub-path
			http.Error(w, "Invalid endpoint", http.StatusBadRequest)
			return
		}

		// GET /api/rules/validation/{name}
		if r.Method == "GET" {
			app.GetRuleValidationHandler(w, r)
			return
		}

		http.Error(w, "Invalid endpoint", http.StatusBadRequest)
	})

	// GET /api/rules/testing
	mux.HandleFunc("/api/rules/testing", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			return
		}

		if r.Method == "GET" {
			app.GetTestingRules(w, r)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})
}

func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

// generateEventID generates a unique ID for an event based on its content and timestamp
func generateEventID(event *storage.Event) string {
	// Create a hash from timestamp and event data
	h := sha256.New()
	h.Write([]byte(event.Timestamp.Format(time.RFC3339Nano)))

	// Add event type
	fmt.Fprintf(h, "%d", int(event.Type))

	// Add event-specific data
	switch ev := event.Data.(type) {
	case *events.ExecEvent:
		h.Write(ev.Hdr.Comm[:])
		fmt.Fprintf(h, "%d", ev.Hdr.PID)
	case *events.FileOpenEvent:
		h.Write(ev.Filename[:])
		fmt.Fprintf(h, "%d", ev.Hdr.PID)
	case *events.ConnectEvent:
		fmt.Fprintf(h, "%d:%d", ev.Port, ev.Hdr.PID)
	}

	return hex.EncodeToString(h.Sum(nil))[:16] // Use first 16 chars as ID
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}
