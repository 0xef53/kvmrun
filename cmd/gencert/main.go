package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/pkg/kcfg"
)

var (
	Info  = log.New(os.Stdout, "", 0)
	Error = log.New(os.Stderr, "Error: ", 0)
)

func usage() {
	s := "Create server + client certificates signed with the same self-issued CA\n\n"
	s += "Usage:\n"
	s += "  " + filepath.Base(os.Args[0]) + "\n\n"
	s += "Options:\n"
	s += "  -config\n"
	s += "      path to the kvmrun config file\n"
	s += "  -f\n"
	s += "      update existing server + client certificates \n\n"

	fmt.Fprintf(os.Stderr, s)

	os.Exit(2)
}

func main() {
	confFile := "/etc/kvmrun/kvmrun.ini"

	var rewrite bool

	flag.Usage = usage
	flag.StringVar(&confFile, "config", confFile, "")
	flag.BoolVar(&rewrite, "f", rewrite, "")
	flag.Parse()

	KConf, err := kcfg.NewConfig(confFile)
	if err != nil {
		Error.Fatalln(err)
	}

	if err := os.MkdirAll(KConf.Common.CertDir, 0700); err != nil {
		Error.Fatalln(err)
	}

	if err := func() error {
		hosts := []string{
			"127.0.0.1",
			"localhost",
		}

		if h, fqdn, err := getHostname(); err == nil {
			hosts = append(hosts, h, fqdn)
		} else {
			return err
		}

		for _, addr := range KConf.Server.BindAddrs {
			hosts = append(hosts, addr.String())
		}

		if _, err := os.Stat(KConf.Common.ServerCrt); err == nil && !rewrite {
			return fmt.Errorf("Found an existing server certificate: %s", KConf.Common.ServerCrt)
		}

		if _, err := os.Stat(KConf.Common.ClientCrt); err == nil && !rewrite {
			return fmt.Errorf("Found an existing client certificate: %s", KConf.Common.ClientCrt)
		}

		// Check CA
		caCert, caKey, err := func() ([]byte, crypto.PrivateKey, error) {
			if _, err := os.Stat(KConf.Common.CACrt); err == nil {
				fmt.Println("Using an existing CA to generate server + client certificates:", KConf.Common.CACrt)
				return loadCA(KConf.Common.CACrt, KConf.Common.CAKey)
			}
			crt, key, err := generateCA()
			if err != nil {
				return nil, nil, err
			}
			if err := saveCerts(KConf.Common.CACrt, crt); err != nil {
				return nil, nil, err
			}
			if err := saveKey(KConf.Common.CAKey, key); err != nil {
				return nil, nil, err
			}
			return crt, key, nil
		}()

		ca, err := parseCA(caCert)
		if err != nil {
			return err
		}

		// Gen server.crt
		serverCert, serverKey, err := generateCertificate(ca, caKey, hosts)
		if err != nil {
			return err
		}
		if err := saveCerts(KConf.Common.ServerCrt, serverCert, caCert); err != nil {
			return err
		}
		if err := saveKey(KConf.Common.ServerKey, serverKey); err != nil {
			return err
		}
		if rewrite {
			Info.Println("Updated an existing", KConf.Common.ServerCrt)
		} else {
			Info.Println("Generated new", KConf.Common.ServerCrt)
		}

		// Gen client.crt
		clientCert, clientKey, err := generateCertificate(ca, caKey, nil)
		if err != nil {
			return err
		}
		if err := saveCerts(KConf.Common.ClientCrt, clientCert, caCert); err != nil {
			return err
		}
		if err := saveKey(KConf.Common.ClientKey, clientKey); err != nil {
			return err
		}
		if rewrite {
			Info.Println("Updated an existing", KConf.Common.ClientCrt)
		} else {
			Info.Println("Generated new", KConf.Common.ClientCrt)
		}

		return nil
	}(); err != nil {
		Error.Fatalln(err)
	}
}

func loadCA(crtFile, keyFile string) ([]byte, crypto.PrivateKey, error) {
	cert, err := tls.LoadX509KeyPair(crtFile, keyFile)
	if err != nil {
		return nil, nil, err
	}

	if len(cert.Certificate) != 1 {
		return nil, nil, fmt.Errorf("CA.crt should contain only one certificate")
	}

	return cert.Certificate[0], cert.PrivateKey, nil
}

func generateCA() ([]byte, *ecdsa.PrivateKey, error) {
	return generateCertificate(nil, nil, nil)
}

func parseCA(data []byte) (*x509.Certificate, error) {
	certs, err := x509.ParseCertificates(data)
	if err != nil {
		return nil, err
	}
	return certs[0], nil
}

func generateCertificate(signer *x509.Certificate, signerKey interface{}, hosts []string) ([]byte, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Kvmrun Services"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(5 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	switch {
	case signer == nil: // CA
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	case len(hosts) > 0: // server cert
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		for _, h := range hosts {
			if ip := net.ParseIP(h); ip != nil {
				template.IPAddresses = append(template.IPAddresses, ip)
			} else {
				template.DNSNames = append(template.DNSNames, h)
			}
		}
	default: // client cert
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	if signer == nil {
		signer = &template
		signerKey = key
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, signer, &key.PublicKey, signerKey)
	return derBytes, key, err
}

func saveKey(name string, key *ecdsa.PrivateKey) error {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	return savePEM(name, "EC PRIVATE KEY", b)
}

func saveCerts(name string, certs ...[]byte) error {
	return savePEM(name, "CERTIFICATE", certs...)
}

func savePEM(name, header string, certs ...[]byte) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, derBytes := range certs {
		if err := pem.Encode(f, &pem.Block{Type: header, Bytes: derBytes}); err != nil {
			return err
		}
	}
	return f.Close()
}

func getHostname() (string, string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", "", err
	}

	var fqdn string

	addrs, err := net.LookupIP(hostname)
	if err != nil {
		return "", "", err
	}

	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			hosts, err := net.LookupAddr(ipv4.String())
			if err != nil || len(hosts) == 0 {
				return "", "", err
			}

			fqdn = hosts[0]

			break
		}
	}

	// return fqdn without trailing dot
	return hostname, strings.TrimSuffix(fqdn, "."), nil
}
