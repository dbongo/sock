package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// Config is a struct for specifying configuration parameters for the Logger
// middleware.
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

// 2014/12/30 20:41:41 [::1]:62629 GET /api/hello 200 13b 437.126µs HTTP/1.1 curl/7.37.1
// 2014/12/30 20:47:04 [::1]:62930 POST /login 200 490b 224.032597ms HTTP/1.1 curl/7.37.1
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
			"%s %s %s %s %d %dB %v %s %s\n",
			timeStamp,
			res.ip,
			res.method,
			res.rawpath,
			res.responseStatus,
			res.responseBytes,
			time.Since(res.start),
			res.proto,
			res.userAgent,
		)
		os.Stdout.WriteString(logRecord)
	}
}
