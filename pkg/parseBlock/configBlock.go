package parseBlock

import (
	"github.com/hyperledger/fabric-protos-go/orderer"
)

// ConfigBlock 配置块信息（自定义）
type ConfigBlock struct {
	BaseConfig        BaseConfig        `json:"base_config"`
	ConfigTransaction ConfigTransaction `json:"config_transaction"`
}

type BaseConfig struct {
	Orderer OrdererConfig           `json:"orderer"` //块中的orderer
	Peer    map[string]*Application `json:"peer"`    //块中的peer
}

type OrdererConfig struct {
	BatchSize    *orderer.BatchSize    `json:"batch_size"`              //出块的大小
	BatchTimeOut string                `json:"batch_timeOut"`           //出块的时间
	MaxChannels  uint64                `json:"max_channels"`            //最多允许的通道数
	Consensus    Consensus             `json:"consensus"`               //共识
	KafkaBrokers *orderer.KafkaBrokers `json:"kafka_brokers,omitempty"` //kafka信息（etcdraft类型时则为空）
	Capabilities string                `json:"capabilities"`            //支持的版本
	Msp          map[string]*MspConfig `json:"msp"`                     //块中包含的orderer
}

type Consensus struct {
	ConsensusType string   `json:"consensus_type"`      //共识类型
	Endpoints     []string `json:"endpoints,omitempty"` //共识节点（kafka类型时，无值）
}

type MspConfig struct {
	RootCerts [][]byte `json:"root_certs"` //orderer组织根证书
	Endpoints []string `json:"endpoints"`  //orderer组织下的节点
}

type Application struct {
	RootCerts   [][]byte `json:"root_certs"`             //peer组织根证书
	AnchorPeers []string `json:"anchor_peers,omitempty"` //锚节点（域名：端口）
}

type ConfigTransaction struct {
	Commiter  map[string]*TransactionConfig `json:"commiter"`  //提交者
	Initiator map[string]*TransactionConfig `json:"initiator"` //发起者
	Policy    string                        `json:"policy"`    //更新配置块的权限策略
}

type TransactionConfig struct {
	Signcerts []byte `json:"signcerts"` //执行者的公钥
	Signature []byte `json:"signature"` //使用公钥的签名
}
