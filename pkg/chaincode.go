/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/common/policydsl"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

type ChainCodeType int32

const (
	ChaincodeSpec_UNDEFINED ChainCodeType = 0
	ChaincodeSpec_GOLANG    ChainCodeType = 1
	ChaincodeSpec_NODE      ChainCodeType = 2
	ChaincodeSpec_CAR       ChainCodeType = 3
	ChaincodeSpec_JAVA      ChainCodeType = 4
)

const (
	LSCC          = "lscc"
	QSCC          = "qscc"
	CSCC          = "cscc"
	lifecycleName = "_lifecycle"
)

func (c ChainCodeType) string() string {
	switch c {
	case ChaincodeSpec_GOLANG:
		return "golang"
	case ChaincodeSpec_JAVA:
		return "java"
	case ChaincodeSpec_NODE:
		return "node"
	}

	return ""
}

// ChainCode 在chaincode上操作所需字段
type ChainCode struct {
	ChannelId    string        //通道名称
	Name         string        // chaincode名称
	Version      string        // chaincode版本
	Type         ChainCodeType // chaincode语言
	IsInit       bool          // fabricV2.2版本参数
	Args         []string
	argBytes     []byte
	TransientMap map[string][]byte
	rawArgs      [][]byte
}

type ApproveChaincode struct {
	Sequence          int64
	Version           string
	InitRequired      bool
	PackageID         string
	EndorsementPlugin string
	ValidationPlugin  string
}

func (c *ChainCode) toChainCodeArgs() [][]byte {
	if len(c.rawArgs) > 0 {
		return c.rawArgs
	}
	args := make([][]byte, len(c.Args))
	for i, arg := range c.Args {
		args[i] = []byte(arg)
	}
	if len(c.argBytes) > 0 {
		args = append(args, c.argBytes)
	}
	return args
}

type ChaincodeInfo struct {
	OutputFile string        // 打包写入文件
	Path       string        // chaincode路径
	Type       ChainCodeType // chaincode语言类型
	Label      string        // chaincode标签
}

type PackageMetadata struct {
	Path  string `json:"path"`  // chaincode路径
	Type  string `json:"type"`  // chaincode语言类型
	Label string `json:"label"` // chaincode标签
}

// InstallRequest holds fields needed to install chaincode
type InstallRequest struct {
	ChannelId        string
	ChainCodeName    string
	ChainCodeVersion string
	ChainCodeType    ChainCodeType
	Namespace        string
	SrcPath          string
	Libraries        []ChaincodeLibrary
}

type ChaincodeLibrary struct {
	Namespace string
	SrcPath   string
}

type ApproveCommitRequest struct {
	ChannelName         string // 通道名称
	ChaincodeName       string // chaincode名称
	ChaincodeVserison   string // chaincode版本
	PackageID           string // 安装完成后的chaincode包id
	SignaturePolicy     string // 自定义背书cel
	ChannelConfigPolicy string // 通道配置策略
	OrgName             string // 组织名称
	Sequence            int64  // chaincode序列号
	InitReqired         bool   //是否要执行init
}

type CheckCommitreadinessRequest struct {
	ChannelName       string // 通道名称
	ChaincodeName     string // chaincode名称
	ChaincodeVserison string // chaincode版本
	SignaturePolicy   string // 自定义背书cel
	InitRequired      bool   //是否要执行init
	Sequence          int64  // chaincode序列号
}

type CommittedQueryInput struct {
	ChannelID     string // 通道名称
	ChaincodeName string // chaincode名称
}

type CollectionConfig struct {
	Name               string
	RequiredPeersCount int32
	MaximumPeersCount  int32
	Organizations      []string
}

// ChainCodesResponse 查询安装或实例化chaincode结果
type ChainCodesResponse struct {
	PeerName   string
	Error      error
	ChainCodes []*peer.ChaincodeInfo
}

// validate 检查chaincode信息
func (info ChaincodeInfo) validate() error {
	var LabelRegexp = regexp.MustCompile(`^[[:alnum:]][[:alnum:]_.+-]*$`)

	if info.Path == "" {
		return errors.New("chaincode path must be specified")
	}
	if result := info.Type.string(); result == "" {
		return errors.New("chaincode language must be specified")
	}
	if info.OutputFile == "" {
		return errors.New("output file must be specified")
	}
	if info.Label == "" {
		return errors.New("package label must be specified")
	}
	if !LabelRegexp.MatchString(info.Label) {
		return errors.Errorf("invalid label '%s'. Label must be non-empty, can only consist of alphanumerics, symbols from '.+-_', and can only begin with alphanumerics", info.Label)
	}

	return nil
}

// validate 校验check commit的请求信息
func (r CheckCommitreadinessRequest) validate() error {
	if r.ChaincodeName == "" {
		return errors.New("chaincode name must be specified")
	}

	if r.ChannelName == "" {
		return errors.New("channel name must be specified")
	}

	if r.ChaincodeVserison == "" {
		return errors.New("chaincode version must be specified")
	}

	if r.Sequence == 0 {
		return errors.New("sequence must be specified")
	}

	return nil
}

// validate 检查approve信息
func (a ApproveCommitRequest) validate() error {
	if a.ChaincodeName == "" {
		return errors.New("chaincode name must be specified")
	}

	if a.ChannelName == "" {
		return errors.New("channel name must be specified")
	}

	if a.ChaincodeVserison == "" {
		return errors.New("chaincode version must be specified")
	}

	if a.Sequence == 0 {
		return errors.New("sequence must be specified")
	}

	return nil
}

// validateProposalResponse 检查proposal response
func validateProposalResponse(proposalResponse *peer.ProposalResponse) error {
	if proposalResponse == nil {
		return errors.New("received nil proposal response")
	}

	if proposalResponse.Response == nil {
		return errors.New("received proposal response with nil response")
	}

	if proposalResponse.Response.Status != int32(common.Status_SUCCESS) {
		return errors.Errorf("submit proposal failed with status: %d - %s", proposalResponse.Response.Status, proposalResponse.Response.Message)
	}
	return nil
}

func createPolicyBytes(signaturePolicy, channelConfigPolicy string) ([]byte, error) {
	if signaturePolicy == "" && channelConfigPolicy == "" {
		// no policy, no problem
		logger.Warn("The endorsement policy is not configured and the default /Channel/Application/Endorsement is used")
		return nil, nil
	}

	if signaturePolicy != "" && channelConfigPolicy != "" {
		// mo policies, mo problems
		return nil, errors.New("cannot specify both \"--signature-policy\" and \"--channel-config-policy\"")
	}

	var applicationPolicy *peer.ApplicationPolicy
	if signaturePolicy != "" {
		signaturePolicyEnvelope, err := policydsl.FromString(signaturePolicy)
		if err != nil {
			return nil, errors.Errorf("invalid signature policy: %s", signaturePolicy)
		}

		applicationPolicy = &peer.ApplicationPolicy{
			Type: &peer.ApplicationPolicy_SignaturePolicy{
				SignaturePolicy: signaturePolicyEnvelope,
			},
		}
	}

	if channelConfigPolicy != "" {
		applicationPolicy = &peer.ApplicationPolicy{
			Type: &peer.ApplicationPolicy_ChannelConfigPolicyReference{
				ChannelConfigPolicyReference: channelConfigPolicy,
			},
		}
	}

	policyBytes := protoutil.MarshalOrPanic(applicationPolicy)
	return policyBytes, nil
}

// packGolangCC read provided src expecting Golang source code, repackage it in provided namespace, and compress it
func packGolangCC(namespace, source string, libs []ChaincodeLibrary) ([]byte, error) {
	twBuf := new(bytes.Buffer)
	tw := tar.NewWriter(twBuf)

	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)

	concatLibs := append(libs, ChaincodeLibrary{SrcPath: source, Namespace: namespace})

	for _, s := range concatLibs {
		_, err := os.Stat(s.SrcPath)
		if err != nil {
			logger.Errorf("%s is not exists: %v", s.SrcPath, err)
			return nil, err
		}
		baseDir := path.Join("/src", s.Namespace)
		err = filepath.Walk(s.SrcPath,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				header, err := tar.FileInfoHeader(info, "")
				if err != nil {
					return err
				}

				header.Mode = 0100000
				if baseDir != "" {
					header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, s.SrcPath))
				}
				if header.Name == baseDir {
					return nil
				}

				if err := tw.WriteHeader(header); err != nil {
					return err
				}

				if info.IsDir() {
					return nil
				}

				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()
				_, err = io.Copy(tw, file)

				return err
			})
		if err != nil {
			logger.Errorf("packaging chaincode failed: %v", err)
			tw.Close()
			return nil, err
		}
	}
	_, err := zw.Write(twBuf.Bytes())
	if err != nil {
		logger.Errorf("zw write failed: %v", err)
		return nil, err
	}
	tw.Close()
	zw.Close()
	return gzBuf.Bytes(), nil
}
