package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"

	"github.com/zenazn/goji/web"
)

var (
	socket = flag.String("sock", "/var/run/docker.sock", "docker socket")
	port   = flag.String("port", ":8080", "Address port to serve assets")
	assets = flag.String("dir", "./", "Path to static assets root dir")
)

// UnixHandler ...
type UnixHandler struct {
	path string
}

func (h *UnixHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := net.Dial("unix", h.path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	c := httputil.NewClientConn(conn, nil)
	defer c.Close()
	res, err := c.Do(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer res.Body.Close()
	copyHeader(w.Header(), res.Header)
	if _, err := io.Copy(w, res.Body); err != nil {
		log.Println(err)
	}
}

func copyHeader(dest, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dest.Add(k, v)
		}
	}
}

func tcpHandler(sock string) http.Handler {
	u, err := url.Parse(sock)
	if err != nil {
		log.Fatal(err)
	}
	return httputil.NewSingleHostReverseProxy(u)
}

func unixHandler(sock string) http.Handler {
	return &UnixHandler{sock}
}

func createHandler(dir string, sock string) http.Handler {
	var (
		h http.Handler

		mux         = web.New()
		fileHandler = http.FileServer(http.Dir(dir))
	)
	if strings.Contains(sock, "http") {
		h = tcpHandler(sock)
	} else {
		if _, err := os.Stat(sock); err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("unix socket %s does not exist", sock)
			}
			log.Fatal(err)
		}
		h = unixHandler(sock)
	}
	mux.Use(HTTPLogger)
	mux.Handle("/static/", http.StripPrefix("/static", h))
	mux.Handle("/*", fileHandler)
	return mux
}

func main() {
	flag.Parse()

	mux := createHandler(*assets, *socket)

	if err := http.ListenAndServe(*port, mux); err != nil {
		log.Fatal(err)
	}
}
