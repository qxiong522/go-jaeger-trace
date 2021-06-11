package trace

import (
	"io"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegerConfig "github.com/uber/jaeger-client-go/config"
)

var tracer opentracing.Tracer
var closer io.Closer
var err error
var once sync.Once

type (
	Option func(opts *jaegerTracerOptions)

	jaegerTracerOptions struct {
		log     bool
		disable bool

		reporterCollectorEndpoint   string
		reporterQueueSize           int
		reporterLogSpans            bool
		reporterBufferFlushInterval time.Duration

		samplerType  string
		samplerParam float64
	}
)

/*
NewJaegerTracer 创建 Jaeger 链路追踪器
Args:
 - jaegerHostPort: jaeger agent 组件 IP 地址 及 端口. 例如："127.0.0.1:6831"
 - serviceName: 服务名称
*/
func NewJaegerTracer(serviceName string, jaegerHostPort string, opts ...Option) (opentracing.Tracer, io.Closer, error) {
	once.Do(func() {
		options := buildOptions(opts...)
		cfg := &jaegerConfig.Configuration{
			ServiceName: serviceName,
			Disabled:    options.disable,
			RPCMetrics:  false,
			Tags:        nil,
			Sampler: &jaegerConfig.SamplerConfig{
				Type:                     options.samplerType,
				Param:                    options.samplerParam,
				MaxOperations:            0,
				OperationNameLateBinding: false,
				Options:                  nil,
			},
			Reporter: &jaegerConfig.ReporterConfig{
				QueueSize:                  options.reporterQueueSize,
				BufferFlushInterval:        options.reporterBufferFlushInterval,
				LogSpans:                   options.reporterLogSpans,
				LocalAgentHostPort:         jaegerHostPort,
				DisableAttemptReconnecting: false,
				AttemptReconnectInterval:   0,
				CollectorEndpoint:          options.reporterCollectorEndpoint,
				User:                       "",
				Password:                   "",
				HTTPHeaders:                nil,
			},
			Headers:             nil,
			BaggageRestrictions: nil,
			Throttler:           nil,
		}

		if options.log {
			tracer, closer, err = cfg.NewTracer(jaegerConfig.Logger(jaeger.StdLogger))
		} else {
			tracer, closer, err = cfg.NewTracer()
		}
		if err == nil {
			opentracing.SetGlobalTracer(tracer)
		}
	})
	return tracer, closer, err
}

// WithDisable 是否启动
func WithDisable(disable bool) Option {
	return func(opts *jaegerTracerOptions) {
		opts.disable = disable
	}
}

// WithLog 是否开启日志
func WithLog(log bool) Option {
	return func(opts *jaegerTracerOptions) {
		opts.log = log
	}
}

// WithReporterQueueSize 设置 设置队列大小，存储采样的 span 信息，队列满了后一次性发送到 jaeger 后端；defaultQueueSize 默认为 100；
func WithReporterQueueSize(queueSize int) Option {
	return func(opts *jaegerTracerOptions) {
		if queueSize > 0 {
			opts.reporterQueueSize = queueSize
		}
	}
}

// WithBufferFlushInterval 强制清空、推送队列时间，对于流量不高的程序，队列可能长时间不能满，那么设置这个时间，超时可以自动推送一次。
// 对于高并发的情况，一般队列很快就会满的，满了后也会自动推送。默认为1秒
func WithBufferFlushInterval(bufferFlushInterval time.Duration) Option {
	return func(opts *jaegerTracerOptions) {
		if bufferFlushInterval >= 0 {
			opts.reporterBufferFlushInterval = bufferFlushInterval
		}
	}
}

// WithSamplerType 设置 采样方式
// "const"			0或1	采样器始终对所有 tracer 做出相同的决定；要么全部采样，要么全部不采样
// "probabilistic"	0.0~1.0	采样器做出随机采样决策，Param 为采样概率
// "ratelimiting"	N		采样器一定的恒定速率对tracer进行采样，Param=2.0，则限制每秒采集2条
// "remote"			无		采样器请咨询Jaeger代理以获取在当前服务中使用的适当采样策略。
func WithSamplerType(samplerType string) Option {
	return func(opts *jaegerTracerOptions) {
		if samplerType != "" {
			opts.samplerType = samplerType
		}
	}
}

// WithSamplerParam 设置 采样率 0 - 1
func WithSamplerParam(samplerParam float64) Option {
	return func(opts *jaegerTracerOptions) {
		if samplerParam >= 0 {
			opts.samplerParam = samplerParam
		}
	}
}

// WithReporterLogSpans 设置 是否把 Log 也推送，span 中可以携带一些日志信息
func WithReporterLogSpans(logSpans bool) Option {
	return func(opts *jaegerTracerOptions) {
		opts.reporterLogSpans = logSpans
	}
}

// WithCollectorEndpoint 设置 api (链路追踪数据直接上报 collector 组件)
// 用 Collector 就不用 agent 了
func WithCollectorEndpoint(collectorEndpoint string) Option {
	return func(opts *jaegerTracerOptions) {
		if collectorEndpoint != "" {
			opts.reporterCollectorEndpoint = collectorEndpoint
		}
	}
}

func buildOptions(opts ...Option) *jaegerTracerOptions {
	options := newDefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	return options
}

func newDefaultOptions() *jaegerTracerOptions {
	return &jaegerTracerOptions{
		log:     false,
		disable: false,

		reporterCollectorEndpoint:   "",
		reporterQueueSize:           50,
		reporterLogSpans:            true,
		reporterBufferFlushInterval: 1,

		samplerType:  jaeger.SamplerTypeConst,
		samplerParam: 1,
	}
}
