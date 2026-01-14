package state

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

// Security errors
var (
	ErrAuthFailed      = errors.New("authentication failed")
	ErrIntegrityFailed = errors.New("integrity check failed")
	ErrTLSRequired     = errors.New("TLS required but not configured")
)

// SecurityConfig holds security settings for replication.
type SecurityConfig struct {
	SecretKey   string // PSK for HMAC authentication
	TLSCertFile string // Server certificate file
	TLSKeyFile  string // Server private key file
	TLSCAFile   string // CA certificate for client verification
	TLSMutual   bool   // Require mutual TLS
}

// secureConn wraps a connection with security features.
type secureConn struct {
	net.Conn
	secretKey []byte
}

// newSecureListener creates a TLS listener if configured, otherwise returns a plain listener.
func newSecureListener(addr string, cfg SecurityConfig) (net.Listener, error) {
	if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
		// Plain TCP
		return net.Listen("tcp", addr)
	}

	// Load server certificate
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Load CA for client verification if mTLS is enabled
	if cfg.TLSMutual && cfg.TLSCAFile != "" {
		caCert, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.ClientCAs = caPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tls.Listen("tcp", addr, tlsConfig)
}

// dialSecure connects with TLS if configured, otherwise plain TCP.
func dialSecure(addr string, cfg SecurityConfig, timeout time.Duration) (net.Conn, error) {
	if cfg.TLSCertFile == "" {
		// Plain TCP
		return net.DialTimeout("tcp", addr, timeout)
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load CA for server verification
	if cfg.TLSCAFile != "" {
		caCert, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caPool
	}

	// Load client certificate for mTLS
	if cfg.TLSMutual && cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	dialer := &net.Dialer{Timeout: timeout}
	return tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
}

// PSK Authentication using HMAC challenge-response

// authChallenge is sent by server to client.
type authChallenge struct {
	Nonce string `json:"nonce"` // Random challenge
}

// authResponse is sent by client to server.
type authResponse struct {
	MAC string `json:"mac"` // HMAC-SHA256(nonce, secret_key)
}

// generateNonce creates a random 32-byte nonce.
func generateNonce() (string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return hex.EncodeToString(nonce), nil
}

// computeMAC computes HMAC-SHA256 of the nonce with the secret key.
func computeMAC(nonce string, secretKey []byte) string {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyMAC verifies the HMAC response.
func verifyMAC(nonce, receivedMAC string, secretKey []byte) bool {
	expectedMAC := computeMAC(nonce, secretKey)
	return hmac.Equal([]byte(expectedMAC), []byte(receivedMAC))
}

// Data Integrity

// dataChunk represents a chunk of data with integrity hash.
type dataChunk struct {
	Data     []byte `json:"data"`
	Checksum string `json:"checksum"` // SHA-256 of Data
}

// computeChecksum computes SHA-256 checksum of data.
func computeChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// verifyChecksum verifies the checksum matches the data.
func verifyChecksum(data []byte, checksum string) bool {
	expected := computeChecksum(data)
	return expected == checksum
}

// hashReader wraps a reader and computes hash as data is read.
type hashReader struct {
	reader io.Reader
	hash   []byte
}

func newHashReader(r io.Reader) *hashReader {
	return &hashReader{reader: r}
}

func (h *hashReader) Read(p []byte) (int, error) {
	n, err := h.reader.Read(p)
	if n > 0 {
		hash := sha256.Sum256(p[:n])
		if h.hash == nil {
			h.hash = hash[:]
		} else {
			combined := append(h.hash, hash[:]...)
			newHash := sha256.Sum256(combined)
			h.hash = newHash[:]
		}
	}
	return n, err
}

func (h *hashReader) Hash() string {
	if h.hash == nil {
		return ""
	}
	return hex.EncodeToString(h.hash)
}
