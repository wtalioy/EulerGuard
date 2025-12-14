package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"aegis/pkg/ai"
	"aegis/pkg/config"
	"aegis/pkg/core"
	"aegis/pkg/events"
	"aegis/pkg/profiler"
	"aegis/pkg/storage"
	"aegis/pkg/tracer"
)

type App struct {
	ctx  context.Context
	opts config.Options

	core *core.CoreComponents

	stats  *Stats
	bridge *Bridge

	profiler *profiler.Profiler
	learning struct {
		active    bool
		startTime time.Time
		duration  time.Duration
	}

	aiService *ai.Service
	sentinel  *ai.Sentinel

	ready       chan struct{}
	stopWatcher chan struct{}
	watcherMu   sync.Mutex
	lastRuleMod time.Time
}

func NewApp(opts config.Options) *App {
	stats := NewStats()

	var aiService *ai.Service
	var err error
	aiService, err = ai.NewService(opts.AI)
	if err != nil {
		log.Printf("[AI] Failed to initialize: %v", err)
	}

	return &App{
		opts:        opts,
		stats:       stats,
		bridge:      NewBridge(stats),
		aiService:   aiService,
		ready:       make(chan struct{}),
		stopWatcher: make(chan struct{}),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	close(a.ready)
}

func (a *App) WaitForReady() { <-a.ready }

func (a *App) Shutdown(ctx context.Context) {
	if a.profiler != nil {
		a.profiler.Stop()
	}
	if a.sentinel != nil {
		a.sentinel.Stop()
	}
	close(a.stopWatcher)
}

func (a *App) Bridge() *Bridge        { return a.bridge }
func (a *App) Stats() *Stats          { return a.stats }
func (a *App) AIService() *ai.Service { return a.aiService }
func (a *App) Sentinel() *ai.Sentinel { return a.sentinel }
func (a *App) Options() *config.Options { return &a.opts }

func (a *App) Diagnose(ctx context.Context) (*ai.DiagnosisResult, error) {
	if a.aiService == nil || !a.aiService.IsEnabled() {
		return nil, nil
	}

	procTreeSize := 0
	if a.core != nil && a.core.ProcessTree != nil {
		procTreeSize = a.core.ProcessTree.Size()
	}

	return a.aiService.Diagnose(ctx, a.stats, a.core.WorkloadReg, procTreeSize)
}

func (a *App) Chat(ctx context.Context, sessionID, message string) (*ai.ChatResponse, error) {
	if a.aiService == nil || !a.aiService.IsEnabled() {
		return nil, nil
	}

	procTreeSize := 0
	if a.core != nil && a.core.ProcessTree != nil {
		procTreeSize = a.core.ProcessTree.Size()
	}

	return a.aiService.Chat(ctx, sessionID, message, a.stats, a.core.WorkloadReg, procTreeSize)
}

func (a *App) ChatStream(ctx context.Context, sessionID, message string) (<-chan ai.ChatStreamToken, error) {
	if a.aiService == nil || !a.aiService.IsEnabled() {
		return nil, nil
	}

	procTreeSize := 0
	if a.core != nil && a.core.ProcessTree != nil {
		procTreeSize = a.core.ProcessTree.Size()
	}

	return a.aiService.ChatStream(ctx, sessionID, message, a.stats, a.core.WorkloadReg, procTreeSize)
}

func (a *App) GetChatHistory(sessionID string) []ai.Message {
	if a.aiService == nil {
		return nil
	}
	return a.aiService.GetChatHistory(sessionID)
}

func (a *App) ClearChat(sessionID string) {
	if a.aiService != nil {
		a.aiService.ClearChat(sessionID)
	}
}

// Phase 3: AI Intent Parsing
func (a *App) ParseIntent(ctx context.Context, input string, reqCtx *ai.RequestContext) (*ai.Intent, error) {
	if a.aiService == nil || !a.aiService.IsEnabled() {
		return nil, fmt.Errorf("AI service not available")
	}
	return a.aiService.ParseIntent(ctx, input, reqCtx)
}

// Phase 3: AI Rule Generation
func (a *App) GenerateRule(ctx context.Context, req *ai.RuleGenRequest) (*ai.RuleGenResponse, error) {
	if a.aiService == nil || !a.aiService.IsEnabled() {
		return nil, fmt.Errorf("AI service not available")
	}
	if a.core == nil {
		return nil, fmt.Errorf("core components not available")
	}
	return a.aiService.GenerateRule(ctx, req, a.core.RuleEngine, a.core.Storage)
}

// Phase 3: AI Event Explanation
func (a *App) ExplainEvent(ctx context.Context, req *ai.ExplainRequest, event *storage.Event) (*ai.ExplainResponse, error) {
	if a.aiService == nil || !a.aiService.IsEnabled() {
		return nil, fmt.Errorf("AI service not available")
	}
	if a.core == nil {
		return nil, fmt.Errorf("core components not available")
	}
	return a.aiService.ExplainEvent(ctx, req, event, a.core.RuleEngine, a.core.Storage, a.core.ProfileReg)
}

// Phase 3: AI Context Analysis
func (a *App) Analyze(ctx context.Context, req *ai.AnalyzeRequest) (*ai.AnalyzeResponse, error) {
	if a.aiService == nil || !a.aiService.IsEnabled() {
		return nil, fmt.Errorf("AI service not available")
	}
	if a.core == nil {
		return nil, fmt.Errorf("core components not available")
	}
	return a.aiService.Analyze(ctx, req, a.core.ProfileReg, a.core.WorkloadReg, a.core.RuleEngine)
}

func (a *App) Run() error {
	log.Println("Starting eBPF tracer...")

	components, err := core.Bootstrap(a.opts)
	if err != nil {
		return err
	}
	defer components.Close()

	a.core = components

	a.stats.SetWorkloadCountFunc(components.WorkloadReg.Count)

	a.bridge.SetRuleEngine(components.ProcessTree, components.RuleEngine)
	a.bridge.SetWorkloadRegistry(components.WorkloadReg)

	// Initialize Sentinel if AI service is available
	if a.aiService != nil {
		a.sentinel = ai.NewSentinel(
			a.aiService,
			components.RuleEngine,
			components.Storage,
			components.ProfileReg,
		)
		a.sentinel.Start()
		log.Println("[Sentinel] AI Sentinel started")
	}

	go a.watchRulesFile()

	chain := events.NewHandlerChain()
	chain.Add(a.bridge)

	log.Println("eBPF tracer started")
	return tracer.EventLoop(components.Reader, chain, components.ProcessTree, components.WorkloadReg, components.Storage, components.ProfileReg)
}

func (a *App) watchRulesFile() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	if info, err := os.Stat(a.opts.RulesPath); err == nil {
		a.lastRuleMod = info.ModTime()
	}

	for {
		select {
		case <-a.stopWatcher:
			return
		case <-ticker.C:
			info, err := os.Stat(a.opts.RulesPath)
			if err != nil {
				continue
			}

			a.watcherMu.Lock()
			if info.ModTime().After(a.lastRuleMod) {
				a.lastRuleMod = info.ModTime()
				a.watcherMu.Unlock()

				if err := a.reloadRules(); err != nil {
					log.Printf("Failed to reload rules: %v", err)
				} else {
					log.Println("Rules reloaded due to file change")
					a.bridge.NotifyRulesReload()
				}
			} else {
				a.watcherMu.Unlock()
			}
		}
	}
}

func (a *App) reloadRules() error {
	if a.core == nil {
		return nil
	}

	if err := a.core.ReloadRules(a.opts.RulesPath); err != nil {
		return err
	}

	a.bridge.SetRuleEngine(a.core.ProcessTree, a.core.RuleEngine)

	return nil
}
