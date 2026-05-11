# sealed-secret-api

A minimal Go HTTP API that wraps the `kubeseal` binary.  
Send plaintext key/value pairs over a simple JSON API, get back a ready-to-apply `SealedSecret` YAML.

The API pipes a generated plain `Secret` YAML into `kubeseal --cert` and returns its stdout —
no custom crypto, 100% identical output to running `kubeseal` yourself.

---

## Environment Variables

| Variable               | Required | Default                         | Description                                        |
|------------------------|----------|---------------------------------|----------------------------------------------------|
| `AUTH_PASS`            | **Yes**  | —                               | Basic auth password. App refuses to start without it. |
| `AUTH_USER`            | No       | `admin`                         | Basic auth username                                |
| `PORT`                 | No       | `8080`                          | Listen port                                        |
| `CERT_FILE`            | No       | `/certs/sealed-secrets.pem`     | Path to the sealed-secrets public cert PEM (volume mount) |
| `CONTROLLER_NAMESPACE` | No       | `kube-system`                   | Passed to `kubeseal --controller-namespace`        |
| `CONTROLLER_NAME`      | No       | `sealed-secrets-controller`     | Passed to `kubeseal --controller-name`             |

---

## Fetching the Certificate

```bash
# Option 1 – kubeseal CLI
kubeseal --fetch-cert \
  --controller-namespace kube-system \
  --controller-name sealed-secrets-controller \
  > sealed-secrets.pem

# Option 2 – kubectl directly
kubectl get secret -n kube-system \
  -l sealedsecrets.bitnami.com/sealed-secrets-key=active \
  -o jsonpath='{.items[0].data.tls\.crt}' | base64 -d > sealed-secrets.pem
```

---

## Build & Run

```bash
# Build (KUBESEAL_VERSION is baked in at build time)
docker build \
  --build-arg KUBESEAL_VERSION=0.36.6 \
  -t sealed-secret-api .

# Run — mount cert, set password
docker run -p 8080:8080 \
  -e AUTH_PASS=supersecret \
  -v $(pwd)/sealed-secrets.pem:/certs/sealed-secrets.pem:ro \
  sealed-secret-api
```

Docker Compose:
```bash
mkdir -p certs && cp sealed-secrets.pem certs/
AUTH_PASS=supersecret docker-compose up
```

---

## API Reference

### `GET /healthz`
No authentication required.

**Response:** `200 OK` — body: `ok`

---

### `POST /seal`
Encrypts secrets via `kubeseal` and returns `SealedSecret` YAML.

**Authentication:** HTTP Basic Auth

**Request body (JSON):**
```json
{
  "name": "my-secret",
  "namespace": "production",
  "secrets": {
    "DATABASE_URL": "postgres://user:pass@host/db",
    "API_KEY": "hunter2"
  },
  "scope": "strict"
}
```

| Field       | Type              | Required | Values |
|-------------|-------------------|----------|--------|
| `name`      | string            | Yes      | SealedSecret resource name |
| `namespace` | string            | Yes      | Target namespace |
| `secrets`   | map[string]string | Yes      | One or more key → plaintext pairs |
| `scope`     | string            | No       | `strict` (default), `namespace-wide`, `cluster-wide` |

**Success (`200 OK`):**
```json
{
  "yaml": "apiVersion: bitnami.com/v1alpha1\nkind: SealedSecret\n..."
}
```

**Error:**
```json
{
  "error": "kubeseal error: ..."
}
```

---

## curl Examples

```bash
# Seal and print YAML
curl -s -u admin:supersecret \
  -X POST http://localhost:8085/seal \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-db-secret",
    "namespace": "production",
    "secrets": {
      "DB_PASSWORD": "s3cr3t",
      "API_TOKEN": "tok_abc123"
    }
  }' | jq -r .yaml

# Seal and apply directly
curl -s -u admin:supersecret \
  -X POST http://localhost:8085/seal \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-secret",
    "namespace": "staging",
    "scope": "namespace-wide",
    "secrets": { "TOKEN": "abc123" }
  }' | jq -r .yaml | kubectl apply -f -
```

---

## Sealing Scopes

| Scope            | Can be applied to            |
|------------------|------------------------------|
| `strict`         | Exact namespace + name only  |
| `namespace-wide` | Any name within namespace    |
| `cluster-wide`   | Any namespace and name       |

---

## Kubernetes Deployment

```bash
# Create auth secret
kubectl create secret generic sealed-secret-api-auth \
  -n kube-system \
  --from-literal=AUTH_USER=admin \
  --from-literal=AUTH_PASS=$(openssl rand -base64 32)

# Apply
kubectl apply -f k8s-deployment.yaml

# Port-forward for testing
kubectl port-forward -n kube-system svc/sealed-secret-api 8080:8080
```

The `k8s-deployment.yaml` mounts the cert directly from the sealed-secrets
controller's TLS secret — no manual cert management required.
