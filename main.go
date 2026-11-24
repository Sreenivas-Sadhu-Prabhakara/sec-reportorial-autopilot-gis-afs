// Command sec-reportorial-autopilot-gis-afs keeps a Philippine corporation's
// reportorial deadlines in one place: when the General Information Sheet falls
// due, counted from the company's own annual meeting, and when the audited
// financial statements fall due under the schedule the Commission published
// for the season.
//
// It is one binary with no dependencies. It serves a small interface on the
// loopback address and stores everything in a plain JSON file on the same
// machine. It makes no outbound network connections of any kind: there is no
// telemetry, no update check, and no link to eFAST or to any other portal. It
// records and reminds; the filing is still made by the user.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

//go:embed web
var webFiles embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on; loopback only unless you deliberately change it")
	data := flag.String("data", "./sec-reportorial-autopilot-gis-afs-data.json", "path to the local data file")
	flag.Parse()

	if err := run(*addr, *data); err != nil {
		log.Fatalf("sec-reportorial-autopilot-gis-afs: %v", err)
	}
}

func run(addr, data string) error {
	store, err := OpenStore(data)
	if err != nil {
		return err
	}

	ui, err := fs.Sub(webFiles, "web")
	if err != nil {
		return fmt.Errorf("could not open the embedded interface: %w", err)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %w", addr, err)
	}

	server := &http.Server{
		Handler:           NewServer(store, ui),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	out := log.New(os.Stdout, "", 0)
	out.Printf("SEC reportorial calendar — General Information Sheet and Audited Financial Statements")
	out.Printf("  open      http://%s", listener.Addr().String())
	out.Printf("  data file %s", store.Path())
	out.Printf("  offline   nothing is sent anywhere; this tool files nothing with the Commission")
	out.Printf("  stop      Ctrl-C")

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
