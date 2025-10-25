# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is **Online Boutique**, a cloud-native microservices demo application maintained by Honeycomb. It's a 10-tier microservices e-commerce platform written in 5 languages (Go, Java, .NET, Node.js, Python) that demonstrates OpenTelemetry instrumentation, Kubernetes deployment, and observability patterns.

**Origin**: Forked from Google Cloud Platform's microservices-demo in 2021. Migrated from OpenCensus/Stackdriver to OpenTelemetry with Honeycomb as the telemetry backend.

## Build and Development Commands

### Prerequisites
- Docker Desktop or Minikube (4 CPUs, 6GB RAM, 32GB disk)
- kubectl
- skaffold
- Helm

### Deployment
```bash
# Deploy all services (first time ~20 minutes)
skaffold run

# Deploy with auto-rebuild on code changes
skaffold dev

# Clean up deployment
skaffold delete
```

### Setup Honeycomb Integration
```bash
# Add Honeycomb API key as Kubernetes secret
export HONEYCOMB_API_KEY=<your-key>
kubectl create secret generic honeycomb --from-literal=api-key=$HONEYCOMB_API_KEY

# Install OpenTelemetry Collector
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm install opentelemetry-collector open-telemetry/opentelemetry-collector \
  --set mode=deployment \
  --set image.repository="otel/opentelemetry-collector-k8s" \
  --values ./kubernetes-manifests/additional_resources/opentelemetry-collector-values.yaml
```

### Service-Specific Development

**Go services** (frontend, checkoutservice, productcatalogservice, shippingservice):
```bash
cd src/<service-name>
go mod download
go build
go test ./...
```

**Java service** (adservice):
```bash
cd src/adservice
./gradlew build
./gradlew test
```

**C# service** (cartservice):
```bash
cd src/cartservice
dotnet build
dotnet test tests/
```

**Node.js services** (currencyservice, paymentservice):
```bash
cd src/<service-name>
npm install
npm test
```

**Python services** (emailservice, recommendationservice, loadgenerator):
```bash
cd src/<service-name>
pip install -r requirements.txt
python -m pytest  # if tests exist
```

**Frontend JavaScript** (browser instrumentation):
```bash
cd src/frontend
npm install
npm run build  # rebuilds instrumentation-load.js in ./dist
```

### Kubernetes Operations
```bash
# Verify cluster connection
kubectl get nodes

# Check pod status
kubectl get pods

# View service logs
kubectl logs <pod-name>

# Access frontend (Docker Desktop)
# http://localhost:80

# Access frontend (Minikube)
minikube service frontend-external
```

## GitHub Actions CI/CD

The repository uses GitHub Actions for continuous integration and deployment. Each service has its own workflow that automatically builds and pushes Docker images to AWS ECR.

### Workflow Pattern

All service workflows follow the same pattern:
- **Trigger**: Push to `main` branch with changes in the service's `src/<service-name>/` directory
- **Path filtering**: Uses `dorny/paths-filter@v3` to only run when service-specific files change
- **Authentication**: AWS OIDC authentication via `aws-actions/configure-aws-credentials@v4`
- **Build**: Docker image built from service's Dockerfile
- **Tagging**: Images tagged with both `${{ github.sha }}` and `latest`
- **Push**: Images pushed to AWS ECR repository `microservices-demo/<service-name>`
- **Runner**: Executes on `ubuntu-24.04-arm`

### Service Workflows

Each service has a dedicated workflow file in `.github/workflows/`:
- `adservice.yml`
- `cartservice.yml`
- `checkoutservice.yml`
- `currencyservice.yml`
- `emailservice.yml`
- `frontend.yml`
- `loadgenerator.yml`
- `paymentservice.yml`
- `productcatalogservice.yml`
- `recommendationservice.yml`
- `shippingservice.yml`

### Required Secrets

Workflows require the following GitHub secrets:
- `AWS_GHA_OIDC_ROLE`: AWS IAM role ARN for OIDC authentication
- `AWS_REGION`: AWS region for ECR (e.g., `us-east-1`)

### Workflow Behavior

**When changes are pushed to main:**
1. Workflow checks if files in `src/<service-name>/**` changed
2. If yes, authenticates to AWS and builds Docker image
3. Pushes image with git SHA tag and `latest` tag to ECR
4. Skips execution if no service files changed (path filter optimization)

**Example workflow execution:**
```bash
# After pushing changes to src/frontend/ on main branch:
# - Builds Docker image from src/frontend/Dockerfile
# - Tags as: <registry>/microservices-demo/frontend:<sha>
# - Tags as: <registry>/microservices-demo/frontend:latest
# - Pushes both tags to AWS ECR
```

## Architecture

### Service Communication
All services communicate via **gRPC** using Protocol Buffers defined in `./pb/demo.proto`. The frontend exposes an HTTP interface for browser access.

### Service Dependencies
```
frontend (HTTP) → all other services (gRPC)
  ├─→ adservice
  ├─→ cartservice → redis
  ├─→ checkoutservice
  │    ├─→ cartservice
  │    ├─→ currencyservice
  │    ├─→ emailservice
  │    ├─→ paymentservice
  │    └─→ shippingservice
  ├─→ currencyservice (highest QPS)
  ├─→ productcatalogservice
  ├─→ recommendationservice
  └─→ shippingservice

loadgenerator → frontend (simulates user traffic)
```

### Key Services

| Service | Language | Port | Purpose |
|---------|----------|------|---------|
| frontend | Go | 8080 | Web UI, SSR, main entry point |
| checkoutservice | Go | 5050 | Order orchestration, payment/shipping coordination |
| cartservice | C# | 7070 | Shopping cart with Redis backend |
| productcatalogservice | Go | 3550 | Product catalog from JSON |
| currencyservice | Node.js | 7000 | Currency conversion (highest QPS) |
| paymentservice | Node.js | 50051 | Payment processing (mock) |
| shippingservice | Go | 50051 | Shipping calculations (mock) |
| emailservice | Python | 8080 | Order confirmation emails (mock) |
| recommendationservice | Python | 8080 | Product recommendations |
| adservice | Java | 9555 | Contextual advertisements |
| loadgenerator | Python/Locust | - | Synthetic traffic generation |

## OpenTelemetry Instrumentation

All services are instrumented with OpenTelemetry. Each service README in `src/<service-name>/README.md` contains language-specific instrumentation details.

### Common Patterns

**Go services** use:
- `otlptracegrpc` exporter to OpenTelemetry Collector
- `otelgrpc` interceptors for gRPC client/server
- `otelhttp` for HTTP instrumentation (frontend)
- Resource attributes with `semconv.ServiceNameKey`
- Composite TextMapPropagator (Baggage + TraceContext)

**Initialization example** (Go):
```go
endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
if endpoint == "" {
    endpoint = "opentelemetry-collector:4317"
}
```

**gRPC instrumentation** (all languages):
- Server: Interceptors on incoming requests
- Client: Interceptors on outgoing requests

### Baggage Usage

**Frontend** (`placeOrderHandler` in `handlers.go`):
- Sets `app.user_id` and `app.request_id` in Baggage
- Propagates to all downstream services

**Checkoutservice** (`PlaceOrder` function):
- Reads `userid` and `requestID` from Baggage
- Adds `app.order_id` to Baggage

Baggage requires a Span Processor to export to telemetry backends.

### Frontend Browser Instrumentation

The frontend includes OpenTelemetry Web SDK for Real User Monitoring (RUM):
- Document loads, resource loads, page loads
- Web vitals (LCP, CLS, INP)
- Telemetry sent to 'browser' dataset in Honeycomb
- Build with `npm run build` in `src/frontend/`

## Demo Story: Intentional Performance Degradation

This application includes **intentional bugs** for demonstration purposes:

### Checkoutservice Memory Leak
- **Location**: `src/checkoutservice/`
- **Behavior**: Internal cache grows with each request
- **Result**: OOM crash after ~4 hours, pod restart
- **Side effect**: Growing cache causes exponential latency via synthetic `getDiscounts` SQL delays

### Frontend User ID Assignment
- **Location**: `src/frontend/middleware.go` (`ensureSessionID`)
- **Behavior**: When checkoutservice cache exceeds threshold, assigns problematic user ID `20109`
- **Result**: Single user experiences severe performance degradation while others remain unaffected

### Discovery Pattern
Use Honeycomb BubbleUp and SLO features to identify:
1. High-cardinality user ID patterns (finding user `20109` among thousands)
2. Memory leak correlation with latency
3. Cache size impact on SQL query delays

## Honeycomb Analysis Guidelines

When analyzing telemetry in Honeycomb:

### Service Metrics
- Services: `service.name` attribute
- Span names: `name` attribute
- Users: `app.user_id` attribute
- Requests: event counts
- Errors: `error = true` attribute
- Latency: `duration_ms` attribute
- HTTP details: `http.url`, `http.status_code`, `http.response_bytes`

### Sampling Analysis
- `SampleRate`: 1 = 100%, 5 = 20%
- `meta.refinery.reason`: Refinery sampling rule name
- `meta.original_sample_rate`: Pre-Refinery sample rate
- `meta.refinery_send_reason`: Why trace was sent (got_root, expired, ejected_full, ejected_memsize, late_span)

### RUM Analysis (Browser Dataset)
- Web vitals: LCP, CLS, INP scores
- Document/resource loads with URLs
- Session tracking: `session.id`
- HTTP status codes + `exception.message`

### Kubernetes Integration
- Datasets: `k8s-events`, `k8s-metrics`
- Resource attributes: `k8s.cluster.name`, `k8s.node.name`, `k8s.pod.name`
- Cross-reference app traces with k8s platform metrics during incidents

## File Structure

```
/
├── src/                          # Service source code
│   ├── <service-name>/
│   │   ├── Dockerfile
│   │   ├── README.md            # Service-specific OTel instrumentation details
│   │   └── <language-specific files>
├── pb/                           # Protocol Buffer definitions
│   └── demo.proto
├── kubernetes-manifests/         # K8s deployment manifests
│   ├── <service-name>.yaml
│   └── additional_resources/
│       └── opentelemetry-collector-values.yaml
├── skaffold.yaml                 # Build and deployment configuration
└── .cursor/rules/                # AI coding assistant rules
    ├── honeycomb.mdc            # Honeycomb analysis guidelines
    └── kubernetes.mdc           # K8s platform analysis guidelines
```

## Important Notes

- **Deployment**: Use `skaffold run/dev` at repository root, NOT individual service builds
- **Proto changes**: All services must rebuild if `pb/demo.proto` changes
- **Environment variables**: Services default to `opentelemetry-collector:4317` if `OTEL_EXPORTER_OTLP_ENDPOINT` not set
- **Secrets**: Never commit Honeycomb API keys; always use Kubernetes secrets
- **Demo purpose**: This code intentionally includes performance bugs for observability demonstrations
