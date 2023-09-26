package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"testing"
	"time"

	pBlock "github.com/hw09234/gosdk/pkg/parseBlock"

	gosdk "github.com/hw09234/gosdk/pkg"
	"github.com/stretchr/testify/require"
)

func TestEventProvider_ListenEventFullBlock(t *testing.T) {
	eventConf := newEventConfig(t, "v1")
	ep, err := gosdk.NewEventClient(eventConf)
	require.NoError(t, err)
	ch := make(chan pBlock.Block)
	errCh := ep.ListenEventFullBlock(2, ch)
	require.NoError(t, err)

	go func() {
		for {
			select {
			case block := <-ch:
				if block.Error != nil {
					t.Errorf("receive Block Error: %s", block.Error.Error())
					return
				} else {
					t.Logf("receive BlockNumber %d", block.Header.Number)
				}
				t.Logf("receive Block %#v", block)
			case err = <-errCh:
				t.Errorf("get full block failed: %v", err)
			}
		}
	}()
	select {}
}

func TestEventProvider_ListenEventFilterBlock(t *testing.T) {
	eventConf := newEventConfig(t, "v1")
	ep, err := gosdk.NewEventClient(eventConf)
	require.NoError(t, err)
	ch := make(chan gosdk.FilteredBlockResponse)
	errCh := ep.ListenEventFilterBlock(2, ch)
	require.NoError(t, err)
	sig := make(chan os.Signal)
	signal.Notify(sig)
	go func() {
		for {
			select {
			case block := <-ch:
				if block.Error != nil {
					t.Errorf("receive Block Error: %s", block.Error.Error())
					return
				} else {
					t.Logf("receive BlockNumber %d", block.BlockNumber)
				}
				t.Logf("receive Block %#v", block)
			case err = <-errCh:
				t.Errorf("get filter block failed: %v", err)
			}
		}
	}()

	select {
	case <-sig:
		t.Log("exit success")
	}
}

func newEventConfig(t *testing.T, version string) gosdk.EventConfig {
	var tls, admin string

	if version == "v2" {
		tls = org1PeerTLSPathV2
		admin = org1AdminPathV2
	} else {
		tls = org1PeerTLSPath
		admin = org1AdminPath
	}

	tlsBytes, err := ioutil.ReadFile(tls)
	require.NoError(t, err)
	peerConf := []gosdk.PeerConfig{
		{
			Name:             "peer0.org1.example.com",
			Host:             "localhost:7051",
			Timeout:          3 * time.Second,
			KeepaliveTime:    10 * time.Second,
			KeepaliveTimeout: 3 * time.Second,
			DomainName:       "peer0.org1.example.com",
			TlsConfig: gosdk.TlsConfig{
				ServerCert: tlsBytes,
			},
			UseTLS: true,
		},
	}
	cert, prikey, err := findCertAndKeyFile(admin)
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
	return gosdk.EventConfig{
		CryptoConfig: cryptoConfig,
		PeerConfigs:  peerConf,
		UserConfig:   userConf,
		CName:        channelName,
	}
}

func newEventConfigGM(t *testing.T, version string) gosdk.EventConfig {
	var tls, admin string

	if version == "v2" {
		tls = org1PeerGMTLSPathV2
		admin = org1AdminGMPathV2
	} else {
		tls = org1PeerGMTLSPath
		admin = org1AdminGMPath
	}

	tlsBytes, err := ioutil.ReadFile(tls)
	require.NoError(t, err)
	peerConf := []gosdk.PeerConfig{
		{
			Name:             "peer0.org1.example.com",
			Host:             "localhost:7051",
			Timeout:          3 * time.Second,
			KeepaliveTime:    10 * time.Second,
			KeepaliveTimeout: 3 * time.Second,
			DomainName:       "peer0.org1.example.com",
			TlsConfig: gosdk.TlsConfig{
				ServerCert: tlsBytes,
			},
			UseTLS: true,
		},
	}
	cert, prikey, err := findCertAndKeyFile(admin)
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
	return gosdk.EventConfig{
		CryptoConfig: gmCryptoConfig,
		PeerConfigs:  peerConf,
		UserConfig:   userConf,
		CName:        channelName,
	}
}

func findCertAndKeyFile(msppath string) (string, string, error) {
	findCert := func(path string) (string, error) {
		list, err := ioutil.ReadDir(path)
		if err != nil {
			return "", err
		}
		var file os.FileInfo
		for _, item := range list {
			if !item.IsDir() {
				if file == nil {
					file = item
				} else if item.ModTime().After(file.ModTime()) {
					file = item
				}
			}
		}
		if file == nil {
			return "", fmt.Errorf("have't file in the %s", path)
		}
		return filepath.Join(path, file.Name()), nil
	}
	prikey, err := findCert(filepath.Join(msppath, "keystore"))
	if err != nil {
		return "", "", err
	}
	cert, err := findCert(filepath.Join(msppath, "signcerts"))
	if err != nil {
		return "", "", err
	}
	return cert, prikey, nil
}

/*
func TestEventClient_ListenEventFullBlock(t *testing.T) {
	clientConfig := newClientConfig(t)
	sdk, err := gosdk.NewFabricClient(&clientConfig)
	require.Nil(t, err, "init sdk failed")
	t.Log("Init sdk success...")

	ch := make(chan pBlock.Block)
	ch, err := sdk.ListenEventFullBlock("mychannel", 2)
	require.NoError(t, err)

	go func() {
		for {
			block := <-ch
			if block.Error != nil {
				t.Errorf("receive Block Error: %s", block.Error.Error())
				return
			} else {
				t.Logf("receive BlockNumber %d", block.Header.Number)
			}
			t.Logf("receive Block %#v", block)
		}
	}()
	select {}
}

func TestEventClient_ListenEventFilterBlock(t *testing.T) {
	clientConfig := newClientConfig(t)
	sdk, err := gosdk.NewFabricClient(&clientConfig)
	require.Nil(t, err, "init sdk failed")
	t.Log("Init sdk success...")

	ch, err := sdk.ListenEventFilterBlock("mychannel", 2)
	require.NoError(t, err)

	go func() {
		for {
			block := <-ch
			if block.Error != nil {
				t.Errorf("receive Block Error: %s", block.Error.Error())
				return
			} else {
				t.Logf("receive BlockNumber %d", block.BlockNumber)
			}
			t.Logf("receive Block %#v", block)
		}
	}()
	select {}
}*/
