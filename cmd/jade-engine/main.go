package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/mcembalest/jade/engine"
)

func main() {
	root := flag.String("root", "", "workspace directory")
	address := flag.String("address", "127.0.0.1:7333", "HTTP listen address")
	flag.Parse()
	if *root == "" {
		log.Fatal("-root is required")
	}
	host, portText, err := net.SplitHostPort(*address)
	if err != nil {
		log.Fatal(err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		log.Fatal("-address must bind a loopback IP")
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		log.Fatal(err)
	}
	handler, err := engine.NewHandler(*root, port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("JaDE engine: http://%s", *address)
	log.Fatal(http.ListenAndServe(*address, handler))
}
