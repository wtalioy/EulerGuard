package ui

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"

	"eulerguard/pkg/config"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var WailsAssets embed.FS

type WailsEmitter struct {
	ctx context.Context
}

func (w *WailsEmitter) Emit(eventName string, data any) {
	if w.ctx != nil {
		runtime.EventsEmit(w.ctx, eventName, data)
	}
}

func RunWails(opts config.Options) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("must run as root (current euid=%d)", os.Geteuid())
	}

	app := NewApp(opts)
	emitter := &WailsEmitter{}

	onStartup := func(ctx context.Context) {
		emitter.ctx = ctx
		app.Bridge().SetEmitter(emitter)
		app.Startup(ctx)
	}

	go func() {
		app.WaitForReady()
		log.Println("Wails ready, starting tracer...")
		if err := app.Run(); err != nil {
			log.Printf("Tracer error: %v", err)
		}
	}()

	return wails.Run(&options.App{
		Title:            "EulerGuard",
		Width:            1400,
		Height:           900,
		MinWidth:         1024,
		MinHeight:        768,
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 15, A: 255},
		AssetServer:      &assetserver.Options{Assets: WailsAssets},
		OnStartup:        onStartup,
		OnShutdown:       app.Shutdown,
		Bind:             []any{app},
		Linux:            &linux.Options{ProgramName: "EulerGuard"},
	})
}
