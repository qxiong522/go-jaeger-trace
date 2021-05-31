package tool

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"time"

	tracemid "go-qx-trace/mid"
)

type (
	RequestMiddleware interface {
		ReqMid(ctx context.Context, r *http.Request) (*http.Request, error)
		RespMid(ctx context.Context, resp *http.Response) (*http.Response, error)
	}
)

func SendRequestWithTrace(ctx context.Context, method, url string, data map[string]interface{}) (*http.Response, error) {
	return SendRequest(ctx, method, url, data, tracemid.SetReqTraceMid())
}

func SendRequest(ctx context.Context, method, url string, data map[string]interface{}, reqOpt ...RequestMiddleware) (*http.Response, error) {
	var (
		err  error
		req  *http.Request
		resp *http.Response
	)
	// 跳过https证书校验
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   time.Duration(5) * time.Second,
		Transport: tr,
	}
	//请求数据
	byteDates, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	reqBodyReader := bytes.NewReader(byteDates)
	//构建req
	req, err = http.NewRequest(method, url, reqBodyReader)
	if err != nil {
		return nil, err
	}

	for _, middleware := range reqOpt {
		req, err = middleware.ReqMid(ctx, req)
		if err != nil {
			log.Printf("warnnig: called req middleware start failed, err:%v\n", err)
			continue
		}
	}

	//发送请求
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}

	for _, middleware := range reqOpt {
		resp, err = middleware.RespMid(ctx, resp)
		if err != nil {
			log.Printf("called req middleware end failed, err:%v\n", err)
			continue
		}
	}

	return resp, nil
}
