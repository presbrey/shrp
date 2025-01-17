package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

var (
	// Define command-line flags
	hostHeader  = flag.String("host", "", "Value of the Host header to send to the next hop server")
	nextHop     = flag.String("nexthop", "https://httpbin.org/", "URL of the next hop (target) server")
	listenAddr  = flag.String("listen", ":8000", "Address to listen on")
	logRequests = flag.Bool("log", false, "Enable request logging")
	flagDaemon  = flag.Bool("daemon", false, "Run as a daemon")
	flagDebug   = flag.Bool("debug", false, "Enable debug mode")
	insecureSSL = flag.Bool("insecure", false, "Ignore SSL certificate errors")
)

func init() {
	// Parse the flags
	flag.Parse()
	if *flagDebug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}
}

func main() {
	// Handle daemonization
	if *flagDaemon {
		if !runningAsDaemon() {
			daemonizeProcess()
			return
		}
	}

	// Parse the next hop URL
	target, err := url.Parse(*nextHop)
	if err != nil {
		log.Fatalf("Invalid next hop URL: %v", err)
	}

	// Create a reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Optionally set the Host header
	if *hostHeader != "" {
		proxy.Director = func(req *http.Request) {
			req.Host = *hostHeader
			req.URL.Host = target.Host
			req.URL.Scheme = target.Scheme
		}
	}

	// Optionally add request logging
	if *logRequests {
		proxy.Transport = &loggingRoundTripper{http.DefaultTransport}
	}

	switch t := http.DefaultTransport.(type) {
	case *http.Transport:
		// Optionally ignore SSL certificate errors
		if *insecureSSL {
			if t.TLSClientConfig == nil {
				t.TLSClientConfig = &tls.Config{}
			}
			t.TLSClientConfig.InsecureSkipVerify = true
		}
		if *hostHeader != "" {
			t.TLSClientConfig.ServerName = *hostHeader
		}
		if *flagDebug {
			log.Printf("%+v", t.TLSClientConfig)
		}
	}
	switch p := proxy.Transport.(type) {
	case *http.Transport:
		// Optionally ignore SSL certificate errors
		if *insecureSSL {
			if p.TLSClientConfig == nil {
				p.TLSClientConfig = &tls.Config{}
			}
			p.TLSClientConfig.InsecureSkipVerify = true
		}
		if *hostHeader != "" {
			p.TLSClientConfig.ServerName = *hostHeader
		}
		if *flagDebug {
			log.Printf("%+v", p.TLSClientConfig)
		}
	case *loggingRoundTripper:
		// Optionally ignore SSL certificate errors
		if *insecureSSL {
			if p.wrapped.(*http.Transport).TLSClientConfig == nil {
				p.wrapped.(*http.Transport).TLSClientConfig = &tls.Config{}
			}
			p.wrapped.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true
		}
		if *hostHeader != "" {
			p.wrapped.(*http.Transport).TLSClientConfig.ServerName = *hostHeader
		}
		if *flagDebug {
			log.Printf("%+v", p.wrapped.(*http.Transport).TLSClientConfig)
		}
	}

	// Create a handler that will be used to serve all requests
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	// Start the server
	fmt.Printf("Starting reverse proxy server on %s, forwarding to %s\n", *listenAddr, *nextHop)
	log.Fatal(http.ListenAndServe(*listenAddr, handler))
}

// loggingRoundTripper is a custom RoundTripper that logs requests
type loggingRoundTripper struct {
	wrapped http.RoundTripper
}

func (l *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	log.Printf("Proxying request: %s %s", req.Method, req.URL)
	return l.wrapped.RoundTrip(req)
}

// runningAsDaemon checks if the current process is running as a daemon
func runningAsDaemon() bool {
	return os.Getenv("FORKED") == "1"
}
