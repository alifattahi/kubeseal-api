package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

// SealRequest is the JSON body sent by the client.
type SealRequest struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Secrets   map[string]string `json:"secrets"`
	// Scope: strict (default) | namespace-wide | cluster-wide
	Scope string `json:"scope,omitempty"`
}

// SealResponse is returned on success.
type SealResponse struct {
	YAML string `json:"yaml"`
}

// ErrorResponse is returned on failure.
type ErrorResponse struct {
	Error string `json:"error"`
}

// plainSecretTemplate is the Secret YAML we pipe into kubeseal.
var plainSecretTemplate = template.Must(template.New("secret").Parse(`apiVersion: v1
kind: Secret
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
type: Opaque
stringData:
{{- range $k, $v := .Secrets }}
  {{ $k }}: {{ $v | js }}
{{- end }}
`))

// SealHandler returns an http.HandlerFunc that shells out to the kubeseal
// binary and returns the resulting SealedSecret YAML.
func SealHandler(certFile, controllerNamespace, controllerName string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "only POST is allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SealRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, fmt.Sprintf("invalid JSON body: %v", err), http.StatusBadRequest)
			return
		}

		if err := validateRequest(req); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Scope == "" {
			req.Scope = "strict"
		}

		// Build the plain Secret YAML that will be piped to kubeseal
		var plainSecret bytes.Buffer
		if err := plainSecretTemplate.Execute(&plainSecret, req); err != nil {
			log.Printf("ERROR rendering secret template: %v", err)
			jsonError(w, "failed to render secret template", http.StatusInternalServerError)
			return
		}

		//log.Printf("Rendered secret:\n%s", plainSecret.String())

		// Build kubeseal arguments
		args := []string{
			"--cert", certFile,
			"--controller-namespace", controllerNamespace,
			"--controller-name", controllerName,
			"--scope", req.Scope,
			"--format", "yaml",
		}
		//fmt.Printf("%#v\n", args)
		// Run kubeseal with a timeout
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "kubeseal", args...)
		cmd.Stdin = &plainSecret

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		log.Printf("Sealing %q/%q with %d keys (scope: %s)", req.Namespace, req.Name, len(req.Secrets), req.Scope)


		fullCmd := append([]string{"kubeseal"}, args...)
		log.Printf("Executing command: %q", fullCmd)

		if err := cmd.Run(); err != nil {
			log.Printf("ERROR kubeseal failed: %v | stderr: %s", err, stderr.String())
			jsonError(w, fmt.Sprintf("kubeseal error: %s", strings.TrimSpace(stderr.String())), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SealResponse{YAML: stdout.String()})
	}
}

func validateRequest(req SealRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("field 'name' is required")
	}
	if strings.TrimSpace(req.Namespace) == "" {
		return fmt.Errorf("field 'namespace' is required")
	}
	if len(req.Secrets) == 0 {
		return fmt.Errorf("field 'secrets' must contain at least one entry")
	}
	validScopes := map[string]bool{"strict": true, "namespace-wide": true, "cluster-wide": true}
	if req.Scope != "" && !validScopes[req.Scope] {
		return fmt.Errorf("invalid scope %q: must be strict, namespace-wide, or cluster-wide", req.Scope)
	}
	return nil
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
