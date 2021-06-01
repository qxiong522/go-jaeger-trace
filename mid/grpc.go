package tracemid

import (
	"context"
	"strings"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tracerLog "github.com/opentracing/opentracing-go/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	trace "github.com/qxiong522/go-qx-trace"
)

const (
	grpcOperationName = "grpc_request"
)

// MDReaderWriter metadata不存在ForeachKey成员方法，这里需要重新声明实现
type MDReaderWriter struct {
	metadata.MD
}

// ForeachKey 读取metadata中的span信息
func (c MDReaderWriter) ForeachKey(handler func(key, val string) error) error {
	for k, vs := range c.MD {
		for _, v := range vs {
			if err := handler(k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c MDReaderWriter) Set(key, val string) {
	key = strings.ToLower(key)
	c.MD[key] = append(c.MD[key], val)
}

// GRPCServerTracerInterceptor GRPC 服务端拦截器
func GRPCServerTracerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	// 从 context 中获取 metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		//如果对metadata进行修改，那么需要用拷贝的副本进行修改。
		md = md.Copy()
	}

	var parentSpan opentracing.Span
	carrier := MDReaderWriter{md}
	//tracer := opentracing.GlobalTracer()
	// 解析出 span
	spanContext, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, carrier)
	if err != nil {
		span := opentracing.GlobalTracer().StartSpan("grpc:" + info.FullMethod)
		defer span.Finish()
	} else {
		// 如果请求有带 trace id 则生成
		parentSpan = opentracing.StartSpan(
			"grpc:"+info.FullMethod,
			opentracing.ChildOf(spanContext),
			opentracing.Tag{Key: string(ext.Component), Value: "GRPC_Server"},
			ext.SpanKindRPCServer,
		)
		defer parentSpan.Finish()
	}
	ctx = opentracing.ContextWithSpan(ctx, parentSpan)
	return handler(ctx, req)
}

// GRPCClientTracerInterceptor GRPC 客户端拦截器
func GRPCClientTracerInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

	spanContext := ctx.Value(trace.TRACER_PARENT_SPAN_CTX_KEY)
	if spanContext == nil {
		// todo: 警告
		return nil
	}

	span := opentracing.StartSpan(
		grpcOperationName,
		opentracing.ChildOf(spanContext.(opentracing.SpanContext)),
		opentracing.Tag{Key: string(ext.Component), Value: "GRPC_Client"},
		ext.SpanKindRPCClient,
	)
	defer span.Finish()

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}
	injectErr := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, MDReaderWriter{md})
	if injectErr != nil {
		span.LogFields(tracerLog.String("inject_err", injectErr.Error()))
		return nil
	}

	newCtx := metadata.NewOutgoingContext(ctx, md)
	err := invoker(newCtx, method, req, reply, cc, opts...)
	if err != nil {
		span.LogFields(tracerLog.String("call_error", err.Error()))
	}
	return err
}
