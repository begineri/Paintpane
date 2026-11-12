package api

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// NamespaceResolver is called when a static namespace lookup fails.
// It receives the namespace and collectionID from the URL and returns
// the direct path to serve (bypassing resolveCollectionPath) and whether it matched.
type NamespaceResolver func(namespace, collectionID string) (directPath string, ok bool)

// Server represents the Local REST/WebSocket server.
type Server struct {
	httpServer *http.Server
	mux        *http.ServeMux
	upgrader   websocket.Upgrader

	// WebSocket management
	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex

	// Local file serving
	namespaces map[string]string // name -> absPath
	resolver   NamespaceResolver // dynamic fallback resolver

	// Callbacks
	onAddCollection func(url string) error
}

var (
	globalServer *Server
)

// GetServer returns the global API server instance.
func GetServer() *Server {
	return globalServer
}

// NewServer creates a new API server.
func NewServer() *Server {
	s := &Server{
		mux: http.NewServeMux(),
		upgrader: websocket.Upgrader{
			// The program's own clients send no Origin; of browsers, only an extension may connect.
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "" || isExtensionOrigin(origin)
			},
		},
		clients:    make(map[*websocket.Conn]bool),
		namespaces: make(map[string]string),
	}
	s.setupRoutes()
	globalServer = s
	return s
}

// Who may call the local API. Any program of the person's can reach 127.0.0.1, so the guards are
// about browsers: a web page the person has open must not list or read their pictures (/local/),
// add collections (/add) or listen to the wallpaper feed (/ws).
//
//   - /local/ serves the program's own sources (Favorites, Local Folder, Google Photos), which call
//     it with Go's HTTP client: no Origin, no Fetch Metadata. Every browser request is refused.
//   - /health, /add and /ws are the Spice browser extension's. Paintpane does not ship it, and the
//     program itself never calls them, but a person may load it from the Spice project. Its manifest has
//     host permissions for 127.0.0.1:49452, so it needs no CORS headers: requests from an
//     extension origin are accepted, those from web pages are not.
//   - Every route requires a loopback Host header, which shuts out a page that rebinds its own
//     domain to 127.0.0.1 (its same-origin requests carry neither Origin nor Fetch Metadata).
func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/health", localOrExtension(s.handleHealth))
	s.mux.HandleFunc("/ws", localOrExtension(s.handleWebSocket))
	s.mux.HandleFunc("/add", localOrExtension(s.handleAdd))
	s.mux.HandleFunc("/local/", localOnly(s.handleLocal))
}

// localOnly lets through requests from local programs, refusing browsers altogether.
func localOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) || fromBrowserPage(r) {
			http.Error(w, "this route is for Paintpane only", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// localOrExtension lets through requests from local programs and from browser extensions.
func localOrExtension(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) || (fromBrowserPage(r) && !isExtensionOrigin(r.Header.Get("Origin"))) {
			http.Error(w, "this route is for Paintpane and its browser extension only", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// fromBrowserPage reports whether a browser sent the request: it names an Origin (also "null", from
// a file:// page) or Fetch Metadata other than "none" (typed into the address bar). Browsers send
// Fetch Metadata to loopback addresses, so a cross-site request without an Origin (an image, a
// no-cors fetch) is caught too.
func fromBrowserPage(r *http.Request) bool {
	site := r.Header.Get("Sec-Fetch-Site")
	return r.Header.Get("Origin") != "" || (site != "" && site != "none")
}

// isExtensionOrigin reports whether origin belongs to a browser extension.
func isExtensionOrigin(origin string) bool {
	for _, scheme := range []string{"chrome-extension://", "moz-extension://", "safari-web-extension://"} {
		if strings.HasPrefix(origin, scheme) && len(origin) > len(scheme) {
			return true
		}
	}
	return false
}

// RegisterNamespace registers a local directory to be served under /local/{name}.
func (s *Server) RegisterNamespace(name, path string) {
	s.namespaces[name] = path
}

// SetDynamicResolver sets a callback for resolving namespaces not found in the static map.
// This enables providers like Local Folder to serve user-selected directories dynamically.
func (s *Server) SetDynamicResolver(resolver NamespaceResolver) {
	s.resolver = resolver
}

// HandleDebug registers a dev-tooling handler under /debug/<path>. Requests that
// come from a web page (they carry an Origin or a cross-site Sec-Fetch-Site
// header, or name another host, as a page that rebinds its domain to
// 127.0.0.1 does) are refused, so only local tools such as curl can use these
// routes. Release builds do not register any.
func (s *Server) HandleDebug(path string, h http.HandlerFunc) {
	s.mux.HandleFunc("/debug/"+path, func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) || fromBrowserPage(r) {
			http.Error(w, "debug routes are for local tools only", http.StatusForbidden)
			return
		}
		h(w, r)
	})
}

// isLoopbackHost reports whether a Host header names this machine by "localhost" or a loopback
// address, with or without a port.
func isLoopbackHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// SetAddCollectionHandler sets the callback for adding a collection.
func (s *Server) SetAddCollectionHandler(handler func(url string) error) {
	s.onAddCollection = handler
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Start starts the server.
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:              "127.0.0.1:49452",
		Handler:           s.mux,
		ReadHeaderTimeout: 3 * time.Second, // G112: Potential Slowloris Attack
	}
	// This is blocking
	return s.httpServer.ListenAndServe()
}

// Stop stops the server.
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(context.Background())
	}
	return nil
}

// BroadcastWallpaper sends a "set_wallpaper" command to all connected clients.
func (s *Server) BroadcastWallpaper(path string) error {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	msg := map[string]string{
		"type": "set_wallpaper",
		"url":  path, // Using 'url' key to match extension expectation, though it's a path
	}

	for client := range s.clients {
		err := client.WriteJSON(msg)
		if err != nil {
			log.Printf("Failed to broadcast to client: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
	return nil
}

// BroadcastLanguage sends a "set_language" command to all connected clients.
func (s *Server) BroadcastLanguage(langCode string) error {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	msg := map[string]string{
		"type":     "set_language",
		"language": langCode,
	}

	for client := range s.clients {
		err := client.WriteJSON(msg)
		if err != nil {
			log.Printf("Failed to broadcast language: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
	return nil
}
