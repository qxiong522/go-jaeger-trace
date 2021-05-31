package trace

import (
	"io"
	"sync"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegerConfig "github.com/uber/jaeger-client-go/config"
)

var tracer opentracing.Tracer
var closer io.Closer
var err error
var once sync.Once

/*
NewJaegerTracer 创建 Jaeger 链路追踪器
Args:
 - jaegerHostPort: jaeger agent 组件 IP 地址 及 端口. 例如："127.0.0.1:6831"
 - serviceName: 服务名称
*/
func NewJaegerTracer(serviceName string, jaegerHostPort string) (opentracing.Tracer, io.Closer, error) {
	once.Do(func() {
		cfg := &jaegerConfig.Configuration{
			Sampler: &jaegerConfig.SamplerConfig{
				Type:  "const", //固定采样
				Param: 1,       //1=全采样、0=不采样
			},

			Reporter: &jaegerConfig.ReporterConfig{
				LogSpans:           true,
				LocalAgentHostPort: jaegerHostPort,
			},

			ServiceName: serviceName,
		}

		tracer, closer, err = cfg.NewTracer(jaegerConfig.Logger(jaeger.StdLogger))
		if err == nil {
			opentracing.SetGlobalTracer(tracer)
		}
	})
	return tracer, closer, err
}
