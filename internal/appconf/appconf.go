package appconf

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	grpcserver "github.com/0xef53/go-grpc/server"

	"gopkg.in/gcfg.v1"
)

type KvmrunConfig struct {
	QemuRootDir string `gcfg:"qemu-rootdir"`
	CertDir     string `gcfg:"cert-dir"`
}

// Config represents the Kvmrun configuration
type Config struct {
	Kvmrun KvmrunConfig      `gcfg:"common"`
	Server grpcserver.Config `gcfg:"server"`

	TLSConfig *tls.Config `gcfg:"-"`

	ServerCrt string `gcfg:"-"`
	ServerKey string `gcfg:"-"`
	ClientCrt string `gcfg:"-"`
	ClientKey string `gcfg:"-"`
}

// NewConfig reads and parses the configuration file and returns
// a new instance of KvmrunConfig on success.
func newConfig(p string) (*Config, error) {
	cfg := Config{
		Kvmrun: KvmrunConfig{
			QemuRootDir: "/",
			CertDir:     "/usr/share/kvmrun/tls",
		},
		Server: grpcserver.Config{
			Port:           9393,
			GatewayPort:    9898,
			GRPCSocketPath: "/run/kvmrund.sock",
		},
	}

	err := gcfg.ReadFileInto(&cfg, p)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.ClientCrt = filepath.Join(cfg.Kvmrun.CertDir, "client.crt")
	cfg.ClientKey = filepath.Join(cfg.Kvmrun.CertDir, "client.key")

	// Client TLS
	if v, err := tlsConfig(cfg.ClientCrt, cfg.ClientKey); err == nil {
		cfg.TLSConfig = v
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if v := strings.TrimSpace(cfg.Kvmrun.QemuRootDir); len(v) == 0 {
		// Switch to default value
		cfg.Kvmrun.QemuRootDir = "/"
	} else {
		if p, err := filepath.Abs(v); err == nil {
			cfg.Kvmrun.QemuRootDir = p
		} else {
			return nil, err
		}
	}

	return &cfg, nil
}

func NewServerConfig(p string) (*Config, error) {
	cfg, err := newConfig(p)
	if err != nil {
		return nil, err
	}

	cfg.ServerCrt = filepath.Join(cfg.Kvmrun.CertDir, "server.crt")
	cfg.ServerKey = filepath.Join(cfg.Kvmrun.CertDir, "server.key")

	// Server TLS
	if v, err := tlsConfig(cfg.ServerCrt, cfg.ServerKey); err == nil {
		cfg.Server.TLSConfig = v
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	return cfg, nil
}

func NewClientConfig(p string) (*Config, error) {
	return newConfig(p)
}

func tlsConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	if len(cert.Certificate) != 2 {
		return nil, fmt.Errorf("certificate should have 2 concatenated certificates: cert + CA")
	}

	ca, err := x509.ParseCertificate(cert.Certificate[1])
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(ca)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		RootCAs:      certPool,
		ClientCAs:    certPool,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		ClientSessionCache:       tls.NewLRUClientSessionCache(0),
		NextProtos:               []string{"h2", "http/1.1"},
	}, nil
}
