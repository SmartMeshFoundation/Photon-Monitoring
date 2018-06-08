package restful

import (
	"fmt"

	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/models"
	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/ethereum/go-ethereum/common"
)

/*
Delegate 提交委托
提交委托包含两个部分，一个是 UpdateTransfer，一个是 WithDraw
SM 应该校验用户提交信息,比如签名是否正确, nonce 是否比上一次的更大等,
对于 withdraw 部分,应该校验Secret 是否正确,对应的 hashlock 是否在 lockroot 中包含等.
示例请求以及响应
Post /\<delegater\>/delegate
```json
{
    "channel_address": "0x5B3F0E96E45e1e4351F6460feBfB6007af25FBB0",
 "update_transfer":{
        "nonce": 32,
        "transferred_amount": 1800000000000000,
        "locksroot": " 0x447b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e89",
        "extra_hash": " 0x557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e89",
        "closing_signature": " 0x557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e89557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e8927",
        "non_closing_signature": " 0x557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e89557b478a024ade59c5c18e348c357aae6a4ec6e30131213f8cf6444214c57e8927"
 },
 "withdraws":[
     {
        "locked_encoded": "0x00000033333333333333333333333333333333333333333",
        "merkle_proof": "0x3333333333333333333333333333",
        "secret": "0x333333333333333333333333333333333333333",
     },
      {
        "locked_encoded": "0x00000033333333333333333333333333333333333333333",
        "merkle_proof": "0x3333333333333333333333333333",
        "secret": "0x333333333333333333333333333333333333333",
     },
 ],
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
	err = db.DelegateNewDelegate(req, delegater)
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
