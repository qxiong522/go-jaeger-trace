package tool

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"time"

	tracemid "github.com/qxiong522/go-jaeger-trace/mid"
)

type (
	RequestMiddleware interface {
		ReqMid(ctx context.Context, r *http.Request) (*http.Request, error)
		RespMid(ctx context.Context, resp *http.Response) (*http.Response, error)
	}
)

type (
	Option func(opts *reqOptions)

	reqOptions struct {
		client     *http.Client
		timeout    int
		middleware []RequestMiddleware
	}
)

func WithReqMiddle(reqMids ...RequestMiddleware) Option {
	return func(opts *reqOptions) {
		for _, reqMid := range reqMids {
			opts.middleware = append(opts.middleware, reqMid)
		}
	}
}

func WithReqMiddleTimeout(timeout int) Option {
	return func(opts *reqOptions) {
		if timeout > 0 {
			opts.timeout = timeout
		}
	}
}

func WithReqClient(client *http.Client) Option {
	return func(opts *reqOptions) {
		if client != nil {
			opts.client = client
		}
	}
}

func SendRequestWithTrace(ctx context.Context, method, url string, data map[string]interface{}, reqOpt ...Option) (*http.Response, error) {
	reqOpt = append(reqOpt, WithReqMiddle(tracemid.SetReqTraceMid()))
	return SendRequest(ctx, method, url, data, reqOpt...)
}

func SendRequest(ctx context.Context, method, url string, data map[string]interface{}, reqOpts ...Option) (*http.Response, error) {
	opts := buildOpts(reqOpts...)

	var (
		err  error
		req  *http.Request
		resp *http.Response
	)

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

	for _, middleware := range opts.middleware {
		req, err = middleware.ReqMid(ctx, req)
		if err != nil {
			log.Printf("warnnig: called req middleware start failed, err:%v\n", err)
			continue
		}
	}

	//发送请求
	resp, err = opts.client.Do(req)
	if err != nil {
		return nil, err
	}

	for _, middleware := range opts.middleware {
		resp, err = middleware.RespMid(ctx, resp)
		if err != nil {
			log.Printf("warnning: called req middleware end failed, err:%v\n", err)
			continue
		}
	}

	return resp, nil
}

func buildOpts(opts ...Option) *reqOptions {
	options := newDefaultOpts()
	for _, opt := range opts {
		opt(options)
	}
	return options
}

func newDefaultOpts() *reqOptions {
	defaultTimeout := 5
	// 跳过https证书校验
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defaultClient := &http.Client{
		Timeout:   time.Duration(defaultTimeout) * time.Second,
		Transport: tr,
	}
	return &reqOptions{
		client:     defaultClient,
		timeout:    defaultTimeout,
		middleware: make([]RequestMiddleware, 0),
	}
}
