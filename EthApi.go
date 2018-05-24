package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const MaxTry = 20
const url = "http://localhost:8545"

//Eth_API_Request Eth api 请求数据
type EthApiRequest struct {
	Id      int64       `json:"id"`
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

//Eth_API_Result Eth api 返回结果
type EthApiResult struct {
	Id      int64       `json:"id"`
	Jsonrpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

//SendTransaction 参数
//"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
//"to": "0xd46e8dd67c5d32be8058bb8eb970870f07244567",
//"gas": "0x76c0", // 30400
//"gasPrice": "0x9184e72a000", // 10000000000000
//"value": "0x9184e72a", // 2441406250
//"data": "0xd46e8dd67c5d32be8d46e8dd67c5d32be8058bb8eb970870f072445675058bb8eb970870f072445675"
type TransactionPram struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Gas      string `json:"gas"`
	GasPrice string `json:"gasPrice"`
	Value    string `json:"value"`
	Data     string `json:"data"`
}

/*
函数名：CallEthApi
功能：调用geth API功能模块
版本：1.0
日期：2018.05.01
码农：SmartMesh
参数：
    url： 插口，字符串,"http://localhost:8545"；
    id： 序号，整型,1；
	jsonrpc：rpc版本信息，字符串,"2.0"；
	method：功能，字符串,"eth_getBalance"；
	parms:参数列表，字符数组,["0x1a9ec3b0b807464e6d3398a59d6b0a369bf422fa","0x6666"]。
返回：
	result：结果，EthApiResult类型,{2 2.0 0xcdcd77c0992ec5bbfc459984220f8c45084cc24d9b6efed1fae540db8de801d2}；
	err:错误,error类型,nil；
	Status：http返回头状态，字符串,"200 OK"。
修改记录：
*/
func CallEthApi(url string, id int64, jsonrpc string, method string, Params []string) (result EthApiResult, err error, Status string) {
	var resp *http.Response
	var count int
	var payload EthApiRequest
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()
	payload.Id = id
	payload.Jsonrpc = jsonrpc
	payload.Method = method
	payload.Params = Params
	p, err := json.Marshal(payload)
	if err != nil {
		log.Println(err)
	}
	for count = 0; count < MaxTry; count = count + 1 {
		client := &http.Client{}
		client.Timeout = 6000
		var req *http.Request
		req, err = http.NewRequest(http.MethodPost, url, bytes.NewReader(p))
		//resp, err = http.Post(url, "application/json", strings.NewReader(ps))
		if err != nil {
			log.Println(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", "name=anny")
		req.Close = false
		//resp, err = http.DefaultClient.Do(req)
		resp, err = client.Do(req)
		if err == nil {
			if resp != nil {
				//io.Copy(os.Stdout, resp.Body)
				p, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
				}
				//var s string = string(p[:])
				//fmt.Println(s)
				err = json.Unmarshal(p, &result)
				if err != nil {
					log.Println(err)
				}
			}
			break
		}
		time.Sleep(time.Second)
	}
	if count >= MaxTry {
		Status = "504 TimeOut"
	} else {
		Status = resp.Status
	}
	return result, err, Status
}

/*
函数名：Str2hexstr
功能：字符串转hex字符串
版本：1.0
日期：2018.05.01
码农：SmartMesh
参数：
    str： 源串,字符串,"baz(uint32,bool)"；
返回：
	hexstr：hex字符串,字符串,"0x62617a2875696e7433322c626f6f6c29"。
修改记录：
*/
func Str2hexstr(str string, hexflag bool) (hexstr string) {
	if hexflag {
		hexstr = "0x"
	}
	for _, v := range str {
		hexstr = hexstr + fmt.Sprintf("%02x", v)
	}
	return hexstr
}

/*
函数名：EthSign
功能：签名
版本：1.0
日期：2018.05.01
码农：SmartMesh
参数：
	accountAddress： 账号地址，字符串,"0x1a9ec3b0b807464e6d3398a59d6b0a369bf422fa"；
	msg：待签名信息，hex字符串，字符串,"0x1a9ec3b0b807464e6d3398a59d6b0a369bf422fa"；
返回：
修改记录：
*/
func EthSign(accountAddress string, msg string, nonce int64) (SignMessage string, Status string, err error) {
	var Param []string
	Param = append(Param, accountAddress)
	Param = append(Param, msg)
	result, err, Status := CallEthApi(url, nonce, "2.0", "eth_sign", Param)
	SignMessage = fmt.Sprintf("%s", result.Result)
	return
}

/*
函数名：EthSendTransaction
功能：发起交易
版本：1.0
日期：2018.05.01
码农：SmartMesh
参数：
	accountAddress： 账号地址，字符串,"0x1a9ec3b0b807464e6d3398a59d6b0a369bf422fa"；
	msg：待签名信息，hex字符串，字符串,"0x1a9ec3b0b807464e6d3398a59d6b0a369bf422fa"；
返回：
修改记录：
*/
func EthSendTransaction(trans TransactionPram, nonce int64) (resultMsg string, Status string, err error) {
	var Param []string
	t, err := json.Marshal(trans)
	Param = append(Param, string(t))
	fmt.Println(Param)
	result, err, Status := CallEthApi(url, nonce, "2.0", "eth_sendTransaction", Param)
	resultMsg = fmt.Sprintf("%s", result.Result)
	return
}

/*
函数名：EthApiTest
功能：测试ethapi
版本：1.0
日期：2018.05.01
码农：SmartMesh
参数：
返回：
修改记录：
*/
func EthApiTest() {
	node1 := "0x1a9ec3b0b807464e6d3398a59d6b0a369bf422fa"
	node2 := "0x33df901abc22dcb7f33c2a77ad43cc98fbfa0790"
	var Param []string
	Param = append(Param, node1)
	Param = append(Param, "latest")
	result, err, Status := CallEthApi(url, 1, "2.0", "eth_getBalance", Param)
	fmt.Println("eth_getBalance:", result.Result)
	if Status != "200 OK" {
		fmt.Println(Status)
	}
	if err != nil {
		log.Println(err)
	}

	Param = nil
	s := "baz(uint32,bool)"
	hs := Str2hexstr(s, true)
	//fmt.Println(hs)
	Param = append(Param, hs)
	result, err, Status = CallEthApi(url, 2, "2.0", "web3_sha3", Param)
	fmt.Println("web3_sha3:", result.Result)
	if Status != "200 OK" {
		fmt.Println(Status)
	}
	if err != nil {
		log.Println(err)
	}

	sig, Status, err := EthSign(node1, node1, 3)

	fmt.Println("eth_sign：", sig)
	if Status != "200 OK" {
		fmt.Println(Status)
	}
	if err != nil {
		log.Println(err)
	}

	var trans TransactionPram
	trans.From = node1
	trans.To = node2
	trans.Gas = "0x76c0"
	trans.GasPrice = "0x9184e72a000"
	trans.Value = "0x9184e72a" // 2441406250
	trans.Data = ""
	resultMsg, Status, err := EthSendTransaction(trans, 4)
	fmt.Println("eth_SendTransaction：", resultMsg)
	if Status != "200 OK" {
		fmt.Println(Status)
	}
	if err != nil {
		log.Println(err)
	}
}
