package e2e

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	gohfc "github.com/hw09234/gohfc/pkg"
	"github.com/stretchr/testify/assert"
)

func newLedgerConfig(t *testing.T, version string) gohfc.LedgerConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "crypto-configV2"
	} else {
		cryptoPath = "crypto-config"
	}

	tlscert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.crt", cryptoPath))
	assert.Nil(t, err, "read tls cert failed")

	ucert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts/Admin@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read identity failed")

	ukey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read key failed")

	return gohfc.LedgerConfig{
		CryptoConfig: gohfc.CryptoConfig{
			Family:    "ecdsa",
			Algorithm: "P256-SHA256",
			Hash:      "SHA2-256",
		},
		UserConfig: gohfc.UserConfig{
			Cert:  ucert,
			Key:   ukey,
			MspID: "Org1MSP",
		},
		PeersConfig: []gohfc.PeerConfig{
			{
				Name:             "peer0.org1.example.com",
				Host:             "localhost:7051",
				OrgName:          "org1",
				UseTLS:           true,
				Timeout:          3 * time.Second,
				KeepaliveTime:    10 * time.Second,
				KeepaliveTimeout: 3 * time.Second,
				TlsConfig: gohfc.TlsConfig{
					ServerCert: tlscert,
				},
				DomainName: "peer0.org1.example.com",
				TlsMutual:  false,
			},
		},
	}
}

func newLedgerConfigGM(t *testing.T, version string) gohfc.LedgerConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "gm-crypto-configV2"
	} else {
		cryptoPath = "gm-crypto-config"
	}

	tlscert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.crt", cryptoPath))
	assert.Nil(t, err, "read tls cert failed")

	ucert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts/Admin@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read identity failed")

	ukey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read key failed")

	return gohfc.LedgerConfig{
		CryptoConfig: gmCryptoConfig,
		UserConfig: gohfc.UserConfig{
			Cert:  ucert,
			Key:   ukey,
			MspID: "Org1MSP",
		},
		PeersConfig: []gohfc.PeerConfig{
			{
				Name:             "peer0.org1.example.com",
				Host:             "localhost:7051",
				OrgName:          "org1",
				UseTLS:           true,
				Timeout:          3 * time.Second,
				KeepaliveTime:    10 * time.Second,
				KeepaliveTimeout: 3 * time.Second,
				TlsConfig: gohfc.TlsConfig{
					ServerCert: tlscert,
				},
				DomainName: "peer0.org1.example.com",
				TlsMutual:  false,
			},
		},
	}
}
