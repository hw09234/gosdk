/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/hw09234/gm-crypto/tls"
	"github.com/hw09234/gm-crypto/tls/credentials"
	"github.com/hw09234/gm-crypto/x509"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"gopkg.in/yaml.v2"
)

// endorseType在endorse时区分instantiate操作和其它的操作
type endorseType int

const (
	instantiate endorseType = iota
	other
)

const (
	TIMEOUT            = 10 * time.Second // 默认连接超时时间10s
	INSTANTIATETIMEOUT = 3 * time.Minute  // 默认实例化超时时间3min
	KEEPALIVETIME      = 60 * time.Second // 默认grpc keepalive时间间隔60s
	KEEPALIVETIMEOUT   = 30 * time.Second // 默认grpc keepalive超时时间30s
)

// ClientConfig 读取client.yaml配置，构造peer和orderer连接
type ClientConfig struct {
	Crypto     CryptoConfig             `yaml:"Crypto"`
	Orderers   map[string]OrdererConfig `yaml:"Orderers"`
	Peers      map[string]PeerConfig    `yaml:"Peers"`
	EventPeers []PeerConfig             `yaml:"EventPeers"`
	Users      map[string]UserConfig    `yaml:"Users"`
	Channels   []ChannelChaincodeConfig `yaml:"Channels"`
}

// CryptoConfig 确定加密算法
type CryptoConfig struct {
	Family    string `yaml:"Family"`
	Algorithm string `yaml:"Algorithm"`
	Hash      string `yaml:"Hash"`
}

type node struct {
	name               string            // node名称
	orgName            string            // node组织名称
	uri                string            // node ip
	timeout            time.Duration     // 连接node超时时长，默认超时时间10s
	instantiateTimeout time.Duration     // chaincode实例化超时时长，如果设置为3min以下，将使用最小值3min代替
	opts               []grpc.DialOption // grpc连接参数
	tlsCertHash        []byte            // tls证书
	alive              bool
}

// nodeConfig peer和orderer节点通用配置
type nodeConfig struct {
	Name               string        `yaml:"Name"`               // peer名称
	Host               string        `yaml:"Host"`               // peer ip
	OrgName            string        `yaml:"OrgName"`            // peer组织名称
	UseTLS             bool          `yaml:"UseTLS"`             // 是否使用TLS
	Timeout            time.Duration `yaml:"Timeout"`            // 连接超时时间，默认超时时间10s
	InstantiateTimeout time.Duration `yaml:"InstantiateTimeout"` // 实例化超时时间，如果设置为3min以下，将使用最小值3min代替
	DomainName         string        `yaml:"DomainName"`         // 域名
	TlsMutual          bool          `yaml:"TlsMutual"`          // 是否开启双向认证
	KeepaliveTime      time.Duration `yaml:"KeepaliveTime"`      // grpc keepalive时间间隔，如果设置为60s以下，将使用最小值60s代替
	KeepaliveTimeout   time.Duration `yaml:"KeepaliveTimeout"`   // grpc keepalive超时时间，默认超时时间30s
	TlsConfig          `yaml:"TlsConfig"`
}

// OrdererConfig orderer节点
type OrdererConfig = nodeConfig

// PeerConfig peer节点
type PeerConfig = nodeConfig

// TlsConfig tls证书
type TlsConfig struct {
	ServerCertPath string `yaml:"ServercertPath"` // server端tls证书路径
	ServerCert     []byte // server端tls证书
	ClientCertPath string `yaml:"ClientcertPath"` // client端tls证书路径
	ClientCert     []byte // client端tls证书
	ClientKeyPath  string `yaml:"ClientkeyPath"` // client端tls私钥路径
	ClientKey      []byte // client端tls私钥
}

// ChannelConfig 通道及chaincode配置
type ChannelChaincodeConfig struct {
	ChannelId        string          `yaml:"ChannelId"`
	ChaincodeName    string          `yaml:"ChaincodeName"`
	ChaincodeVersion string          `yaml:"ChaincodeVersion"` // 版本信息在invoke和query交易时无需指定，以fabric中最高版本为准
	ChaincodeType    ChainCodeType   `yaml:"ChaincodeType"`    // chaincode语言类型
	ChaincodePolicy  ChaincodePolicy `yaml:"ChaincodePolicy"`
}

// ChaincodePolicy 背书策略
type ChaincodePolicy struct {
	Orgs []string `yaml:"Orgs"`
	Rule string   `yaml:"Rule"`
}

// CAConfig holds config for Fabric CA
type CAConfig struct {
	CryptoConfig
	Uri               string
	SkipTLSValidation bool
	MspId             string
}

// DiscoveryConfig discovery配置，用于接收传递过来的配置信息
type DiscoveryConfig struct {
	CryptoConfig
	PeerConfigs []PeerConfig
	UserConfig
}

// UserConfig 用户msp证书私钥配置
type UserConfig struct {
	CertPath string `yaml:"CertPath"` // msp证书路径
	Cert     []byte // msp证书
	KeyPath  string `yaml:"KeyPath"` // msp私钥路径
	Key      []byte // msp私钥
	MspID    string `yaml:"MspID"`
}

type LedgerConfig struct {
	CryptoConfig
	UserConfig
	PeersConfig []PeerConfig
}

type EventConfig struct {
	CryptoConfig
	UserConfig
	PeerConfigs   []PeerConfig
	CName         string
	RetryTimeout  time.Duration
	RetryInterval time.Duration
	Peers         []*Peer
}

func newNode(nodeConfig nodeConfig, crypto CryptoSuite) (*node, error) {
	if nodeConfig.Timeout == 0 {
		nodeConfig.Timeout = TIMEOUT
	}
	if nodeConfig.InstantiateTimeout < INSTANTIATETIMEOUT {
		nodeConfig.InstantiateTimeout = INSTANTIATETIMEOUT
	}
	n := node{name: nodeConfig.Name, uri: nodeConfig.Host, orgName: nodeConfig.OrgName,
		timeout: nodeConfig.Timeout, instantiateTimeout: nodeConfig.InstantiateTimeout, alive: true}

	if err := n.addGrpcOption(nodeConfig, crypto); err != nil {
		return nil, errors.Errorf("add grpc option failed: %s", err.Error())
	}

	return &n, nil
}

// addGrpcOption 添加grpc连接参数
func (n *node) addGrpcOption(nodeConfig nodeConfig, crypto CryptoSuite) error {
	if nodeConfig.UseTLS {
		if err := n.addTlsOption(nodeConfig, crypto); err != nil {
			return errors.Errorf("add tls option faile: %s", err.Error())
		}
	} else {
		n.opts = []grpc.DialOption{grpc.WithInsecure()}
	}

	if nodeConfig.KeepaliveTime < KEEPALIVETIME {
		nodeConfig.KeepaliveTime = KEEPALIVETIME
	}
	if nodeConfig.KeepaliveTimeout == 0 {
		nodeConfig.KeepaliveTimeout = KEEPALIVETIMEOUT
	}

	n.opts = append(
		n.opts,
		grpc.WithKeepaliveParams(
			keepalive.ClientParameters{
				Time:                nodeConfig.KeepaliveTime,
				Timeout:             nodeConfig.KeepaliveTimeout,
				PermitWithoutStream: true,
			},
		),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxRecvMsgSize),
			grpc.MaxCallSendMsgSize(maxSendMsgSize),
		),
	)

	return nil
}

// addTlsOption 添加tls连接参数
func (n *node) addTlsOption(nodeConfig nodeConfig, crypto CryptoSuite) error {
	var (
		cert tls.Certificate
		err  error
		c    = &tls.Config{
			ServerName: nodeConfig.DomainName,
			MinVersion: tls.VersionTLS12,
		}
	)
	if crypto != nil && strings.ToUpper(crypto.GetFamily()) == "GM" {
		c.GMSupport = &tls.GMSupport{}
	}
	if nodeConfig.TlsMutual {
		cert, err = tls.X509KeyPair(nodeConfig.TlsConfig.ClientCert, nodeConfig.TlsConfig.ClientKey)
		if err != nil {
			return errors.Errorf("failed to construct client key pair: %s", err.Error())
		}
		if crypto != nil {
			n.tlsCertHash = crypto.Hash(cert.Certificate[0])
		}
	}

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(nodeConfig.TlsConfig.ServerCert)
	c.RootCAs = certpool
	c.Certificates = []tls.Certificate{cert}

	n.opts = append(n.opts, grpc.WithTransportCredentials(credentials.NewTLS(c)))

	return nil
}

// NewCAConfig create new Fabric CA config from provided yaml file in path
func NewCAConfig(path string) (*CAConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := new(CAConfig)
	err = yaml.Unmarshal([]byte(data), config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// initConfigFile 从配置文件中读取基本配置
func initConfigFile(file string) (*ClientConfig, error) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil, errors.WithMessagef(err, "%s is not exists", file)
	}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Errorf("read config file failed: %s", err.Error())
	}

	config := new(ClientConfig)
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, errors.Errorf("unmarshal clientConfig from file failed: %s", err.Error())
	}

	err = readCert(config)
	if err != nil {
		return nil, errors.WithMessage(err, "read cert failed")
	}

	return config, nil
}

func readCert(config *ClientConfig) error {
	for i, orderer := range config.Orderers {
		scert, err := ioutil.ReadFile(orderer.ServerCertPath)
		if err != nil {
			return errors.WithMessagef(err, "read orderer server cert %s failed", orderer.ServerCertPath)
		}
		orderer.ServerCert = scert
		config.Orderers[i] = orderer

		if !orderer.TlsMutual {
			continue
		}

		ccert, err := ioutil.ReadFile(orderer.ClientCertPath)
		if err != nil {
			return errors.WithMessagef(err, "read orderer client cert %s failed", orderer.ClientCertPath)
		}
		orderer.ClientCert = ccert

		ckey, err := ioutil.ReadFile(orderer.ClientKeyPath)
		if err != nil {
			return errors.WithMessagef(err, "read orderer client key %s failed", orderer.ClientKeyPath)
		}
		orderer.ClientKey = ckey
		config.Orderers[i] = orderer
	}

	for i, peer := range config.Peers {
		scert, err := ioutil.ReadFile(peer.ServerCertPath)
		if err != nil {
			return errors.WithMessagef(err, "read peer server cert %s failed", peer.ServerCertPath)
		}
		peer.ServerCert = scert
		config.Peers[i] = peer

		if !peer.TlsMutual {
			continue
		}

		ccert, err := ioutil.ReadFile(peer.ClientCertPath)
		if err != nil {
			return errors.WithMessagef(err, "read peer client cert %s failed", peer.ClientCertPath)
		}
		peer.ClientCert = ccert

		ckey, err := ioutil.ReadFile(peer.ClientKeyPath)
		if err != nil {
			return errors.WithMessagef(err, "read peer client key %s failed", peer.ClientKeyPath)
		}
		peer.ClientKey = ckey
		config.Peers[i] = peer
	}

	for i, peer := range config.EventPeers {
		scert, err := ioutil.ReadFile(peer.ServerCertPath)
		if err != nil {
			return errors.WithMessagef(err, "read event cert %s failed", peer.ServerCertPath)
		}
		peer.ServerCert = scert
		config.EventPeers[i] = peer

		if !peer.TlsMutual {
			continue
		}

		ccert, err := ioutil.ReadFile(peer.ClientCertPath)
		if err != nil {
			return errors.WithMessagef(err, "read event client cert %s failed", peer.ClientCertPath)
		}
		peer.ClientCert = ccert

		ckey, err := ioutil.ReadFile(peer.ClientKeyPath)
		if err != nil {
			return errors.WithMessagef(err, "read event client key %s failed", peer.ClientKeyPath)
		}
		peer.ClientKey = ckey
		config.EventPeers[i] = peer
	}

	for i, user := range config.Users {
		cert, err := ioutil.ReadFile(user.CertPath)
		if err != nil {
			return errors.WithMessagef(err, "read user cert %s failed", user.CertPath)
		}
		user.Cert = cert

		key, err := ioutil.ReadFile(user.KeyPath)
		if err != nil {
			return errors.WithMessagef(err, "read user key %s failed", user.KeyPath)
		}
		user.Key = key
		config.Users[i] = user
	}

	return nil
}
