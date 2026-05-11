package main

import (
	"log"
	"net/http"
	"os"

	"sealed-secret-api/handlers"
	"sealed-secret-api/middleware"
)

func main() {
	cfg := loadConfig()

	mux := http.NewServeMux()

	// Health check (no auth)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Seal endpoint (basic auth protected)
	mux.Handle("/seal", middleware.BasicAuth(cfg.AuthUser, cfg.AuthPass)(
		http.HandlerFunc(handlers.SealHandler(cfg.CertFile, cfg.ControllerNamespace, cfg.ControllerName)),
	))

	addr := ":" + cfg.Port
	log.Printf("Starting sealed-secret-api on %s", addr)
	log.Printf("Cert file: %s", cfg.CertFile)
	log.Printf("Controller namespace: %s | Controller name: %s", cfg.ControllerNamespace, cfg.ControllerName)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

type config struct {
	Port                string
	AuthUser            string
	AuthPass            string
	CertFile            string
	ControllerNamespace string
	ControllerName      string
}

func loadConfig() config {
	return config{
		Port:                getEnv("PORT", "8080"),
		AuthUser:            getEnv("AUTH_USER", "admin"),
		AuthPass:            mustEnv("AUTH_PASS"),
		CertFile:            getEnv("CERT_FILE", "/certs/sealed-secrets.pem"),
		ControllerNamespace: getEnv("CONTROLLER_NAMESPACE", "kube-system"),
		ControllerName:      getEnv("CONTROLLER_NAME", "sealed-secrets-controller"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Required environment variable %q is not set", key)
	}
	return v
}
