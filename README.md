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



## Routers (Original Structure)

package router

import (
	"net/http"
	"os"
	//"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"auth-service/internal/handler"
	"x/shared/auth/middleware"
	"x/shared/utils/cache"
)

func SetupRoutes(
	r chi.Router,
	h *handler.AuthHandler,
	oauthHandler *handler.OAuth2Handler,
	auth *middleware.MiddlewareWithClient,
	wsHandler *handler.WSHandler,
	cache *cache.Cache,
	rdb *redis.Client,
) chi.Router {
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "ngrok-skip-browser-warning"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	//r.Use(auth.RateLimit(rdb, 100, time.Minute, time.Minute, "global_user_auth"))

	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		_ = os.MkdirAll(uploadDir, 0755)
	}

	// ================================
	// OAUTH2 PUBLIC ENDPOINTS (Outside /api/v1)
	// ================================
	r.Route("/oauth2", func(oauth chi.Router) {
		// Authorization endpoint - public
		oauth.Get("/authorize", oauthHandler.Authorize)
		
		// Token endpoint - public (client authenticates via credentials)
		oauth.Post("/token", oauthHandler.Token)
		
		// Token revocation - public
		oauth.Post("/revoke", oauthHandler.Revoke)
		
		// Token introspection - public (requires client auth)
		oauth.Post("/introspect", oauthHandler.Introspect)
		
		// Consent endpoints - require user authentication
		oauth.Group(func(consent chi.Router) {
			consent.Use(auth.Require([]string{"main", "temp"}, nil, nil))
			consent.Get("/consent", oauthHandler.ShowConsent)
			consent.Post("/consent", oauthHandler.GrantConsent)
		})
	})

	r.Route("/api/v1", func(api chi.Router) {
		//api.Use(auth.RateLimit(rdb, 5, 30*time.Second, 30*time.Second, "user_auth"))
		
		// ---------------- Public ----------------
		api.Group(func(pub chi.Router) {
			pub.Get("/auth/health", h.Health)
			pub.Post("/auth/submit-identifier", h.SubmitIdentifier)
			pub.Post("/auth/google", h.GoogleAuthHandler)
			pub.Post("/auth/telegram", h.TelegramLogin)
			pub.Post("/auth/apple", h.AppleAuthHandler)
			pub.Post("/auth/password/forgot", h.HandleForgotPassword)
			pub.Handle("/auth/uploads/*", http.StripPrefix("/auth/uploads/", http.FileServer(http.Dir(uploadDir))))
		})

		// ---------------- Account Initialization ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp"}, []string{"init_account"}, nil))
			g.Post("/auth/verify-identifier", h.VerifyIdentifier)
			g.Post("/auth/set-password", h.SetPassword)
			g.Post("/auth/login-password", h.LoginWithPassword)
			g.Get("/auth/cached-status", h.GetCachedUserStatus)
			g.Get("/auth/resend-identifier-code", h.ResendOTP)
		})

		// ---------------- Password Reset ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp"}, []string{"password_reset"}, nil))
			g.Post("/auth/password/reset", h.HandleResetPassword)
		})

		// ---------------- Registration & OTP ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp", "main"}, []string{"register", "email_change", "incomplete_profile", "general", "verify-otp", "phone_change"}, nil))
			g.Post("/auth/register/otp/request", h.HandleRequestOTP)
			g.Post("/auth/register/otp/verify", h.HandleVerifyOTP)
		})

		// ---------------- Email & Phone Change ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp", "main"}, []string{"email_change"}, nil))
			g.Patch("/auth/email", h.HandleChangeEmail)
		})
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp", "main"}, []string{"phone_change"}, nil))
			g.Patch("/auth/phone/update", h.HandlePhoneChange)
		})

		// ---------------- Profile Completion ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp", "main"}, []string{"general", "register", "incomplete_profile"}, nil))
			g.Post("/auth/profile/nationality", h.HandleUpdateNationality)
		})

		// ---------------- Authenticated User ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"main"}, nil, nil))

			g.Get("/auth/ws", wsHandler.HandleWS)

			g.Route("/auth/2fa", func(r chi.Router) {
				r.Get("/init", h.HandleInitiate2FA)
				r.Post("/enable", h.HandleEnable2FA)
				r.Post("/disable", h.HandleDisable2FA)
				r.Post("/verify", h.HandleVerify2FA)
				r.Get("/status", h.Handle2FAStatus)
			})

			g.Route("/auth/profile", func(r chi.Router) {
				r.Get("/", h.HandleProfile)
				r.Post("/update", h.HandleUpdateProfile)
				r.Get("/picture/get", h.GetProfilePicture)
				r.Post("/picture", h.UploadProfilePicture)
				r.Delete("/picture/remove", h.DeleteProfilePicture)
				r.Get("/email/request-change", h.HandleRequestEmailChange)
			})

			g.Route("/auth/preferences", func(r chi.Router) {
				r.Get("/", h.HandleGetPreferences)
				r.Post("/update", h.HandleUpdatePreferences)
			})

			g.Route("/auth/password", func(r chi.Router) {
				r.Get("/request-change", h.HandleRequestPasswordChange)
			})

			g.Route("/auth/phone", func(r chi.Router) {
				r.Get("/request-change", h.HandleRequestPhoneChange)
				r.Get("/request-verification", h.HandleRequestPhoneVerification)
				r.Get("/get-verification-status", h.HandleGetPhoneVerificationStatus)
			})

			g.Route("/auth/email", func(r chi.Router) {
				r.Get("/request-verification", h.HandleRequestEmailVerification)
				r.Get("/get-verification-status", h.HandleGetEmailVerificationStatus)
			})

			g.Route("/auth/sessions", func(r chi.Router) {
				r.Get("/", h.ListSessionsHandler(auth.Client))
				r.Delete("/", h.LogoutAllHandler(auth.Client, rdb))
				r.Delete("/{id}", h.DeleteSessionByIDHandler(auth.Client))
			})
			g.Delete("/auth/logout", h.LogoutHandler(auth.Client))

			// ================================
			// OAUTH2 CLIENT MANAGEMENT (Authenticated)
			// ================================
			g.Route("/oauth2/clients", func(oauth chi.Router) {
				oauth.Post("/", oauthHandler.RegisterClient)
				oauth.Get("/", oauthHandler.ListMyClients)
				oauth.Get("/{client_id}", oauthHandler.GetClient)
				oauth.Put("/{client_id}", oauthHandler.UpdateClient)
				oauth.Delete("/{client_id}", oauthHandler.DeleteClient)
				oauth.Post("/{client_id}/regenerate-secret", oauthHandler.RegenerateClientSecret)
			})

			// ================================
			// USER CONSENT MANAGEMENT (Authenticated)
			// ================================
			g.Route("/oauth2/consents", func(consent chi.Router) {
				consent.Get("/", oauthHandler.ListMyConsents)
				consent.Delete("/", oauthHandler.RevokeAllConsents)
				consent.Delete("/{client_id}", oauthHandler.RevokeConsent)
			})
		})
	})

	return r
}