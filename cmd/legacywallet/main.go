package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	if err := wails.Run(&options.App{
		Title:     "Legacy Wallet",
		Width:     1180,
		Height:    780,
		MinWidth:  960,
		MinHeight: 620,
		Frameless: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Windows: &windows.Options{
			Theme: windows.Light,
		},
		BackgroundColour: &options.RGBA{R: 212, G: 208, B: 200, A: 1},
		OnStartup:        app.Startup,
		OnBeforeClose:    app.BeforeClose,
		OnShutdown:       app.Shutdown,
		Bind:             []any{app},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Legacy Wallet: %v\n", err)
		os.Exit(1)
	}
}
