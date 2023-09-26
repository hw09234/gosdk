/*
Copyright: peerfintech. All Rights Reserved.
*/

package parseBlock

import (
	"time"

	google_protobuf "github.com/golang/protobuf/ptypes/timestamp"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hw09234/gm-crypto/x509"
)

// Block 整理后的区块结构体
// Config字段只有配置区块才会赋值
type Block struct {
	ChannelID             string              `json:"channel_id"`                         // 通道名称
	Size                  uint64              `json:"size"`                               // 区块大小
	Header                *cb.BlockHeader     `json:"header,omitempty"`                   // 区块头
	BlockHash             []byte              `json:"block_hash"`                         // 区块哈希
	Transactions          []*Transaction      `json:"transactions,omitempty"`             // 区块内交易
	BlockCreatorSignature *SignatureMetadata  `json:"block_creator_signature,omitempty"`  // 区块创建者签名
	LastConfigBlockNumber *LastConfigMetadata `json:"last_config_block_number,omitempty"` // 最新配置区块
	OrdererKafkaMetadata  *OrdererMetadata    `json:"orderer_kafka_metadata,omitempty"`   // kafka offset
	ReceivedTime          time.Time           `json:"block_time_stamp"`                   // 接收区块时间
	MaxTxTime             time.Time           `json:"max_tx_time"`                        // 交易最大时间
	MinTxTime             time.Time           `json:"min_tx_time"`                        // 交易最小时间
	Config                *cb.Config          `json:"config"`                             // 配置区块的内容
	Error                 error               `json:"error"`                              // 错误信息
}

type SignatureMetadata struct {
	SignatureHeader *SignatureHeader `json:"signature_header,omitempty"`
	Signature       []byte           `json:"signature,omitempty"`
}

type LastConfigMetadata struct {
	LastConfigBlockNum uint64             `json:"last_config_block_num,omitempty"`
	SignatureData      *SignatureMetadata `json:"signature_data,omitempty"`
}

type OrdererMetadata struct {
	LastOffsetPersisted uint64             `json:"last_offset_persisted,omitempty"`
	SignatureData       *SignatureMetadata `json:"signature_data,omitempty"`
}

type Transaction struct {
	Signature               []byte             `json:"signature,omitempty"`
	ChannelHeader           *ChannelHeader     `json:"channel_header,omitempty"`
	SignatureHeader         *SignatureHeader   `json:"signature_header,omitempty"`
	TxActionSignatureHeader *SignatureHeader   `json:"tx_action_signature_header,omitempty"`
	ChaincodeSpec           *ChaincodeSpec     `json:"chaincode_spec,omitempty"`
	Endorsements            []*Endorsement     `json:"endorsements,omitempty"`
	ProposalHash            []byte             `json:"proposal_hash,omitempty"`
	Events                  *pb.ChaincodeEvent `json:"events,omitempty"`
	Response                *pb.Response       `json:"response,omitempty"`
	NsRwset                 []*NsReadWriteSet  `json:"ns_read_write_Set,omitempty"`
	// Capture transaction validation code
	ValidationCode     uint8  `json:"validation_code"`
	ValidationCodeName string `json:"validation_code_name,omitempty"`
	Size               uint64 `json:"size"`
}

type ChannelHeader struct {
	Type        int32                      `json:"type,omitempty"`
	Version     int32                      `json:"version,omitempty"`
	Timestamp   *google_protobuf.Timestamp `json:"timestamp,omitempty"`
	ChannelId   string                     `json:"channel_id,omitempty"`
	TxId        string                     `json:"tx_id,omitempty"`
	Epoch       uint64                     `json:"epoch,omitempty"`
	ChaincodeId *pb.ChaincodeID            `json:"chaincode_id,omitempty"`
}
type ChaincodeSpec struct {
	Type        pb.ChaincodeSpec_Type `json:"type,omitempty"`
	ChaincodeId *pb.ChaincodeID       `json:"chaincode_id,omitempty"`
	Input       *ChaincodeInput       `json:"input,omitempty"`
	Timeout     int32                 `json:"timeout,omitempty"`
}

type ChaincodeInput struct {
	Args []string
}

type Endorsement struct {
	SignatureHeader *SignatureHeader `json:"signature_header,omitempty"`
	Signature       []byte           `json:"signature,omitempty"`
}

type SignatureHeader struct {
	Certificate *x509.Certificate
	MspId       string `json:"msp_id,omitempty"`
	Nonce       []byte `json:"nonce,omitempty"`
}

// NsReadWriteSet 读写集
type NsReadWriteSet struct {
	Namespace string           `json:"namespace,omitempty"`
	KVRWSet   *kvrwset.KVRWSet `json:"kVRWSet,omitempty"`
}
