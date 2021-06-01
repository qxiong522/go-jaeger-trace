package tracemid

import (
	trace "github.com/qxiong522/go-jaeger-trace"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tracerLog "github.com/opentracing/opentracing-go/log"
	"gorm.io/gorm"
)

func before(db *gorm.DB) {
	// 先从父级spans生成子span
	parentSpan := db.Statement.Context.Value(trace.TRACER_PARENT_SPAN_CTX_KEY)
	subSpan := opentracing.StartSpan(
		gormOperationName,
		opentracing.ChildOf(parentSpan.(opentracing.SpanContext)),
		opentracing.Tag{Key: string(ext.Component), Value: "gorm"},
		ext.SpanKindRPCClient,
	)
	// 利用db实例去传递span
	db.InstanceSet(gormSpanKey, subSpan)
	return
}

func after(db *gorm.DB) {
	// 从GORM的DB实例中取出span
	_span, isExist := db.InstanceGet(gormSpanKey)
	if !isExist {
		return
	}

	// 断言进行类型转换
	span, ok := _span.(opentracing.Span)
	if !ok {
		return
	}
	defer func() {
		span.Finish()
	}()

	// Error
	if db.Error != nil {
		span.LogFields(tracerLog.Error(db.Error))
	}

	// sql
	span.LogFields(tracerLog.String("sql", db.Dialector.Explain(db.Statement.SQL.String(), db.Statement.Vars...)))
	return
}

type GormV2OpentracingPlugin struct{}

func (op *GormV2OpentracingPlugin) Name() string {
	return "opentracingPlugin"
}

func (op *GormV2OpentracingPlugin) Initialize(db *gorm.DB) (err error) {
	// 开始前
	_ = db.Callback().Create().Before("gorm:before_create").Register(callBackBeforeName, before)
	_ = db.Callback().Query().Before("gorm:query").Register(callBackBeforeName, before)
	_ = db.Callback().Delete().Before("gorm:before_delete").Register(callBackBeforeName, before)
	_ = db.Callback().Update().Before("gorm:setup_reflect_value").Register(callBackBeforeName, before)
	_ = db.Callback().Row().Before("gorm:row").Register(callBackBeforeName, before)
	_ = db.Callback().Raw().Before("gorm:raw").Register(callBackBeforeName, before)

	// 结束后
	_ = db.Callback().Create().After("gorm:after_create").Register(callBackAfterName, after)
	_ = db.Callback().Query().After("gorm:after_query").Register(callBackAfterName, after)
	_ = db.Callback().Delete().After("gorm:after_delete").Register(callBackAfterName, after)
	_ = db.Callback().Update().After("gorm:after_update").Register(callBackAfterName, after)
	_ = db.Callback().Row().After("gorm:row").Register(callBackAfterName, after)
	_ = db.Callback().Raw().After("gorm:raw").Register(callBackAfterName, after)
	return
}
