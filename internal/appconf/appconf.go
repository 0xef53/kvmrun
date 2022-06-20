package appconf

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xef53/kvmrun/internal/grpcserver"

	"gopkg.in/gcfg.v1"
)

type CommonParams struct {
	CertDir   string      `gcfg:"cert-dir"`
	CACrt     string      `gcfg:"-"`
	CAKey     string      `gcfg:"-"`
	ServerCrt string      `gcfg:"-"`
	ServerKey string      `gcfg:"-"`
	ClientCrt string      `gcfg:"-"`
	ClientKey string      `gcfg:"-"`
	TLSConfig *tls.Config `gcfg:"-"`
}

// KvmrunConfig represents the Kvmrun configuration
type KvmrunConfig struct {
	Common CommonParams
	Server grpcserver.ServerConf
}

// NewConfig reads and parses the configuration file and returns
// a new instance of KvmrunConfig on success.
func NewConfig(p string) (*KvmrunConfig, error) {
	cfg := KvmrunConfig{
		Common: CommonParams{
			CertDir: "/usr/share/kvmrun/tls",
		},
		Server: grpcserver.ServerConf{
			BindSocket: "/run/kvmrund.sock",
		},
	}

	err := gcfg.ReadFileInto(&cfg, p)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %s", err)
	}

	cfg.Common.CACrt = filepath.Join(cfg.Common.CertDir, "CA.crt")
	cfg.Common.CAKey = filepath.Join(cfg.Common.CertDir, "CA.key")
	cfg.Common.ServerCrt = filepath.Join(cfg.Common.CertDir, "server.crt")
	cfg.Common.ServerKey = filepath.Join(cfg.Common.CertDir, "server.key")
	cfg.Common.ClientCrt = filepath.Join(cfg.Common.CertDir, "client.crt")
	cfg.Common.ClientKey = filepath.Join(cfg.Common.CertDir, "client.key")

	// Client TLS
	if v, err := tlsConfig(cfg.Common.ClientCrt, cfg.Common.ClientKey); err == nil {
		cfg.Common.TLSConfig = v
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Server TLS
	if v, err := tlsConfig(cfg.Common.ServerCrt, cfg.Common.ServerKey); err == nil {
		cfg.Server.TLSConfig = v
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	return &cfg, nil
}

func tlsConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	if len(cert.Certificate) != 2 {
		return nil, fmt.Errorf("certificate should have 2 concatenated certificates: server + CA")
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
