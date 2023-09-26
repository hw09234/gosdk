/*
Copyright: peerfintech. All Rights Reserved.
*/

package parseBlock

import (
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	pbmsp "github.com/hyperledger/fabric-protos-go/msp"
//	ab "github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-protos-go/peer/lifecycle"
	"github.com/hyperledger/fabric/common/flogging"
	utils "github.com/hyperledger/fabric/protoutil"
	"github.com/hw09234/gm-crypto/x509"
	"github.com/pkg/errors"
)

// string定义参考common/channelconfig包
const (
	ApplicationGroupKey    = "Application"
	AnchorPeersKey         = "AnchorPeers"
	OrdererGroupKey        = "Orderer"
	BatchSizeKey           = "BatchSize"
	BatchTimeoutKey        = "BatchTimeout"
	ChannelRestrictionsKey = "ChannelRestrictions"
	CapabilitiesKey        = "Capabilities"
	ConsensusTypeKey       = "ConsensusType"
	AdminsPolicyKey        = "Admins"
	EndpointsKey           = "Endpoints"
	MSPKey                 = "MSP"
)

var logger = flogging.MustGetLogger("parseBlock")

// GetApproveChaincodeInfo 从区块中获取chaincode approved信息
func GetApproveChaincodeInfo(block *cb.Block) (*lifecycle.ApproveChaincodeDefinitionForMyOrgArgs, error) {
	arg, err := getInputArgs(block)
	if err != nil {
		return nil, errors.WithMessage(err, "unmarshal block failed")
	}

	appChaincode := &lifecycle.ApproveChaincodeDefinitionForMyOrgArgs{}
	err = proto.Unmarshal([]byte(arg), appChaincode)
	if err != nil {
		return nil, errors.WithMessage(err, "proto unmarshal approve chaincode failed")
	}

	return appChaincode, nil
}

// GetCommitChaincodeInfo 从区块中获取chaincode committed信息
func GetCommitChaincodeInfo(block *cb.Block) (*lifecycle.CommitChaincodeDefinitionArgs, error) {
	arg, err := getInputArgs(block)
	if err != nil {
		return nil, errors.WithMessage(err, "unmarshal block failed")
	}

	comChaincode := &lifecycle.CommitChaincodeDefinitionArgs{}
	err = proto.Unmarshal([]byte(arg), comChaincode)
	if err != nil {
		return nil, errors.WithMessage(err, "proto unmarshal approve chaincode failed")
	}

	return comChaincode, nil
}

// deserializeIdentity 解析背书签名者的证书信息和msp id
func deserializeIdentity(serializedID []byte) (*x509.Certificate, string, error) {
	sId := &pbmsp.SerializedIdentity{}
	err := proto.Unmarshal(serializedID, sId)
	if err != nil {
		return nil, "", fmt.Errorf("Could not deserialize a SerializedIdentity, err %s", err)
	}

	bl, _ := pem.Decode(sId.IdBytes)
	if bl == nil {
		return nil, "", fmt.Errorf("Could not decode the PEM structure")
	}
	cert, err := x509.ParseCertificate(bl.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("ParseCertificate failed %s", err)
	}
	return cert, sId.Mspid, nil
}

// copyChannelHeaderToLocalChannelHeader 构造自定义区块的channel header
func copyChannelHeaderToLocalChannelHeader(localChannelHeader *ChannelHeader,
	chHeader *cb.ChannelHeader, chaincodeHeaderExtension *peer.ChaincodeHeaderExtension) {
	localChannelHeader.Type = chHeader.Type
	localChannelHeader.Version = chHeader.Version
	localChannelHeader.Timestamp = chHeader.Timestamp
	localChannelHeader.ChannelId = chHeader.ChannelId
	localChannelHeader.TxId = chHeader.TxId
	localChannelHeader.Epoch = chHeader.Epoch
	localChannelHeader.ChaincodeId = chaincodeHeaderExtension.ChaincodeId
}

// copyChaincodeSpecToLocalChaincodeSpec 构造自定义区块的chancode specification，构造chaincode的元数据
func copyChaincodeSpecToLocalChaincodeSpec(localChaincodeSpec *ChaincodeSpec, chaincodeSpec *peer.ChaincodeSpec) {
	localChaincodeSpec.Type = chaincodeSpec.Type
	localChaincodeSpec.ChaincodeId = chaincodeSpec.ChaincodeId
	localChaincodeSpec.Timeout = chaincodeSpec.Timeout
	chaincodeInput := &ChaincodeInput{}
	for _, input := range chaincodeSpec.Input.Args {
		chaincodeInput.Args = append(chaincodeInput.Args, string(input))
	}
	localChaincodeSpec.Input = chaincodeInput
}

// copyEndorsementToLocalEndorsement 构造自定义区块的endorsement信息
func copyEndorsementToLocalEndorsement(localTransaction *Transaction, allEndorsements []*peer.Endorsement) {
	for _, endorser := range allEndorsements {
		endorsement := &Endorsement{}
		endorserSignatureHeader := &cb.SignatureHeader{}

		endorserSignatureHeader.Creator = endorser.Endorser
		endorsement.SignatureHeader = getSignatureHeaderFromBlockData(endorserSignatureHeader)
		endorsement.Signature = endorser.Signature
		localTransaction.Endorsements = append(localTransaction.Endorsements, endorsement)
	}
}

// getValueFromBlockMetadata 获取元数据
func getValueFromBlockMetadata(block *cb.Block, index cb.BlockMetadataIndex) []byte {
	valueMetadata := &cb.Metadata{}
	if index == cb.BlockMetadataIndex_LAST_CONFIG {
		if err := proto.Unmarshal(block.Metadata.Metadata[index], valueMetadata); err != nil {
			return nil
		}

		lastConfig := &cb.LastConfig{}
		if err := proto.Unmarshal(valueMetadata.Value, lastConfig); err != nil {
			return nil
		}
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, lastConfig.Index)
		return b
	//} else if index == cb.BlockMetadataIndex_ORDERER {
	//	if err := proto.Unmarshal(block.Metadata.Metadata[index], valueMetadata); err != nil {
	//		return nil
	//	}
	//
	//	kafkaMetadata := &ab.KafkaMetadata{}
	//	if err := proto.Unmarshal(valueMetadata.Value, kafkaMetadata); err != nil {
	//		return nil
	//	}
	//	b := make([]byte, 8)
	//	binary.LittleEndian.PutUint64(b, uint64(kafkaMetadata.LastOffsetPersisted))
	//	return b
	} else if index == cb.BlockMetadataIndex_TRANSACTIONS_FILTER {
		return block.Metadata.Metadata[index]
	}
	return valueMetadata.Value
}

// getSignatureHeaderFromBlockMetadata 获取signature metadata，包含背书签名者的证书信息和msp id
func getSignatureHeaderFromBlockMetadata(block *cb.Block, index cb.BlockMetadataIndex) (*SignatureMetadata, error) {
	signatureMetadata := &cb.Metadata{}
	if err := proto.Unmarshal(block.Metadata.Metadata[index], signatureMetadata); err != nil {
		return nil, err
	}
	localSignatureHeader := &cb.SignatureHeader{}

	if len(signatureMetadata.Signatures) > 0 {
		if err := proto.Unmarshal(signatureMetadata.Signatures[0].SignatureHeader, localSignatureHeader); err != nil {
			return nil, err
		}

		localSignatureMetadata := &SignatureMetadata{}
		localSignatureMetadata.SignatureHeader = getSignatureHeaderFromBlockData(localSignatureHeader)
		localSignatureMetadata.Signature = signatureMetadata.Signatures[0].Signature

		return localSignatureMetadata, nil
	}
	return nil, nil
}

// getSignatureHeaderFromBlockData 解析背书签名者的证书信息和msp id
func getSignatureHeaderFromBlockData(header *cb.SignatureHeader) *SignatureHeader {
	signatureHeader := &SignatureHeader{}
	signatureHeader.Certificate, signatureHeader.MspId, _ = deserializeIdentity(header.Creator)
	signatureHeader.Nonce = header.Nonce
	return signatureHeader

}

// addTransactionValidation 获取交易验证信息
func addTransactionValidation(tran *Transaction, txIdx int, transactionFilter []uint8) error {
	if len(transactionFilter) > txIdx {
		tran.ValidationCode = transactionFilter[txIdx]
		tran.ValidationCodeName = peer.TxValidationCode_name[int32(tran.ValidationCode)]
		return nil
	}
	return fmt.Errorf("invalid index or transaction filler. Index: %d", txIdx)
}

// newTxValidationFlagsSetValue
// copied from core/ledger/util/txvalidationflags.go
func newTxValidationFlagsSetValue(size int, value peer.TxValidationCode) []uint8 {
	inst := make([]uint8, size)
	for i := range inst {
		inst[i] = uint8(value)
	}

	return inst
}

// ParseBlock 解析区块
// input: blockEvent: 获得的区块
//        size: 数据大小
// output: Block: 自定义区块
func ParseBlock(block *cb.Block, size uint64, hash HashSuite) Block {
	var localBlock Block

	localBlock.Size = size
	localBlock.ReceivedTime = time.Now().Local()
	localBlock.BlockHash = hash.Hash(toBytes(block.Header))
	localBlock.Header = block.Header
	transactionFilter := newTxValidationFlagsSetValue(len(block.Data.Data), peer.TxValidationCode_NOT_VALIDATED)

	// process block metadata before data
	localBlock.BlockCreatorSignature, _ = getSignatureHeaderFromBlockMetadata(block, cb.BlockMetadataIndex_SIGNATURES)
	lastConfigBlockNumber := &LastConfigMetadata{}
	lastConfigBlockNumber.LastConfigBlockNum = binary.LittleEndian.Uint64(getValueFromBlockMetadata(block, cb.BlockMetadataIndex_LAST_CONFIG))
	lastConfigBlockNumber.SignatureData, _ = getSignatureHeaderFromBlockMetadata(block, cb.BlockMetadataIndex_LAST_CONFIG)
	localBlock.LastConfigBlockNumber = lastConfigBlockNumber

	txBytes := getValueFromBlockMetadata(block, cb.BlockMetadataIndex_TRANSACTIONS_FILTER)
	for index, b := range txBytes {
		transactionFilter[index] = b
	}

	ordererKafkaMetadata := &OrdererMetadata{}
	ordererKafkaMetadata.LastOffsetPersisted = binary.BigEndian.Uint64(getValueFromBlockMetadata(block, cb.BlockMetadataIndex_ORDERER))
	ordererKafkaMetadata.SignatureData, _ = getSignatureHeaderFromBlockMetadata(block, cb.BlockMetadataIndex_ORDERER)
	localBlock.OrdererKafkaMetadata = ordererKafkaMetadata

	for txIndex, data := range block.Data.Data {
		localTransaction := &Transaction{}
		localTransaction.Size = uint64(len(data))
		//Get envelope which is stored as byte array in the data field.
		envelope, err := utils.GetEnvelopeFromBlock(data)
		if err != nil {
			logger.Errorf("get envelope %d from block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
			continue
		}
		localTransaction.Signature = envelope.Signature
		//Get payload from envelope struct which is stored as byte array.
		payload, err := utils.UnmarshalPayload(envelope.Payload)
		if err != nil {
			logger.Errorf("get payload from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
			continue
		}
		chHeader, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			logger.Errorf("unmarshal channel header from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
			continue
		}
		headerExtension := &peer.ChaincodeHeaderExtension{}
		if err := proto.Unmarshal(chHeader.Extension, headerExtension); err != nil {
			logger.Errorf("unmarshal channel header extension from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
			continue
		}
		localChannelHeader := &ChannelHeader{}
		copyChannelHeaderToLocalChannelHeader(localChannelHeader, chHeader, headerExtension)

		localBlock.ChannelID = localChannelHeader.ChannelId
		txTime := time.Unix(localChannelHeader.Timestamp.Seconds, int64(localChannelHeader.Timestamp.Nanos)).Local()
		if txIndex == 0 || txTime.Before(localBlock.MinTxTime) {
			localBlock.MinTxTime = txTime
		}
		if txTime.After(localBlock.MaxTxTime) {
			localBlock.MaxTxTime = txTime
		}

		// Performance measurement code ends
		localTransaction.ChannelHeader = localChannelHeader
		localSignatureHeader := &cb.SignatureHeader{}
		if err := proto.Unmarshal(payload.Header.SignatureHeader, localSignatureHeader); err != nil {
			logger.Errorf("unmarshal channel header signature from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
			continue
		}
		localTransaction.SignatureHeader = getSignatureHeaderFromBlockData(localSignatureHeader)
		//localTransaction.SignatureHeader.Nonce = localSignatureHeader.Nonce
		//localTransaction.SignatureHeader.Certificate, _ = deserializeIdentity(localSignatureHeader.Creator)

		if cb.HeaderType(chHeader.Type) == cb.HeaderType_ENDORSER_TRANSACTION {
			transaction := &peer.Transaction{}
			if err := proto.Unmarshal(payload.Data, transaction); err != nil {
				logger.Errorf("unmarshal data to transaction from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}
			chaincodeActionPayload, chaincodeAction, err := utils.GetPayloads(transaction.Actions[0])
			if err != nil {
				logger.Errorf("get action payload from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}
			localSignatureHeader = &cb.SignatureHeader{}
			if err := proto.Unmarshal(transaction.Actions[0].Header, localSignatureHeader); err != nil {
				logger.Errorf("unmarshal action header from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}
			localTransaction.TxActionSignatureHeader = getSignatureHeaderFromBlockData(localSignatureHeader)
			//signatureHeader = &SignatureHeader{}
			//signatureHeader.Certificate, _ = deserializeIdentity(localSignatureHeader.Creator)
			//signatureHeader.Nonce = localSignatureHeader.Nonce
			//localTransaction.TxActionSignatureHeader = signatureHeader

			chaincodeProposalPayload := &peer.ChaincodeProposalPayload{}
			if err := proto.Unmarshal(chaincodeActionPayload.ChaincodeProposalPayload, chaincodeProposalPayload); err != nil {
				logger.Errorf("unmarshal chaincode proposal payload from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}
			chaincodeInvocationSpec := &peer.ChaincodeInvocationSpec{}
			if err := proto.Unmarshal(chaincodeProposalPayload.Input, chaincodeInvocationSpec); err != nil {
				logger.Errorf("unmarshal payload input from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}
			localChaincodeSpec := &ChaincodeSpec{}
			copyChaincodeSpecToLocalChaincodeSpec(localChaincodeSpec, chaincodeInvocationSpec.ChaincodeSpec)
			localTransaction.ChaincodeSpec = localChaincodeSpec
			copyEndorsementToLocalEndorsement(localTransaction, chaincodeActionPayload.Action.Endorsements)
			proposalResponsePayload := &peer.ProposalResponsePayload{}
			if err := proto.Unmarshal(chaincodeActionPayload.Action.ProposalResponsePayload, proposalResponsePayload); err != nil {
				logger.Errorf("unmarshal proposal response payload from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}
			localTransaction.ProposalHash = proposalResponsePayload.ProposalHash
			localTransaction.Response = chaincodeAction.Response
			events := &peer.ChaincodeEvent{}
			if err := proto.Unmarshal(chaincodeAction.Events, events); err != nil {
				logger.Errorf("unmarshal action events from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}
			localTransaction.Events = events

			txReadWriteSet := &rwset.TxReadWriteSet{}
			if err := proto.Unmarshal(chaincodeAction.Results, txReadWriteSet); err != nil {
				logger.Errorf("unmarshal action to read and write set from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}

			if len(chaincodeAction.Results) != 0 {
				for _, nsRwset := range txReadWriteSet.NsRwset {
					nsReadWriteSet := &NsReadWriteSet{}
					kvRWSet := &kvrwset.KVRWSet{}
					nsReadWriteSet.Namespace = nsRwset.Namespace
					if err := proto.Unmarshal(nsRwset.Rwset, kvRWSet); err != nil {
						continue
					}
					nsReadWriteSet.KVRWSet = kvRWSet
					localTransaction.NsRwset = append(localTransaction.NsRwset, nsReadWriteSet)
				}
			}

			// add the transaction validation a
			addTransactionValidation(localTransaction, txIndex, transactionFilter)
			//append the transaction
			localBlock.Transactions = append(localBlock.Transactions, localTransaction)
		} else if cb.HeaderType(chHeader.Type) == cb.HeaderType_CONFIG {
			configEnv := &cb.ConfigEnvelope{}
			_, err = utils.UnmarshalEnvelopeOfType(envelope, cb.HeaderType_CONFIG, configEnv)
			if err != nil {
				logger.Errorf("unmarshal envelope %d to config type in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}

			// TODO 测试DeepMarshalJSON对不同fabric版本的支持性
			//buf := &bytes.Buffer{}
			//if err := protolator.DeepMarshalJSON(buf, configEnv.Config); err != nil {
			//	fmt.Printf("Bad DeepMarshalJSON Buffer : %s\n", err)
			//	continue
			//}
			//localBlock.Config = buf.String()
			localBlock.Config = configEnv.Config

			payload, err := utils.UnmarshalPayload(configEnv.LastUpdate.Payload)
			if err != nil {
				logger.Errorf("get payload from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}
			chHeader, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
			if err != nil {
				logger.Errorf("unmarshal channel header from envelope %d in block %d failed: %s", txIndex, localBlock.Header.Number, err.Error())
				continue
			}

			if len(localTransaction.ProposalHash) == 0 {
				localTransaction.ProposalHash = hash.Hash(configEnv.LastUpdate.Payload)
			}

			localChannelHeader := &ChannelHeader{}
			copyChannelHeaderToLocalChannelHeader(localChannelHeader, chHeader, headerExtension)

			localTransaction.TxActionSignatureHeader = localTransaction.SignatureHeader

			// add the transaction validation
			addTransactionValidation(localTransaction, txIndex, transactionFilter)
			// append the transaction
			localBlock.Transactions = append(localBlock.Transactions, localTransaction)
		}
	}

	return localBlock
}

// GetAnchorPeersFromBlock 获取锚节点
// input: block: 区块
// output: map[string][]*peer.AnchorPeer: 锚节点信息
func GetAnchorPeersFromBlock(block *cb.Block) (map[string][]*peer.AnchorPeer, error) {
	orgAnchorPeers := make(map[string][]*peer.AnchorPeer)

	if block == nil {
		return orgAnchorPeers, errors.New("block is nil")
	}

	if !utils.IsConfigBlock(block) {
		return orgAnchorPeers, errors.New("the block is not a config block")
	}

	envelope, err := utils.ExtractEnvelope(block, 0)
	if err != nil {
		return orgAnchorPeers, errors.New("can not extract the envelope info")
	}

	configEnv := &cb.ConfigEnvelope{}
	_, err = utils.UnmarshalEnvelopeOfType(envelope, cb.HeaderType_CONFIG, configEnv)
	if err != nil {
		return orgAnchorPeers, errors.New("can not unmarshal envelope to config envelope")
	}

	appGroup := configEnv.Config.ChannelGroup.Groups[ApplicationGroupKey]
	for key, conGroupValue := range appGroup.Groups {
		anchorValue := conGroupValue.Values[AnchorPeersKey]
		anchorPeers := &peer.AnchorPeers{}
		if anchorValue == nil {
			continue
		}
		err = proto.Unmarshal(anchorValue.Value, anchorPeers)
		if err != nil {
			continue
		}
		orgAnchorPeers[key] = anchorPeers.AnchorPeers
	}

	return orgAnchorPeers, nil
}

// GetConfigBlockFromBlock 获取配置块信息
// input: block: 区块
// output: ConfigBlock: 解析的配置块信息
func GetConfigBlockFromBlock(block *cb.Block) (*ConfigBlock, error) {
	var configBlock ConfigBlock
	if block == nil {
		return &configBlock, errors.New("block is nil")
	}

	if !utils.IsConfigBlock(block) {
		return &configBlock, errors.New("the block is not a config block")
	}

	envelope, err := utils.ExtractEnvelope(block, 0)
	if err != nil {
		return &configBlock, errors.New("can not extract the envelope info")
	}

	configEnv := &cb.ConfigEnvelope{}
	_, err = utils.UnmarshalEnvelopeOfType(envelope, cb.HeaderType_CONFIG, configEnv)
	if err != nil {
		return &configBlock, errors.New("can not unmarshal envelope to config envelope")
	}

	batchSize, err := unmarshalBatchSize(configEnv.Config.ChannelGroup.Groups[OrdererGroupKey].Values[BatchSizeKey].Value)
	if err != nil {
		return &configBlock, errors.New("can not unmarshal batchSize to configBlock batchSize")
	}
	configBlock.BaseConfig.Orderer.BatchSize = batchSize

	batchTimeOut, err := unmarshalBatchTimeout(configEnv.Config.ChannelGroup.Groups[OrdererGroupKey].Values[BatchTimeoutKey].Value)
	if err != nil {
		return &configBlock, errors.New("can not unmarshal batchTimeOut to configBlock batchTimeOut")
	}
	configBlock.BaseConfig.Orderer.BatchTimeOut = batchTimeOut.Timeout

	if configEnv.Config.ChannelGroup.Groups[OrdererGroupKey].Values[ChannelRestrictionsKey].Value != nil {
		maxChannels, err := unmarshalChannelRestrictions(configEnv.Config.ChannelGroup.Groups[OrdererGroupKey].Values[ChannelRestrictionsKey].Value)
		if err != nil {
			return &configBlock, errors.New("can not unmarshal channelRestrictions to configBlock channelRestrictions")
		}
		configBlock.BaseConfig.Orderer.MaxChannels = maxChannels.MaxCount
	}
	configBlock.BaseConfig.Orderer.MaxChannels = 0

	capabilities, err := unmarshalCapabilities(configEnv.Config.ChannelGroup.Groups[OrdererGroupKey].Values[CapabilitiesKey].Value)
	if err != nil {
		return &configBlock, errors.New("can not unmarshal capabilities to configBlock capabilities")
	}
	for key := range capabilities.Capabilities {
		configBlock.BaseConfig.Orderer.Capabilities = key
	}
	consensusType, err := unmarshalConsensusType(configEnv.Config.ChannelGroup.Groups[OrdererGroupKey].Values[ConsensusTypeKey].Value)
	if err != nil {
		return &configBlock, errors.New("can not unmarshal consensusType to configBlock consensusType")
	}
	configBlock.BaseConfig.Orderer.Consensus.ConsensusType = consensusType.Type

	configMetadata, err := unmarshalConfigMetadata(consensusType.Metadata)
	if err != nil {
		return &configBlock, errors.New("can not unmarshal configMetadata to configBlock configMetadata")
	}
	for _, consenter := range configMetadata.Consenters {
		configBlock.BaseConfig.Orderer.Consensus.Endpoints = append(configBlock.BaseConfig.Orderer.Consensus.Endpoints, consenter.Host)
	}

	configBlock.BaseConfig.Orderer.Msp = getMspConfigFromConfigEnvelope(configEnv)
	configBlock.BaseConfig.Peer = getApplicationFromConfigEnvelope(configEnv)

	policy, err := unmarshalPolicy(configEnv.Config.ChannelGroup.Policies[AdminsPolicyKey].Policy.Value)
	if err != nil {
		return &configBlock, errors.New("can not unmarshal policy to configBlock policy")
	}
	implicitMetaPolicy, err := unmarshalImplicitMetaPolicy(policy.Value)
	if err != nil {
		return &configBlock, errors.New("can not unmarshal implicitMetaPolicy to configBlock implicitMetaPolicy")
	}
	configBlock.ConfigTransaction.Policy = implicitMetaPolicy.Rule.String()

	initiator, err := getInitiatorFromConfigEnvelope(configEnv)
	if err != nil {
		return &configBlock, err
	}
	configBlock.ConfigTransaction.Initiator = initiator

	commiter, err := getCommiterFromConfigEnvelope(envelope)
	if err != nil {
		return &configBlock, err
	}
	configBlock.ConfigTransaction.Commiter = commiter
	return &configBlock, nil
}

// getMspConfigFromConfigEnvelope 获取orderer组织信息
// input: ConfigEnvelope: 配置块的env结构
// output: map[string]*MspConfig: orderer组织信息
func getMspConfigFromConfigEnvelope(configEnv *cb.ConfigEnvelope) map[string]*MspConfig {
	ordererMSPConfig := make(map[string]*MspConfig)
	for _, value := range configEnv.Config.ChannelGroup.Groups[OrdererGroupKey].Groups {
		endpoints, err := unmarshalEndpoints(value.Values[EndpointsKey].Value)
		if err != nil {
			continue
		}
		msp, err := unmarshalFabricMSPConfig(value.Values[MSPKey].Value)
		if err != nil {
			continue
		}
		ordererMsp := &MspConfig{
			RootCerts: msp.RootCerts,
			Endpoints: endpoints.Addresses,
		}
		ordererMSPConfig[msp.Name] = ordererMsp
	}
	return ordererMSPConfig
}

// getApplicationFromConfigEnvelope 获取peer组织信息
// input: ConfigEnvelope: 配置块的env结构
// output: map[string]*Application: peer组织信息
func getApplicationFromConfigEnvelope(configEnv *cb.ConfigEnvelope) map[string]*Application {
	application := make(map[string]*Application)
	for _, value := range configEnv.Config.ChannelGroup.Groups[ApplicationGroupKey].Groups {
		var anchors []string
		if value.Values[AnchorPeersKey] != nil {
			anchorPeers, err := unmarshalAnchorPeer(value.Values[AnchorPeersKey].Value)
			if err != nil {
				continue
			}
			for _, anchor := range anchorPeers.AnchorPeers {
				anchorInfo := fmt.Sprintf("%s:%v", anchor.Host, anchor.Port)
				anchors = append(anchors, anchorInfo)
			}
		}
		applicationMsp, err := unmarshalFabricMSPConfig(value.Values[MSPKey].Value)
		if err != nil {
			continue
		}
		simpleApplication := &Application{
			RootCerts:   applicationMsp.RootCerts,
			AnchorPeers: anchors,
		}
		application[applicationMsp.Name] = simpleApplication
	}
	return application
}

// getInitiatorFromConfigEnvelope 获取交易发起者信息
// input: ConfigEnvelope: 配置块的env结构
// output: map[string]*TransactionConfig: 交易发起者信息
func getInitiatorFromConfigEnvelope(configEnv *cb.ConfigEnvelope) (map[string]*TransactionConfig, error) {
	initiator := make(map[string]*TransactionConfig)
	lastUpdatePayload, err := utils.UnmarshalPayload(configEnv.LastUpdate.Payload)
	if err != nil {
		return initiator, errors.New("can not unmarshal lastUpdatePayload to configBlock lastUpdatePayload")
	}
	initiatorSignatureHeader, err := utils.UnmarshalSignatureHeader(lastUpdatePayload.Header.SignatureHeader)
	if err != nil {
		return initiator, errors.New("can not unmarshal signatureHeader to configBlock signatureHeader")
	}
	serializedIdentity, err := unmarshalSerializedIdentity(initiatorSignatureHeader.Creator)
	if err != nil {
		return initiator, errors.New("can not unmarshal serializedIdentity to configBlock serializedIdentity")
	}
	initiatorConfig := &TransactionConfig{
		Signcerts: serializedIdentity.IdBytes,
		Signature: configEnv.LastUpdate.Signature,
	}
	initiator[serializedIdentity.Mspid] = initiatorConfig
	return initiator, nil
}

// getCommiterFromConfigEnvelope 获取交易提交者信息
// input: Envelope: 区块的env结构
// output: map[string]*TransactionConfig: 交易提交者信息
func getCommiterFromConfigEnvelope(envelope *cb.Envelope) (map[string]*TransactionConfig, error) {
	commiter := make(map[string]*TransactionConfig)
	payload, err := utils.UnmarshalPayload(envelope.Payload)
	if err != nil {
		return commiter, errors.New("can not unmarshal envelope.Payload to payload")
	}
	commitSignatureHeader, err := utils.UnmarshalSignatureHeader(payload.Header.SignatureHeader)
	if err != nil {
		return commiter, errors.New("can not unmarshal signatureHeader to configBlock signatureHeader")
	}
	creator, err := unmarshalSerializedIdentity(commitSignatureHeader.Creator)
	if err != nil {
		return commiter, errors.New("can not unmarshal creator to configBlock creator")
	}
	commiterConfig := &TransactionConfig{
		Signcerts: creator.IdBytes,
		Signature: envelope.Signature,
	}
	commiter[creator.Mspid] = commiterConfig
	return commiter, nil
}
