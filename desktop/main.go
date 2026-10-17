package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"log"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewDesktop()
	err := wails.Run(&options.App{
		Title: "老孙抢你妹 2026", Width: 1180, Height: 800, MinWidth: 960, MinHeight: 720,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 235, G: 235, B: 235, A: 255},
		OnStartup:        app.startup, OnShutdown: app.shutdown, Bind: []interface{}{app},
		Mac: &mac.Options{TitleBar: mac.TitleBarHiddenInset(), WebviewIsTransparent: false},
	})
	if err != nil {
		log.Fatal(err)
	}
}
