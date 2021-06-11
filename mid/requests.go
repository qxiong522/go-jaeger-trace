package tracemid

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tracerLog "github.com/opentracing/opentracing-go/log"
)

func SetReqTraceMid() *reqTrace {
	return &reqTrace{}
}

type reqTrace struct {
	span opentracing.Span
}

func (m *reqTrace) ReqMid(ctx context.Context, req *http.Request) (*http.Request, error) {
	subSpan, _ := opentracing.StartSpanFromContext(ctx, req.Method+"_request",
		opentracing.Tag{Key: string(ext.Component), Value: "request"},
		opentracing.Tag{Key: "url", Value: req.URL},
		opentracing.Tag{Key: "method", Value: req.Method},
		ext.SpanKindRPCClient,
	)
	defer func() {
		m.span = subSpan
	}()

	injectErr := opentracing.GlobalTracer().Inject(subSpan.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	if injectErr != nil {
		subSpan.LogFields(tracerLog.String("inject_err", injectErr.Error()))
		return req, errors.New(fmt.Sprintf("Inject trace to request failed, err:%v", injectErr))
	}
	return req, nil
}

func (m *reqTrace) RespMid(ctx context.Context, resp *http.Response) (*http.Response, error) {
	if m.span != nil {
		if resp != nil {
			m.span.SetTag("code", resp.StatusCode)
		} else {
			m.span.SetTag("message", "request failed")
		}
		m.span.Finish()
	}
	return resp, nil
}
