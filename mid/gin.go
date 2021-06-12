package tracemid

import (
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// SetGinTraceMid 创建链路追踪中间件
func SetGinTraceMid() gin.HandlerFunc {
	return func(c *gin.Context) {
		globalTracer := opentracing.GlobalTracer()
		if globalTracer == nil {
			c.Next()
			return
		}

		operationName := c.Request.URL.Host + "/" + c.Request.URL.Path
		var parentSpan opentracing.Span
		spCtx, err := globalTracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(c.Request.Header))
		if err != nil {
			// 如果请求没有带 tracer id 则生成新的 span
			parentSpan = globalTracer.StartSpan(operationName)
			defer parentSpan.Finish()
		} else {
			// 如果请求有带 tracer id 从父 span 生成子 span
			parentSpan = opentracing.StartSpan(
				operationName,
				opentracing.ChildOf(spCtx),
				opentracing.Tag{Key: string(ext.Component), Value: "Gin-Http"},
				opentracing.Tag{Key: "path", Value: c.Request.URL.Path},
				opentracing.Tag{Key: "host", Value: c.Request.URL.Host},
				opentracing.Tag{Key: "method", Value: c.Request.Method},
				ext.SpanKindRPCServer,
			)
			defer parentSpan.Finish()
		}
		c.Request = c.Request.WithContext(opentracing.ContextWithSpan(c.Request.Context(), parentSpan))
		c.Set(_HTTP_FRAME_CTX_KEY, _HTTP_FRAME_GIN)
		c.Next()
	}
}
