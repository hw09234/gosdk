/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-protos-go/peer/lifecycle"
	"github.com/hyperledger/fabric/core/chaincode/platforms/golang"
	"github.com/pkg/errors"
)

const (
	deploy      = "deploy"
	upgrade     = "upgrade"
	adminUser   = "adminUser" // 默认admin用户
	defaultUser = "defaultUser"
)

// getChainCodeObj 创建chaincode结构
func getChainCodeObj(args []string, transientMap map[string][]byte, channelName, chaincodeName string, codeType ChainCodeType, isInit bool) (*ChainCode, error) {
	if channelName == "" || chaincodeName == "" {
		return nil, errors.New("channel or chaincode name is nil")
	}

	chaincode := ChainCode{
		ChannelId:    channelName,
		Name:         chaincodeName,
		Type:         codeType,
		IsInit:       isInit,
		Args:         args,
		TransientMap: transientMap,
	}

	return &chaincode, nil
}

func generateRangeNum(min, max int) int {
	rand.Seed(time.Now().Unix())
	randNum := rand.Intn(max-min) + min
	return randNum
}

func containsStr(strList []string, str string) bool {
	for _, v := range strList {
		if v == str {
			return true
		}
	}
	return false
}

// isNum 判断字符串内容是否为数字
func isNum(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// isDomainName 判断地址是否为域名
func isDomainName(s string) bool {
	if s == "localhost" {
		return false
	}

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			continue
		default:
			if !isNum(string(s[i])) {
				return true
			}
		}
	}

	return false
}

// isIPAdress 判断IP地址是否正确
func isIPAdress(s string) bool {
	if s == "localhost" {
		return true
	}

	ip := net.ParseIP(s)
	return ip != nil
}

func writeFile(path, name string, data []byte) error {
	if path == "" {
		return errors.New("empty path not allowed")
	}
	tmpFile, err := ioutil.TempFile(path, ".ccpackage.")
	if err != nil {
		return errors.Wrapf(err, "error creating temp file in directory '%s'", path)
	}
	defer os.Remove(tmpFile.Name())

	if n, err := tmpFile.Write(data); err != nil || n != len(data) {
		if err == nil {
			err = errors.Errorf(
				"failed to write the entire content of the file, expected %d, wrote %d",
				len(data), n)
		}
		return errors.Wrapf(err, "error writing to temp file '%s'", tmpFile.Name())
	}

	if err := tmpFile.Close(); err != nil {
		return errors.Wrapf(err, "error closing temp file '%s'", tmpFile.Name())
	}

	if err := os.Rename(tmpFile.Name(), filepath.Join(path, name)); err != nil {
		return errors.Wrapf(err, "error renaming temp file '%s'", tmpFile.Name())
	}

	return nil
}

// TODO: 增加对node和java语言的支持
// getTarGzBytes 对chaincode信息进行打包
func getTarGzBytes(info ChaincodeInfo) ([]byte, error) {
	var goPlatform = &golang.Platform{}

	payload := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(payload)
	tw := tar.NewWriter(gw)

	normalizedPath, err := goPlatform.NormalizePath(info.Path)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to normalize chaincode path")
	}

	metadataBytes, err := toJSON(normalizedPath, info.Type.string(), info.Label)
	if err != nil {
		return nil, err
	}
	err = writeBytesToPackage(tw, "metadata.json", metadataBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error writing package metadata to tar")
	}

	codeBytes, err := goPlatform.GetDeploymentPayload(info.Path)
	if err != nil {
		return nil, errors.WithMessage(err, "error getting chaincode bytes")
	}

	codePackageName := "code.tar.gz"

	err = writeBytesToPackage(tw, codePackageName, codeBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error writing package code bytes to tar")
	}

	err = tw.Close()
	if err == nil {
		err = gw.Close()
	} else {
		return nil, errors.Wrapf(err, "failed to create tar for chaincode")
	}

	return payload.Bytes(), nil
}

// toJSON 序列化chaincode信息
func toJSON(path, ccType, label string) ([]byte, error) {
	metadata := &PackageMetadata{
		Path:  path,
		Type:  ccType,
		Label: label,
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal chaincode package metadata into JSON")
	}

	return metadataBytes, nil
}

func writeBytesToPackage(tw *tar.Writer, name string, payload []byte) error {
	err := tw.WriteHeader(&tar.Header{
		Name: name,
		Size: int64(len(payload)),
		Mode: 0100644,
	})
	if err != nil {
		return err
	}

	_, err = tw.Write(payload)
	if err != nil {
		return err
	}

	return nil
}

// writePackage 将获取到的package写入文件
func writePackage(proposalResponse *peer.ProposalResponse, OutputDirectory, PackageID string) error {
	result := &lifecycle.GetInstalledChaincodePackageResult{}
	err := proto.Unmarshal(proposalResponse.Response.Payload, result)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal proposal response's response payload")
	}

	outputFile := filepath.Join(OutputDirectory, PackageID+".tar.gz")
	dir, name := filepath.Split(outputFile)
	if dir, err = filepath.Abs(dir); err != nil {
		return err
	}

	err = writeFile(dir, name, result.ChaincodeInstallPackage)
	if err != nil {
		return errors.Wrapf(err, "failed to write chaincode package to %s", outputFile)
	}

	return nil
}
