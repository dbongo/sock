package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// Config ...
type Config struct {
	Prefix               string
	DisableAutoBrackets  bool
	RemoteAddressHeaders []string
}

// Logger ...
type Logger struct {
	http.Handler

	ch   chan *Record
	conf Config
}

// Record ...
type Record struct {
	http.ResponseWriter

	start               time.Time
	ip, method, rawpath string
	responseStatus      int
	responseBytes       int64
	proto, userAgent    string
}

// HTTPLogger ...
func HTTPLogger(h http.Handler) http.Handler {
	l := logHTTP(h)
	fn := func(rw http.ResponseWriter, req *http.Request) {
		l.ServeHTTP(rw, req)
	}
	return http.HandlerFunc(fn)
}

func (log *Logger) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	addr := req.RemoteAddr
	for _, headerKey := range log.conf.RemoteAddressHeaders {
		if val := req.Header.Get(headerKey); len(val) > 0 {
			addr = val
			break
		}
	}
	record := &Record{
		ResponseWriter: w,

		start:          time.Now().UTC(),
		ip:             addr,
		method:         req.Method,
		rawpath:        req.RequestURI,
		responseStatus: http.StatusOK,
		proto:          req.Proto,
		userAgent:      req.UserAgent(),
	}
	log.Handler.ServeHTTP(record, req)
	log.ch <- record
}

func (r *Record) Write(b []byte) (int, error) {
	written, err := r.ResponseWriter.Write(b)
	r.responseBytes += int64(written)
	return written, err
}

// WriteHeader ...
func (r *Record) WriteHeader(status int) {
	r.responseStatus = status
	r.ResponseWriter.WriteHeader(status)
}

func logHTTP(h http.Handler) http.Handler {
	log := &Logger{
		Handler: h,
		ch:      make(chan *Record, 1000),
	}
	go log.response()
	return log
}

// [2015/01/22 23:30:48] [127.0.0.1:53773] 200 GET / 2.490822ms 4921B HTTP/1.1 curl/7.35.0
func (log *Logger) response() {
	for {
		res := <-log.ch
		timeStamp := fmt.Sprintf(
			"%04d/%02d/%02d %02d:%02d:%02d",
			res.start.Year(),
			res.start.Month(),
			res.start.Day(),
			res.start.Hour(),
			res.start.Minute(),
			res.start.Second(),
		)
		logRecord := fmt.Sprintf(
			"[%s] [%s] %d %s %s %v %dB %s %s\n",
			timeStamp,
			res.ip,
			res.responseStatus,
			res.method,
			res.rawpath,
			time.Since(res.start),
			res.responseBytes,
			res.proto,
			res.userAgent,
		)
		os.Stdout.WriteString(logRecord)
	}
}
