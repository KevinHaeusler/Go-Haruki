package webhooks

import (
	"encoding/json"
	"log"
	"net/http"
)

// Start starts a minimal HTTP server that exposes a webhook endpoint.
// addr example: ":8080"
// path example: "/webhook"
// authToken is optional. If provided, the "Authorization" header must match it.
// onNotify is invoked after a successful JSON decode.
func Start(addr, path, authToken string, onNotify func(NotificationPayload)) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if authToken != "" {
			authHeader := r.Header.Get("Authorization")
			log.Printf("Webhook received request from %s. Method: %s, Auth Header: %s", r.RemoteAddr, r.Method, authHeader)
			if authHeader != authToken && authHeader != ("Bearer "+authToken) {
				log.Printf("Webhook auth failed. Expected: %s or Bearer %s, Got: %s", authToken, authToken, authHeader)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		} else {
			log.Printf("Webhook received request from %s. Method: %s, Auth enabled: false", r.RemoteAddr, r.Method)
		}

		defer r.Body.Close()
		var p NotificationPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if onNotify != nil {
			onNotify(p)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("webhook server error: %v", err)
		}
	}()
	return srv, nil
}
