package verifier

import (
	"github.com/SmartMeshFoundation/Photon-Monitoring/models"
	"github.com/ethereum/go-ethereum/common"
)

/*
DelegateVerifier verify delegate from user is valid or not
SM 应该校验用户提交信息,比如签名是否正确,
对于 withdraw 部分,应该校验Secret 是否正确,对应的 hashlock 是否在 lockroot 中包含等.
//todo 为了解决用户进行委托的时候通道已经关闭的问题,这里对c做了修改,后续应该重构解决这个问题
*/
type DelegateVerifier interface {
	VerifyDelegate(c *models.ChannelFor3rd, delegater common.Address) error
}
