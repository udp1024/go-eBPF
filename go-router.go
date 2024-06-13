package main

import (
	"encoding/json"
	"fmt"

	//"io"
	"net/http"
	//"net/url"
	"os"
	"sync"
	"time"
)

// ServiceRule defines the structure for service routing rules
type ServiceRule struct {
	Service     string `json:"service"`
	Destination string `json:"destination"`
}

// RouterConfig holds the array of service rules
type RouterConfig struct {
	Rules []ServiceRule `json:"rules"`
}

// Session represents an established network session
type Session struct {
	DateTimeStamp   time.Time `json:"DateTimeStamp"`
	SourceIP        string    `json:"sourceIP"`
	RequestService  string    `json:"requestService"`
	SourcePort      string    `json:"sourcePort"`
	DestinationIP   string    `json:"DestinationIP"`
	DestinationPort string    `json:"DestinationPort"`
}

// SessionManager manages established sessions
type SessionManager struct {
	Sessions map[string]*Session
	mu       sync.Mutex
}

// NewSessionManager creates a new SessionManager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		Sessions: make(map[string]*Session),
	}
}

// AddOrUpdateSession adds a new session or updates an existing one
func (sm *SessionManager) AddOrUpdateSession(s *Session) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sessionKey := s.SourceIP + ":" + s.SourcePort
	sm.Sessions[sessionKey] = s
}

// CleanupSessions removes sessions that have been inactive for more than 30 seconds
func (sm *SessionManager) CleanupSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for key, session := range sm.Sessions {
		if time.Since(session.DateTimeStamp) > 30*time.Second {
			delete(sm.Sessions, key)
		}
	}
}

// SaveSessionsToFile saves the current sessions to a file
func (sm *SessionManager) SaveSessionsToFile(filename string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sessions := make([]*Session, 0, len(sm.Sessions))
	for _, session := range sm.Sessions {
		sessions = append(sessions, session)
	}
	data, err := json.Marshal(sessions)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// LoadSessionsFromFile loads sessions from a file
func (sm *SessionManager) LoadSessionsFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	var sessions []*Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, session := range sessions {
		sessionKey := session.SourceIP + ":" + session.SourcePort
		sm.Sessions[sessionKey] = session
	}
	return nil
}

// loadConfig loads routing rules from a JSON file
func loadConfig(filename string) (*RouterConfig, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	config := &RouterConfig{}
	err = json.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// routeTraffic routes the incoming request based on the service rules
func routeTraffic(config *RouterConfig, service string) (string, bool) {
	for _, rule := range config.Rules {
		if rule.Service == service {
			return rule.Destination, true
		}
	}
	return "", false
}

// handler handles incoming HTTP requests and routes them based on service rules
func handler(config *RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service := r.Header.Get("X-Service-Type") // Custom header to identify the service type
		if destination, ok := routeTraffic(config, service); ok {
			// Here you would route the request to the destination
			// For this example, we'll just print the destination
			w.Write([]byte("Routing to: " + destination))
		} else {
			http.Error(w, "Service not found", http.StatusNotFound)
		}
	}
}

// Rule represents a routing rule with a service and a destination
type Rule struct {
	Service     string `json:"service"`
	Destination string `json:"destination"`
}

// Router holds the routing rules
type Router struct {
	Rules []Rule `json:"rules"`
}

// NewRouter creates a new Router from a JSON file
func NewRouter(filename string) (*Router, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var router Router
	if err := json.Unmarshal(data, &router); err != nil {
		return nil, err
	}
	return &router, nil
}

// RouteRequest routes an HTTP request based on the router's rules
func (r *Router) RouteRequest(req *http.Request) (string, bool) {
	service := req.Header.Get("X-Service-Type") // Custom header to identify the service type
	for _, rule := range r.Rules {
		if rule.Service == service {
			return rule.Destination, true
		}
	}
	return "", false
}

func main() {
	router, err := NewRouter("go-router.json")
	if err != nil {
		panic(err)
	}

	sessionManager := NewSessionManager()
	go func() {
		for {
			time.Sleep(30 * time.Second)
			sessionManager.CleanupSessions()
		}
	}()
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		sourceIP := req.RemoteAddr
		sourcePort := req.URL.Port()
		requestService := req.Header.Get("X-Service-Type")

		if destination, ok := router.RouteRequest(req); ok {
			// Here you would implement the actual forwarding logic to the destination
			// For this example, we'll simulate a successful routing
			session := &Session{
				DateTimeStamp:   time.Now(),
				SourceIP:        sourceIP,
				RequestService:  requestService,
				SourcePort:      sourcePort,
				DestinationIP:   destination,
				DestinationPort: "80", // Assuming port 80 for HTTP
			}
			sessionManager.AddOrUpdateSession(session)
			sessionManager.SaveSessionsToFile("go-sessions.json")

			fmt.Fprintf(w, "Request routed to: %s\n", destination)
		} else {
			http.Error(w, "Service not found", http.StatusNotFound)
		}
	})

	if err := sessionManager.LoadSessionsFromFile("go-sessions.json"); err != nil {
		fmt.Println("Error loading sessions:", err)
	}

	fmt.Println("Server is running on port 8080")
	http.ListenAndServe(":8080", nil)
}
