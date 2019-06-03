package restful

import (
	"fmt"

	"github.com/SmartMeshFoundation/Photon-Monitoring/models"
	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/ethereum/go-ethereum/common"
)

/*
BakDelegate 提交委托
提交委托包含两个部分，一个是 UpdateTransfer，一个是 WithDraw
SM 应该校验用户提交信息,比如签名是否正确, nonce 是否比上一次的更大等,
对于 withdraw 部分,应该校验Secret 是否正确,对应的 hashlock 是否在 lockroot 中包含等.
示例请求以及响应
Post /\<delegater\>/delegate
```json
{
    "channel_identifier": "0x4fa00ea25da02ecce11ced3d6167601c67762933adb98dfd82ad4f9b40f4db1a",
    "open_block_number": 15338350,
    "token_address": "0x261e9136f561ae44788bd2a08039075119c02eb3",
    "partner_address": "0x3af7fbddef2cebeeb850328a0834aa9a29684332",
    "update_transfer": {
        "nonce": 0,
        "transfer_amount": null,
        "locksroot": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "extra_hash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "closing_signature": null,
        "non_closing_signature": null
    },
    "unlocks": null,
    "punishes": [
        {
            "lock_hash": "0x933c446b9ee22072f26677561d25b1219cf0bc1ebd3cc5c1d8fa23270df6f609",
            "additional_hash": "0x9ec2e16ee3f883ae1becadd741ff41f03c28f423e532c8beea10034af73a65f5",
            "signature": "Wx2DpiTU/CE1l/FtgCxfhIcheRyxj3V9fgmOJ2HeenoeiQUFzE6XMDcPJ9R43OICXxuQBdxOQm25khbp1GbpYRw="
        }
    ]
}
```

```json
{
  "id":3948588888488485，
  "status":1，//1 表示委托失败，2委托成功，但是余额不足，3委托成功，并且余额充足。
  "error": "错误描述", //正常情况下应该为空
}

*/

const (
	delegateError                      = 1
	delegateSuccessButNotEnoughBalance = 2
	delegateSuccess                    = 3
)

type delegateResponse struct {
	Status int
	Error  string
}

//Delegate user's balance proof updated,
func Delegate(w rest.ResponseWriter, r *rest.Request) {
	var err2 error
	req := &models.ChannelFor3rd{}
	delegateStr := r.PathParam("delegater")
	delegater := common.HexToAddress(delegateStr)
	if delegater == utils.EmptyAddress {
		err2 = w.WriteJson(
			&delegateResponse{
				Status: delegateError,
				Error:  "empty delegater",
			},
		)
		return
	}
	err := r.DecodeJsonPayload(req)
	if err != nil {
		log.Error(err.Error())
		err2 = w.WriteJson(&delegateResponse{
			Status: delegateError,
			Error:  err.Error(),
		})
		return
	}
	log.Trace(fmt.Sprintf("req=%s", utils.StringInterface(req, 3)))
	if verify != nil {
		err = verify.VerifyDelegate(req, delegater)
		if err != nil {
			log.Error(err.Error())
			err2 = w.WriteJson(&delegateResponse{
				Status: delegateError,
				Error:  err.Error(),
			})
			return
		}
	}
	err = db.ReceiveDelegate(req, delegater)
	if err != nil {
		log.Error(err.Error())
		err2 = w.WriteJson(&delegateResponse{
			Status: delegateError,
			Error:  err.Error(),
		})
		return
	}
	if db.AccountIsBalanceEnough(delegater) {
		err2 = w.WriteJson(&delegateResponse{
			Status: delegateSuccess,
		})
	} else {
		err2 = w.WriteJson(&delegateResponse{
			Status: delegateSuccessButNotEnoughBalance,
		})
	}
	if err2 != nil {
		log.Error(fmt.Sprintf("write json err %s", err2))
	}

}
