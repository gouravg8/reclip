package main

import (
	"context"
	"fmt"
)

// App struct
type App struct {
	ctx context.Context
	// previewBase is the loopback preview server URL ("" if unavailable,
	// in which case the UI falls back to a relative /localfile URL).
	previewBase string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if base, err := startPreviewServer(); err == nil {
		a.previewBase = base
	} else {
		println("preview server:", err.Error())
	}
}

// PreviewBase returns the loopback preview server base URL for <video>.
// Empty means the UI should use a relative /localfile URL (prod asset server).
func (a *App) PreviewBase() string {
	return a.previewBase
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
