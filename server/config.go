package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"

	log "github.com/cihub/seelog"
)

const (
	CONFIGFILE = "broker.json"
)

func LoadConfig() (*Info, error) {
	content, err := ioutil.ReadFile(CONFIGFILE)
	if err != nil {
		log.Error("\tserver/config.go: Read config file error: ", err)
		return nil, err
	}
	var info Info
	err = json.Unmarshal(content, &info)
	if err != nil {
		log.Error("\tserver/config.go: Unmarshal config file error: ", err)
		return nil, err
	}

	if info.TlsPort != "" {
		if info.TlsInfo.CertFile == "" || info.TlsInfo.KeyFile == "" {
			log.Error("\tserver/config.go: tls config error, no cert or key file.")
			return nil, err
		}

		info.TLSConfig, err = NewTLSConfig(info.TlsInfo)
		if err != nil {
			log.Error("\tserver/config.go: new tlsConfig error: ", err)
			return nil, err
		}

		if info.TlsHost == "" {
			info.TlsHost = "0.0.0.0"
		}

	}

	if info.Port != "" {
		if info.Host == "" {
			info.Host = "0.0.0.0"
		}
	}

	if info.Cluster.Port != "" {
		if info.Cluster.Host == "" {
			info.Cluster.Host = "0.0.0.0"
		}
	}

	return &info, nil
}

func NewTLSConfig(tlsInfo TLSInfo) (*tls.Config, error) {

	cert, err := tls.LoadX509KeyPair(tlsInfo.CertFile, tlsInfo.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("error parsing X509 certificate/key pair: %v", err)
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("error parsing certificate: %v", err)
	}

	// Create TLSConfig
	// We will determine the cipher suites that we prefer.
	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Require client certificates as needed
	if tlsInfo.Verify {
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}
	// Add in CAs if applicable.
	if tlsInfo.CaFile != "" {
		rootPEM, err := ioutil.ReadFile(tlsInfo.CaFile)
		if err != nil || rootPEM == nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		ok := pool.AppendCertsFromPEM([]byte(rootPEM))
		if !ok {
			return nil, fmt.Errorf("failed to parse root ca certificate")
		}
		config.ClientCAs = pool
	}

	return &config, nil
}
