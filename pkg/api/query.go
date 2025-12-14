package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aegis/pkg/ai"
	"aegis/pkg/events"
	"aegis/pkg/storage"
)

// QueryFilter represents a structured query filter.
type QueryFilter struct {
	Types       []string   `json:"types"`       // "exec", "file", "connect"
	Processes   []string   `json:"processes"`   // 进程名列表
	Actions     []string   `json:"actions"`     // "block", "monitor", "allow"
	PIDs        []uint32   `json:"pids"`        // PID 列表
	CgroupIDs   []uint64   `json:"cgroup_ids"`  // CgroupID 列表
	TimeWindow  TimeWindow `json:"time_window"` // 时间窗口
	Correlation bool       `json:"correlation"` // 是否关联同 PID 事件
}

// TimeWindow represents a time range.
type TimeWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// QueryRequest represents a query request.
type QueryRequest struct {
	Filter    *QueryFilter `json:"filter"`     // Structured filter
	Semantic  string       `json:"semantic"`   // Natural language query (mutually exclusive with Filter)
	Page      int          `json:"page"`       // Page number (1-based)
	Limit     int          `json:"limit"`      // Results per page
	SortBy    string       `json:"sort_by"`    // "time", "relevance"
	SortOrder string       `json:"sort_order"` // "asc", "desc"
}

// QueryResponse represents a query response.
type QueryResponse struct {
	Events     []*storage.Event `json:"events"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalPages int              `json:"total_pages"`
	TypeCounts struct {
		Exec    int `json:"exec"`
		File    int `json:"file"`
		Connect int `json:"connect"`
	} `json:"type_counts"`
}

// SemanticQueryService provides semantic query capabilities.
type SemanticQueryService struct {
	aiService *ai.Service
	store     storage.EventStore
}

// NewSemanticQueryService creates a new semantic query service.
func NewSemanticQueryService(aiService *ai.Service, store storage.EventStore) *SemanticQueryService {
	return &SemanticQueryService{
		aiService: aiService,
		store:     store,
	}
}

// Query executes a structured query.
func (s *SemanticQueryService) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	if req.Filter == nil {
		return nil, fmt.Errorf("filter is required")
	}

	// Build storage filter
	filter := storage.Filter{
		PIDs:      req.Filter.PIDs,
		CgroupIDs: req.Filter.CgroupIDs,
		Processes: req.Filter.Processes,
	}

	// Convert type strings to EventType
	for _, t := range req.Filter.Types {
		switch strings.ToLower(t) {
		case "exec":
			filter.Types = append(filter.Types, events.EventTypeExec)
		case "file", "fileopen":
			filter.Types = append(filter.Types, events.EventTypeFileOpen)
		case "connect", "network":
			filter.Types = append(filter.Types, events.EventTypeConnect)
		}
	}

	// Query events
	var allEvents []*storage.Event
	var err error

	if !req.Filter.TimeWindow.Start.IsZero() && !req.Filter.TimeWindow.End.IsZero() {
		allEvents, err = s.store.Query(req.Filter.TimeWindow.Start, req.Filter.TimeWindow.End)
		if err != nil {
			return nil, fmt.Errorf("query events: %w", err)
		}
	} else {
		// Get latest events if no time window
		allEvents, err = s.store.Latest(10000) // Get up to 10k events
		if err != nil {
			return nil, fmt.Errorf("get latest events: %w", err)
		}
	}

	// Apply filters (simplified - would use indexer in production)
	filteredEvents := s.applyFilters(allEvents, &filter)

	// Apply correlation if requested
	if req.Filter.Correlation {
		filteredEvents = s.applyCorrelation(filteredEvents)
	}

	// Sort events
	filteredEvents = s.sortEvents(filteredEvents, req.SortBy, req.SortOrder)

	// Paginate
	total := len(filteredEvents)
	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit < 1 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	start := (page - 1) * limit
	end := start + limit
	if end > total {
		end = total
	}

	var displayedEvents []*storage.Event
	if start < total {
		displayedEvents = filteredEvents[start:end]
	} else {
		// Return empty slice instead of nil to avoid JSON marshaling issues
		displayedEvents = make([]*storage.Event, 0)
	}

	totalPages := (total + limit - 1) / limit

	// Calculate type-specific counts from all filtered events (before pagination)
	typeCounts := struct {
		Exec    int `json:"exec"`
		File    int `json:"file"`
		Connect int `json:"connect"`
	}{}
	for _, ev := range filteredEvents {
		if ev == nil {
			continue
		}
		switch ev.Type {
		case events.EventTypeExec:
			typeCounts.Exec++
		case events.EventTypeFileOpen:
			typeCounts.File++
		case events.EventTypeConnect:
			typeCounts.Connect++
		}
	}

	return &QueryResponse{
		Events:     displayedEvents,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		TypeCounts: typeCounts,
	}, nil
}
// applyFilters applies filters to events.
func (s *SemanticQueryService) applyFilters(events []*storage.Event, filter *storage.Filter) []*storage.Event {
	if filter == nil {
		return events
	}

	var result []*storage.Event
	for _, event := range events {
		if s.matchesFilter(event, filter) {
			result = append(result, event)
		}
	}
	return result
}

// matchesFilter checks if an event matches the filter.
func (s *SemanticQueryService) matchesFilter(event *storage.Event, filter *storage.Filter) bool {
	// Check type
	if len(filter.Types) > 0 {
		matched := false
		for _, t := range filter.Types {
			if event.Type == t {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check PID
	if len(filter.PIDs) > 0 {
		matched := false
		var pid uint32
		switch ev := event.Data.(type) {
		case *events.ExecEvent:
			pid = ev.Hdr.PID
		case *events.FileOpenEvent:
			pid = ev.Hdr.PID
		case *events.ConnectEvent:
			pid = ev.Hdr.PID
		}
		for _, p := range filter.PIDs {
			if pid == p {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check CgroupID
	if len(filter.CgroupIDs) > 0 {
		matched := false
		var cgroupID uint64
		switch ev := event.Data.(type) {
		case *events.ExecEvent:
			cgroupID = ev.Hdr.CgroupID
		case *events.FileOpenEvent:
			cgroupID = ev.Hdr.CgroupID
		case *events.ConnectEvent:
			cgroupID = ev.Hdr.CgroupID
		}
		for _, c := range filter.CgroupIDs {
			if cgroupID == c {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check process name (simplified)
	if len(filter.Processes) > 0 {
		matched := false
		var processName string
		switch ev := event.Data.(type) {
		case *events.ExecEvent:
			processName = strings.TrimRight(string(ev.Hdr.Comm[:]), "\x00")
		case *events.FileOpenEvent:
			processName = strings.TrimRight(string(ev.Hdr.Comm[:]), "\x00")
		case *events.ConnectEvent:
			processName = strings.TrimRight(string(ev.Hdr.Comm[:]), "\x00")
		}
		for _, p := range filter.Processes {
			if strings.Contains(processName, p) || strings.Contains(p, processName) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// applyCorrelation applies correlation to events (groups by PID).
func (s *SemanticQueryService) applyCorrelation(events []*storage.Event) []*storage.Event {
	// Simplified correlation - would use more sophisticated logic in production
	return events
}

// sortEvents sorts events by the specified criteria.
func (s *SemanticQueryService) sortEvents(events []*storage.Event, sortBy, sortOrder string) []*storage.Event {
	// Simplified sorting - would use more sophisticated logic in production
	// For now, just return events as-is (they should already be time-ordered from store)
	return events
}
