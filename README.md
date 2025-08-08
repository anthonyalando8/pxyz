**Project X scalable microservices project structure in Go**, with support for:

* gRPC communication between services
* Docker for containerization
* Kubernetes for orchestration
* Independent deployment per service
* Easy addition of new services later

---

## Microservices Project Structure (Clean, Scalable & Extendable)

```
/X/               # Root of your microservices project
├── /services/                   # All microservices go here
│   ├── /auth-service/           # User auth and session management
│   │   ├── cmd/                 # Main entry point (main.go)
│   │   ├── internal/            # Business logic (domain, usecases, handlers)
│   │   │   ├── domain/
│   │   │   ├── usecase/
│   │   │   └── handler/
│   │   ├── proto/               # Compiled .pb.go files from shared .proto
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── go.sum
│   ├── /cashier-service/        # Deposits, withdrawals, transfers
│   ├── /dashboard-service/      # Net worth, recent activity, announcements
│   ├── /trade-service/          # Market selection, trades, charts
│   └── /kyc-service/            # ID verification & document processing
│   ├── /wallet-service/
|   ├── /dispute-service/
|   ├── /notification-service/
|   └── ...
├── /proto/                      # Shared .proto files across services
│   ├── auth.proto
│   ├── cashier.proto
│   └── ...
│
├── /deployments/                # Kubernetes manifests
│   ├── auth/
│   │   ├── deployment.yaml
│   │   └── service.yaml
│   ├── cashier/
│   └── ...
│
├── /configs/                    # Config templates (.env, YAML, etc.)
│   └── app-config.yaml
│
├── /scripts/                    # Dev utilities (build, lint, test)
│   ├── build.sh
│   └── gen-proto.sh
│
├── docker-compose.yaml          # Optional: local dev/testing
├── Makefile                     # Centralized build/test commands
├── README.md
```

---

## Folder Purpose

| Folder                | Purpose                                                                          |
| --------------------- | -------------------------------------------------------------------------------- |
| `services/`           | Each microservice is an isolated Go module (its own go.mod)                      |
| `proto/`              | Shared protobufs for gRPC; compiled per service into `/proto` folder inside each |
| `deployments/`        | Kubernetes manifests (can also support Helm/Kustomize later)                     |
| `configs/`            | App-specific configuration (env variables, secrets, etc.)                        |
| `scripts/`            | Build, test, and proto-generation scripts                                        |
| `docker-compose.yaml` | For spinning up services locally for testing                                     |
| `Makefile`            | For building, formatting, and generating code across services                    |

---

## Tools/Practices Supported

| Category               | Tools/Practices                                           |
| ---------------------- | --------------------------------------------------------- |
| Communication          | gRPC, Protobuf                                            |
| Deployment             | Docker, Kubernetes                                        |
| Service Discovery      | DNS via K8s service names                                 |
| Secrets                | K8s secrets or Vault                                      |
| Observability          | Prometheus, Grafana, Loki, Jaeger                         |
| CI/CD (later)          | GitHub Actions, ArgoCD, Helm                              |
| Rate Limiting          | Redis, API Gateway                                        |
| Auth                   | JWT, OAuth2 (Auth service)                                |
| Database (per service) | Postgres, Redis, TimescaleDB, etc. (each owns its own DB) |

---

## Service Example: `/services/auth-service/`

```
auth-service/
├── cmd/
│   └── main.go                 # Starts gRPC server
├── internal/
│   ├── domain/                 # Entities like User, Session
│   ├── usecase/                # Business logic (RegisterUser, LoginUser)
│   └── handler/                # gRPC/HTTP handlers (adapters)
├── proto/                      # Compiled .pb.go files
├── Dockerfile
├── go.mod
└── go.sum
```

---

## Adding a New Service Later?

Just:

1. `mkdir services/new-service`
2. `go mod init github.com/yourorg/pxyz/services/new-service`
3. Define its proto in `/proto/`
4. Implement handlers, usecases, domain
5. Add `Dockerfile` + `k8s` manifests

> It's fully isolated and ready to scale independently.

---

> Running docker 
```
cd services/auth-service
docker build -t auth-service .
docker run -p 8080:8080 auth-service
docker-compose up --build # build whole project
# run postgres sql file
psql -U postgres -d auth_db -f init.sql
```

---

## Step-by-Step Setup for Windows

### 1. Install `protoc` (Protocol Buffers Compiler)

1. Download `protoc` from:
   [https://github.com/protocolbuffers/protobuf/releases](https://github.com/protocolbuffers/protobuf/releases)

   * Get the latest release for **Windows (`protoc-<version>-win64.zip`)**
2. Extract the ZIP to:
   `C:\Program Files\protoc\`
3. Add `C:\Program Files\protoc\bin` to your **System PATH**:

   * Open Start Menu → search “Environment Variables” → Edit system variables → `Path` → Add

> Confirm it's working:

```bash
protoc --version
```

---

### 2. Install gRPC Plugins (Go)

Open **Command Prompt or PowerShell** and run:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

> These executables will be placed in:

```
C:\Users\<YourUsername>\go\bin
```

Add that path to your **System PATH** so `protoc` can find them.

---

### 3. Generate Proto Files

Once setup is complete:

```bash
cd proto
protoc --go_out=. --go-grpc_out=. auth.proto
```

This will generate:

* `auth.pb.go`
* `auth_grpc.pb.go`

---

### 4. Install Docker Desktop for Windows

[https://www.docker.com/products/docker-desktop/](https://www.docker.com/products/docker-desktop/)

* Enable WSL2 backend (recommended).
* Confirm installation:

```bash
docker --version
```

---

### 5. (Optional) Install Kubernetes Tools

To run Kubernetes locally:

#### Option A: Minikube

```bash
choco install minikube
```

#### Option B: Kind (Kubernetes-in-Docker)

```bash
choco install kind
```

Install `kubectl` CLI:

```bash
choco install kubernetes-cli
```

---

### 6. VS Code Extensions

Install these from the **Extensions panel** (`Ctrl+Shift+X`):

| Extension Name          | Description                     |
| ----------------------- | ------------------------------- |
| Go                      | Official Go plugin              |
| Docker                  | Build & run containers visually |
| YAML                    | Helpful for Kubernetes configs  |
| Proto3 Language Support | Syntax highlight for .proto     |

---