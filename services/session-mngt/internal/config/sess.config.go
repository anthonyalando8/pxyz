package config
import (
	"os"
	"github.com/joho/godotenv"
)
type AppConfig struct {
	GRPCAddr string
}

func Load() AppConfig {
	_ = godotenv.Load()
	return AppConfig{
		GRPCAddr: os.Getenv("GRPC_ADDR"),
	}
}
