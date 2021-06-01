package tracemid

import (
	"github.com/qxiong522/go-jaeger-trace"

	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// SetGinTraceMid 创建链路追踪中间件
func SetGinTraceMid() gin.HandlerFunc {
	return func(c *gin.Context) {
		var parentSpan opentracing.Span

		spCtx, err := opentracing.GlobalTracer().Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(c.Request.Header))
		if err != nil {
			// 如果请求没有带 tracer id 则生成新的 span
			parentSpan = opentracing.GlobalTracer().StartSpan(c.Request.URL.String())
			defer parentSpan.Finish()
		} else {
			// 如果请求有带 tracer id 从父 span 生成子 span
			parentSpan = opentracing.StartSpan(
				c.Request.URL.String(),
				opentracing.ChildOf(spCtx),
				opentracing.Tag{Key: string(ext.Component), Value: "HTTP"},
				ext.SpanKindRPCServer,
			)
			defer parentSpan.Finish()
		}
		// 键值形式传入 gin.ctx
		c.Set(trace.TRACER_PARENT_SPAN_CTX_KEY, parentSpan.Context())
		c.Next()
	}
}
