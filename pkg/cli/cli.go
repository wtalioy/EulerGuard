package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"aegis/pkg/config"
	"aegis/pkg/core"
	"aegis/pkg/events"
	"aegis/pkg/metrics"
	"aegis/pkg/output"
	"aegis/pkg/profiler"
	"aegis/pkg/rules"
	"aegis/pkg/tracer"
	"aegis/pkg/types"
)

type CLI struct {
	Opts     config.Options
	Handlers *events.HandlerChain
	Profiler *profiler.Profiler
	Core     *core.CoreComponents
}

func RunCLI(opts config.Options, ctx context.Context) error {
	cli := &CLI{
		Opts:     opts,
		Handlers: events.NewHandlerChain(),
	}

	if os.Geteuid() != 0 {
		return fmt.Errorf("must run as root (current euid=%d)", os.Geteuid())
	}

	components, err := core.Bootstrap(cli.Opts)
	if err != nil {
		return err
	}
	defer components.Close()
	cli.Core = components

	if cli.Opts.LearnMode {
		cli.Profiler = profiler.NewProfiler()
		cli.Handlers.Add(cli.Profiler)
		log.Printf("Learning mode enabled for %v, rules will be merged into: %s", cli.Opts.LearnDuration, cli.Opts.RulesPath)
	}

	meter := metrics.NewRateMeter(2 * time.Second)
	printer, err := output.NewPrinter(cli.Opts.JSONLines, meter, cli.Opts.LogFile, cli.Opts.LogBufferSize)
	if err != nil {
		return fmt.Errorf("create printer: %w", err)
	}
	defer printer.Close()

	cli.Handlers.Add(&cliAlertHandler{
		processTree: components.ProcessTree,
		printer:     printer,
		ruleEngine:  components.RuleEngine,
	})

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if cli.Opts.LearnMode {
		go cli.runLearnModeTimer(runCtx, cancel)
	}

	go func() {
		<-runCtx.Done()
		_ = cli.Core.Reader.Close()
	}()

	if cli.Opts.LearnMode {
		log.Printf("Aegis learning mode started (BPF: %s)", cli.Opts.BPFPath)
	} else {
		log.Printf("Aegis tracer ready (BPF: %s, rules: %s)", cli.Opts.BPFPath, cli.Opts.RulesPath)
	}

	return tracer.EventLoop(cli.Core.Reader, cli.Handlers, cli.Core.ProcessTree, cli.Core.WorkloadReg, cli.Core.Storage, cli.Core.ProfileReg)
}

func (cli *CLI) runLearnModeTimer(ctx context.Context, cancel context.CancelFunc) {
	timer := time.NewTimer(cli.Opts.LearnDuration)
	defer timer.Stop()

	select {
	case <-timer.C:
		log.Printf("Learning complete. Collected %d behavior profiles.", cli.Profiler.Count())
		cli.Profiler.Stop()

		generatedRules := cli.Profiler.GenerateRules()
		if len(generatedRules) == 0 {
			log.Printf("No rules generated from learning mode")
		} else {
			existingRules, err := rules.LoadRules(cli.Opts.RulesPath)
			if err != nil {
				existingRules = []types.Rule{}
			}

			mergedRules := rules.MergeRules(existingRules, generatedRules)
			if err := rules.SaveRules(cli.Opts.RulesPath, mergedRules); err != nil {
				log.Printf("Error saving rules: %v", err)
			} else {
				log.Printf("Merged %d new rules into %s (total: %d rules)", len(generatedRules), cli.Opts.RulesPath, len(mergedRules))
			}
		}
		cancel()
	case <-ctx.Done():
	}
}
