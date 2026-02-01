package halb

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// responseWriter extends the standard http.ResponseWriter to expose its properties after a request is completed
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	// track status
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

func extractHost(host string) string {
	host = strings.ToLower(host)
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		// no port specified
		return host
	}
	return h
}

// getClientIP uses a best-effort approach to retrieve the client's original IP from the request
func getClientIP(req *http.Request) string {
	// extract connection IP
	remoteIP := extractHost(req.RemoteAddr)

	// default to remote ip if request originated from an untrusted source
	if !isTrustedProxy(remoteIP) {
		return remoteIP
	}

	// extract first ip if x-forwarded-for is provided
	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if ip != "" {
			return ip
		}
	}

	// extract real-ip if provided
	if realIP := req.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// fallback to remote address
	return remoteIP
}

// isTrustedProxy validates if a request originates from a well-known proxy
func isTrustedProxy(ip string) bool {
	parsedIP, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	return parsedIP.IsPrivate()
}
