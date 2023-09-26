package e2e

import (
	"io/ioutil"
	"testing"
	"time"

	gosdk "github.com/hw09234/gosdk/pkg"
	"github.com/stretchr/testify/require"
)

type adminConfig struct {
	cryptoC   gosdk.CryptoConfig
	peerC     gosdk.PeerConfig
	orderersC []gosdk.OrdererConfig
	userC     gosdk.UserConfig
}

func newOrg1AdminConfig(t *testing.T) adminConfig {
	tlsBytes, err := ioutil.ReadFile(org1PeerTLSPath)
	require.NoError(t, err)
	peerConf := gosdk.PeerConfig{
		Host:             "localhost:7051",
		DomainName:       "peer0.org1.example.com",
		Timeout:          3 * time.Second,
		KeepaliveTime:    10 * time.Second,
		KeepaliveTimeout: 3 * time.Second,
		TlsConfig: gosdk.TlsConfig{
			ServerCert: tlsBytes,
		},
		UseTLS: true,
	}

	orderertlsBytes, err := ioutil.ReadFile(ordererTLSPath)
	require.NoError(t, err)
	orConf := []gosdk.OrdererConfig{
		{
			Host:             "localhost:7050",
			DomainName:       "orderer.example.com",
			Timeout:          3 * time.Second,
			KeepaliveTime:    10 * time.Second,
			KeepaliveTimeout: 3 * time.Second,
			TlsConfig: gosdk.TlsConfig{
				ServerCert: orderertlsBytes,
			},
			UseTLS: true,
		},
	}

	cert, prikey, err := findCertAndKeyFile(org1AdminPath)
	require.NoError(t, err)
	certBytes, err := ioutil.ReadFile(cert)
	require.NoError(t, err)
	keyBytes, err := ioutil.ReadFile(prikey)
	require.NoError(t, err)
	userConf := gosdk.UserConfig{
		Cert:  certBytes,
		Key:   keyBytes,
		MspID: "Org1MSP",
	}
	return adminConfig{
		cryptoC:   cryptoConfig,
		peerC:     peerConf,
		orderersC: orConf,
		userC:     userConf,
	}
}

func newOrg2AdminConfig(t *testing.T) adminConfig {
	tlsBytes, err := ioutil.ReadFile(org2PeerTLSPath)
	require.NoError(t, err)
	peerConf := gosdk.PeerConfig{
		Host:             "localhost:9051",
		DomainName:       "peer0.org2.example.com",
		Timeout:          3 * time.Second,
		KeepaliveTime:    10 * time.Second,
		KeepaliveTimeout: 3 * time.Second,
		TlsConfig: gosdk.TlsConfig{
			ServerCert: tlsBytes,
		},
		UseTLS: true,
	}

	orderertlsBytes, err := ioutil.ReadFile(ordererTLSPath)
	require.NoError(t, err)
	orConf := []gosdk.OrdererConfig{
		{
			Host:             "localhost:7050",
			DomainName:       "orderer.example.com",
			Timeout:          3 * time.Second,
			KeepaliveTime:    10 * time.Second,
			KeepaliveTimeout: 3 * time.Second,
			TlsConfig: gosdk.TlsConfig{
				ServerCert: orderertlsBytes,
			},
			UseTLS: true,
		},
	}

	cert, prikey, err := findCertAndKeyFile(org2AdminPath)
	require.NoError(t, err)
	certBytes, err := ioutil.ReadFile(cert)
	require.NoError(t, err)
	keyBytes, err := ioutil.ReadFile(prikey)
	require.NoError(t, err)
	userConf := gosdk.UserConfig{
		Cert:  certBytes,
		Key:   keyBytes,
		MspID: "Org2MSP",
	}
	return adminConfig{
		cryptoC:   cryptoConfig,
		peerC:     peerConf,
		orderersC: orConf,
		userC:     userConf,
	}
}

func newOrg1AdminConfigGM(t *testing.T) adminConfig {
	tlsBytes, err := ioutil.ReadFile(org1PeerGMTLSPath)
	require.NoError(t, err)
	peerConf := gosdk.PeerConfig{
		Host:             "localhost:7051",
		DomainName:       "peer0.org1.example.com",
		Timeout:          3 * time.Second,
		KeepaliveTime:    10 * time.Second,
		KeepaliveTimeout: 3 * time.Second,
		TlsConfig: gosdk.TlsConfig{
			ServerCert: tlsBytes,
		},
		UseTLS: true,
	}

	orderertlsBytes, err := ioutil.ReadFile(ordererGMTLSPath)
	require.NoError(t, err)
	orConf := []gosdk.OrdererConfig{
		{
			Host:             "localhost:7050",
			DomainName:       "orderer.example.com",
			Timeout:          3 * time.Second,
			KeepaliveTime:    10 * time.Second,
			KeepaliveTimeout: 3 * time.Second,
			TlsConfig: gosdk.TlsConfig{
				ServerCert: orderertlsBytes,
			},
			UseTLS: true,
		},
	}

	cert, prikey, err := findCertAndKeyFile(org1AdminGMPath)
	require.NoError(t, err)
	certBytes, err := ioutil.ReadFile(cert)
	require.NoError(t, err)
	keyBytes, err := ioutil.ReadFile(prikey)
	require.NoError(t, err)
	userConf := gosdk.UserConfig{
		Cert:  certBytes,
		Key:   keyBytes,
		MspID: "Org1MSP",
	}
	return adminConfig{
		cryptoC:   gmCryptoConfig,
		peerC:     peerConf,
		orderersC: orConf,
		userC:     userConf,
	}
}

func newOrg2AdminConfigGM(t *testing.T) adminConfig {
	tlsBytes, err := ioutil.ReadFile(org2PeerGMTLSPath)
	require.NoError(t, err)
	peerConf := gosdk.PeerConfig{
		Host:             "localhost:9051",
		DomainName:       "peer0.org2.example.com",
		Timeout:          3 * time.Second,
		KeepaliveTime:    10 * time.Second,
		KeepaliveTimeout: 3 * time.Second,
		TlsConfig: gosdk.TlsConfig{
			ServerCert: tlsBytes,
		},
		UseTLS: true,
	}

	orderertlsBytes, err := ioutil.ReadFile(ordererGMTLSPath)
	require.NoError(t, err)
	orConf := []gosdk.OrdererConfig{
		{
			Host:             "localhost:7050",
			DomainName:       "orderer.example.com",
			Timeout:          3 * time.Second,
			KeepaliveTime:    10 * time.Second,
			KeepaliveTimeout: 3 * time.Second,
			TlsConfig: gosdk.TlsConfig{
				ServerCert: orderertlsBytes,
			},
			UseTLS: true,
		},
	}

	cert, prikey, err := findCertAndKeyFile(org2AdminGMPath)
	require.NoError(t, err)
	certBytes, err := ioutil.ReadFile(cert)
	require.NoError(t, err)
	keyBytes, err := ioutil.ReadFile(prikey)
	require.NoError(t, err)
	userConf := gosdk.UserConfig{
		Cert:  certBytes,
		Key:   keyBytes,
		MspID: "Org2MSP",
	}
	return adminConfig{
		cryptoC:   gmCryptoConfig,
		peerC:     peerConf,
		orderersC: orConf,
		userC:     userConf,
	}
}
