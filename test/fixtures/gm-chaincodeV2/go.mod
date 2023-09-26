module github.com/PeerFintech/gosdk/test/fixtures/gm-chaincode

go 1.14

require (
	github.com/hyperledger/fabric-chaincode-go v0.0.0-20210319203922-6b661064d4d9
	github.com/hyperledger/fabric-protos-go v0.0.0-20210505131505-0ac7fd605762
)

replace github.com/hyperledger/fabric-chaincode-go => gitlab.peersafe.cn/fabric/fabric-chaincode-go v1.0.1

replace github.com/peersafe/gm-crypto => gitlab.peersafe.cn/fabric/gm-crypto v1.0.1
