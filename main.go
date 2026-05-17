// Package main is the coachly entrypoint.
//
// On launch:
//  1. Resolve the per-user data directory (NFR-D-05).
//  2. Pick a free port on 127.0.0.1.
//  3. Start the HTTP server.
//  4. Open the default browser.
//  5. Block until interrupted.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"coachly/internal/platform"
	"coachly/internal/store"
	"coachly/internal/web"
)

//go:embed all:assets all:ui/dist all:internal/images
var embedded embed.FS

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dataDir, err := platform.DataDir()
	if err != nil {
		return fmt.Errorf("data dir: %w", err)
	}
	invoicesDir := filepath.Join(dataDir, "invoices")

	st := store.New(filepath.Join(dataDir, "store.enc"))

	// Flatten embedded assets: serve /assets/* from both ./assets and ./ui/dist.
	assetsFS, err := buildAssetsFS()
	if err != nil {
		return err
	}
	docsImagesFS, err := fs.Sub(embedded, "internal/images")
	if err != nil {
		return err
	}

	srv, err := web.NewServer(st, dataDir, invoicesDir, assetsFS, docsImagesFS)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	addr := ln.Addr().String()
	url := "http://" + addr + "/"

	httpSrv := &http.Server{Handler: srv.Routes()}

	go func() {
		// Wait a beat so the server is accepting before we open the browser.
		time.Sleep(150 * time.Millisecond)
		log.Printf("coachly läuft auf %s", url)
		if err := platform.OpenBrowser(url); err != nil {
			log.Printf("Konnte Browser nicht öffnen: %v — öffne manuell: %s", err, url)
		}
	}()

	// Graceful shutdown on Ctrl-C / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.Serve(ln) }()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}
	return nil
}

// buildAssetsFS merges ./assets and ./ui/dist under one filesystem rooted at /.
// Both subtrees are served at /assets/* by the HTTP handler.
func buildAssetsFS() (fs.FS, error) {
	assetsSub, err := fs.Sub(embedded, "assets")
	if err != nil {
		return nil, err
	}
	uiSub, err := fs.Sub(embedded, "ui/dist")
	if err != nil {
		return nil, err
	}
	return mergedFS{assetsSub, uiSub}, nil
}

// mergedFS serves Open() by trying each backing FS in order.
type mergedFS []fs.FS

func (m mergedFS) Open(name string) (fs.File, error) {
	for _, sub := range m {
		f, err := sub.Open(name)
		if err == nil {
			return f, nil
		}
	}
	return nil, fs.ErrNotExist
}
