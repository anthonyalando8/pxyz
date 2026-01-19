// scripts/generate_key.go
package main

import (
	"crypto-service/internal/security"
	"fmt"
	"log"
)

func main() {
	key, err := security.GenerateMasterKey()
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Println("==============================================")
	fmt.Println("Generated AES-256 Master Key:")
	fmt.Println("==============================================")
	fmt.Println(key)
	fmt.Println("==============================================")
	fmt.Println("Add this to your .env file as:")
	fmt.Println("CRYPTO_MASTER_KEY=" + key)
	fmt.Println("==============================================")
	fmt.Println("⚠️  KEEP THIS KEY SECURE!")
	fmt.Println("⚠️  DO NOT COMMIT TO VERSION CONTROL!")
	fmt.Println("==============================================")
}