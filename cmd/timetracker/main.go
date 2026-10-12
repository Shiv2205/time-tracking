package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"
	_ "time/tzdata"

	"time-tracking/internal/app"
	"time-tracking/internal/secrets"
	"time-tracking/internal/store"
	"time-tracking/web"
)

func main() {
	demo := flag.Bool("demo", false, "Use separate sample data and simulate submission; never contact Zoho")
	dir := flag.String("data-dir", "", "Override the local application data directory")
	port := flag.Int("port", 8080, "Loopback HTTP port")
	flag.Parse()
	if *port < 1 || *port > 65535 {
		log.Fatal("Port must be between 1 and 65535")
	}
	if *dir == "" {
		root, err := os.UserConfigDir()
		if err != nil {
			log.Fatal(err)
		}
		*dir = filepath.Join(root, "Tempo")
	}
	if *demo {
		*dir = filepath.Join(*dir, "demo")
	}
	st, err := store.Open(*dir)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	origin := "http://127.0.0.1:" + strconv.Itoa(*port)
	a, err := app.New(st, secrets.New(*dir), *demo, origin+"/api/connection/callback")
	if err != nil {
		log.Fatal(err)
	}
	assets, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Addr: net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)), Handler: a.Handler(assets), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	log.Printf("Tempo is ready at %s (demo=%v); data: %s", origin, *demo, *dir)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
