// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"

	"github.com/aclements/go-obj/obj"
)

func main() {
	flagHttp := flag.String("http", "localhost:0", "HTTP service address (e.g., ':6060')")
	flagDev := flag.String("dev", "", "compile and serve web files from file system `path`")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] objfile\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	// Open the object file.
	objPath := flag.Arg(0)
	f, err := os.Open(objPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
	objF, err := obj.Open(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", objPath, err)
	}

	var staticFS fs.FS = embedFS
	if *flagDev == "" {
		// Use the embedded static files.
		var err error
		staticFS, err = fs.Sub(staticFS, "web/dist-prod")
		if err != nil {
			log.Fatalf("embedded file system missing web/dist-prod: %s", err)
		}
	} else {
		// Build web sources in development mode and serve them directly from
		// the file system.
		buildWeb(*flagDev)
		staticFS = os.DirFS("web/dist-dev")
	}

	server, err := newServer(objF, *flagHttp, staticFS)
	if err != nil {
		log.Fatalf("failed to start server: %s", err)
	}
	server.addView(NewHexView(server))
	server.addView(NewAsmView(server))

	addr := "http://" + server.listener.Addr().String()
	fmt.Printf("Listening on %s\n", addr)
	err = server.serve()

	if webpackWatch != nil {
		webpackWatch.Close()
	}
	log.Fatalf("failed to start HTTP server: %v", err)
}

// The following directives build a static copy of the web files in
// web/dist-prod and then embed them in the binary.

//go:generate npm install
//go:generate webpack --mode production

//go:embed web/dist-prod
var embedFS embed.FS

var webpackWatch *os.File

func buildWeb(path string) {
	// Make sure we have node_modules.
	log.Printf("installing NPM packages...")
	cmd := exec.Command("npm", "install", "--no-audit", "--no-fund")
	cmd.Dir = path
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("installing NPM packages failed: %s", err)
	}

	// Run webpack in non-watch mode once to make sure everything is built
	// successfully.
	webpack := []string{"webpack", "--stats", "errors-warnings", "--mode", "development", "--devtool", "inline-source-map"}
	log.Printf("building web assets with webpack...")
	cmd = exec.Command("npx", webpack...)
	cmd.Dir = path
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("webpack failed: %s", err)
	}

	// Start webpack in watch mode. We tell webpack to exit if stdin closes and
	// set up a pipe that will be closed by the kernel if the server process
	// exits.
	//
	// TODO: This will build everything a second time and is surprisingly slow
	// even with filesystem caching enabled. Maybe we could start webpack's own
	// dev server and proxy requests to it (and get rid of web/dist-dev) or vice
	// versa? That would also fix the delay between saving and being able to
	// reload.
	webpack = append(webpack, "-w", "--watch-options-stdin")
	cmd = exec.Command("npx", webpack...)
	cmd.Dir = path
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	var err error
	cmd.Stdin, webpackWatch, err = os.Pipe()
	if err != nil {
		log.Fatalf("creating watch pipe for webpack: %v", err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatalf("webpack watch failed: %s", err)
	}
}
