module ptn-auth-service

go 1.24.5

replace x/shared => ../../shared

require (
	github.com/go-chi/chi/v5 v5.2.2
	github.com/go-chi/cors v1.2.2
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/jackc/pgx/v5 v5.7.6
	github.com/joho/godotenv v1.5.1
	github.com/redis/go-redis/v9 v9.12.1
	golang.org/x/crypto v0.41.0
	google.golang.org/grpc v1.74.2
)

require github.com/stretchr/testify v1.10.0 // indirect

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	golang.org/x/image v0.30.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250818200422-3122310a409c // indirect
	google.golang.org/protobuf v1.36.7
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	x/shared v0.0.0-00010101000000-000000000000
)
