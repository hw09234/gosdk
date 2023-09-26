package parseBlock

import (
	"encoding/asn1"
	"fmt"
	"math"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric-protos-go/orderer/etcdraft"
	"github.com/hyperledger/fabric-protos-go/peer"
	utils "github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

// equal to FABRIC ProviderType in msp/msp.go
const FabricProviderType int32 = 0

type asn1Header struct {
	Number       int64
	PreviousHash []byte
	DataHash     []byte
}

type HashSuite interface {
	// Hash computes Hash value of provided data. Hash function will be different in different crypto implementations.
	Hash(data []byte) []byte
}

func toBytes(b *common.BlockHeader) []byte {
	asn1Header := asn1Header{
		PreviousHash: b.PreviousHash,
		DataHash:     b.DataHash,
	}
	if b.Number > uint64(math.MaxInt64) {
		panic(fmt.Errorf("Golang does not currently support encoding uint64 to asn1"))
	} else {
		asn1Header.Number = int64(b.Number)
	}
	result, err := asn1.Marshal(asn1Header)
	if err != nil {
		panic(err)
	}
	return result
}

// unmarshalAnchorPeer 将byte字段解析为锚节点结构体
func unmarshalAnchorPeer(value []byte) (*peer.AnchorPeers, error) {
	peers := &peer.AnchorPeers{}
	err := proto.Unmarshal(value, peers)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing AnchorPeers")
	}

	return peers, nil
}

// unmarshalMSPConfig 将byte字段解析为MSPConfig结构体
func unmarshalMSPConfig(value []byte) (*msp.MSPConfig, error) {
	mspConfig := &msp.MSPConfig{}
	if err := proto.Unmarshal(value, mspConfig); err != nil {
		return nil, errors.Wrap(err, "failed parsing MSPConfig")
	}

	return mspConfig, nil
}

// unmarshalFabricMSPConfig 将byte字段解析为FabricMSPConfig结构体
func unmarshalFabricMSPConfig(value []byte) (*msp.FabricMSPConfig, error) {
	mspConfig, err := unmarshalMSPConfig(value)
	if err != nil {
		return nil, err
	}

	return getFabricMSPConfig(mspConfig)
}

// getFabricMSPConfig 从MSPConfig结构中获取FabricMSPConfig结构
func getFabricMSPConfig(mspConfig *msp.MSPConfig) (*msp.FabricMSPConfig, error) {
	if mspConfig.Type != FabricProviderType {
		return nil, errors.Errorf("msp config is %d, not fabric type", mspConfig.Type)
	}

	fabricConfig := &msp.FabricMSPConfig{}
	if err := proto.Unmarshal(mspConfig.Config, fabricConfig); err != nil {
		return nil, errors.Wrap(err, "failed marshaling FabricMSPConfig")
	}

	return fabricConfig, nil
}

// unmarshalBatchSize 将byte字段解析为batchSize
func unmarshalBatchSize(value []byte) (*orderer.BatchSize, error) {
	batchSize := new(orderer.BatchSize)
	err := proto.Unmarshal(value, batchSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing BatchSize")
	}
	return batchSize, nil
}

// unmarshalBatchTimeout 将byte字段解析为batchTimeOut
func unmarshalBatchTimeout(value []byte) (*orderer.BatchTimeout, error) {
	batchTimeout := new(orderer.BatchTimeout)
	err := proto.Unmarshal(value, batchTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing BatchSize")
	}
	return batchTimeout, nil
}

// unmarshalChannelRestrictions 将byte字段解析为channelRestrictions
func unmarshalChannelRestrictions(value []byte) (*orderer.ChannelRestrictions, error) {
	channelRestrictions := new(orderer.ChannelRestrictions)
	err := proto.Unmarshal(value, channelRestrictions)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing ChannelRestrictions")
	}
	return channelRestrictions, nil
}

// unmarshalCapabilities 将byte字段解析为capabilities
func unmarshalCapabilities(value []byte) (*common.Capabilities, error) {
	capabilities := new(common.Capabilities)
	err := proto.Unmarshal(value, capabilities)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing Capabilities")
	}
	return capabilities, nil
}

// unmarshalConsensusType 将byte字段解析为consensusType
func unmarshalConsensusType(value []byte) (*orderer.ConsensusType, error) {
	consensusType := new(orderer.ConsensusType)
	err := proto.Unmarshal(value, consensusType)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing ConsensusType")
	}
	return consensusType, nil
}

// unmarshalConfigMetadata 将byte字段解析为ConfigMetadata
func unmarshalConfigMetadata(value []byte) (*etcdraft.ConfigMetadata, error) {
	configMetadata := new(etcdraft.ConfigMetadata)
	err := proto.Unmarshal(value, configMetadata)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing ConfigMetadata")
	}
	return configMetadata, nil
}

// unmarshalEndpoints 将byte字段解析为Endpoints
func unmarshalEndpoints(value []byte) (*common.OrdererAddresses, error) {
	endpoints := new(common.OrdererAddresses)
	err := proto.Unmarshal(value, endpoints)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing Endpoints")
	}
	return endpoints, nil
}

// unmarshalPolicy 将byte字段解析为Policy
func unmarshalPolicy(value []byte) (*common.Policy, error) {
	policy := new(common.Policy)
	err := proto.Unmarshal(value, policy)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing Policy")
	}
	return policy, nil
}

// unmarshalImplicitMetaPolicy 将byte字段解析为ImplicitMetaPolicy
func unmarshalImplicitMetaPolicy(value []byte) (*common.ImplicitMetaPolicy, error) {
	implicitMetaPolicy := new(common.ImplicitMetaPolicy)
	err := proto.Unmarshal(value, implicitMetaPolicy)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing ImplicitMetaPolicy")
	}
	return implicitMetaPolicy, nil
}

// unmarshalSerializedIdentity 将byte字段解析为SerializedIdentity
func unmarshalSerializedIdentity(value []byte) (*msp.SerializedIdentity, error) {
	serializedIdentity := new(msp.SerializedIdentity)
	err := proto.Unmarshal(value, serializedIdentity)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing SerializedIdentity")
	}
	return serializedIdentity, nil
}

func getInputArgs(block *cb.Block) (string, error) {
	envelope, err := utils.GetEnvelopeFromBlock(block.Data.Data[0])
	if err != nil {
		return "", errors.WithMessagef(err, "get envelope from block %d failed", block.Header.Number)
	}
	payload, err := utils.UnmarshalPayload(envelope.Payload)
	if err != nil {
		return "", errors.WithMessagef(err, "get payload from envelope in block %d failed", block.Header.Number)
	}
	chHeader, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return "", errors.WithMessagef(err, "unmarshal channel header from envelope in block %d failed", block.Header.Number)
	}

	if cb.HeaderType(chHeader.Type) != cb.HeaderType_ENDORSER_TRANSACTION {
		return "", errors.New("the type of block header is not transaction")
	}
	transaction := &peer.Transaction{}
	if err := proto.Unmarshal(payload.Data, transaction); err != nil {
		return "", errors.WithMessagef(err, "unmarshal data to transaction from envelope in block %d failed", block.Header.Number)
	}
	chaincodeActionPayload, _, err := utils.GetPayloads(transaction.Actions[0])
	if err != nil {
		return "", errors.WithMessagef(err, "get action payload from envelope in block %d failed", block.Header.Number)
	}
	chaincodeProposalPayload := &peer.ChaincodeProposalPayload{}
	if err := proto.Unmarshal(chaincodeActionPayload.ChaincodeProposalPayload, chaincodeProposalPayload); err != nil {
		return "", errors.WithMessagef(err, "unmarshal chaincode proposal payload from envelope in block %d failed", block.Header.Number)
	}
	chaincodeInvocationSpec := &peer.ChaincodeInvocationSpec{}
	if err := proto.Unmarshal(chaincodeProposalPayload.Input, chaincodeInvocationSpec); err != nil {
		return "", errors.WithMessagef(err, "unmarshal payload input from envelope in block %d failed", block.Header.Number)
	}
	localChaincodeSpec := &ChaincodeSpec{}
	copyChaincodeSpecToLocalChaincodeSpec(localChaincodeSpec, chaincodeInvocationSpec.ChaincodeSpec)

	return localChaincodeSpec.Input.Args[1], nil
}
