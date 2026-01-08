// pkg/security/mpesa_credentials.go
package security

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/base64"
    "encoding/pem"
    "fmt"
    "os"
)

// GenerateSecurityCredential generates M-Pesa security credential from certificate
func GenerateSecurityCredential(certPath, initiatorPassword string) (string, error) {
    // Read certificate file
    certData, err := os.ReadFile(certPath)
    if err != nil {
        return "", fmt. Errorf("failed to read certificate: %w", err)
    }

    // Parse PEM block
    block, _ := pem.Decode(certData)
    if block == nil {
        return "", fmt.Errorf("failed to decode PEM block")
    }

    // Parse certificate
    cert, err := x509.ParseCertificate(block.Bytes)
    if err != nil {
        return "", fmt.Errorf("failed to parse certificate: %w", err)
    }

    // Get public key
    publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
    if !ok {
        return "", fmt.Errorf("certificate does not contain RSA public key")
    }

    // Encrypt initiator password with public key
    encryptedPassword, err := rsa.EncryptPKCS1v15(rand. Reader, publicKey, []byte(initiatorPassword))
    if err != nil {
        return "", fmt.Errorf("failed to encrypt password: %w", err)
    }

    // Encode to base64
    securityCredential := base64.StdEncoding.EncodeToString(encryptedPassword)

    return securityCredential, nil
}