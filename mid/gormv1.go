package tracemid

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tracerLog "github.com/opentracing/opentracing-go/log"
)

const (
	gormSpanKey        = "_gorm_span"
	callBackBeforeName = "gorm_tracer:before"
	callBackAfterName  = "gorm_tracer:after"

	gormOperationName = "gorm_operate"
	gormComponent     = "gorm"
)

func beforeV1(scope *gorm.Scope) {
	// 先从父级spans生成子span
	iCtx, ok := scope.DB().Get("ctx")
	if !ok {
		return
	}
	span, _ := opentracing.StartSpanFromContext(getContext(iCtx), gormOperationName,
		opentracing.Tag{Key: string(ext.Component), Value: gormComponent},
		ext.SpanKindRPCClient,
	)
	// 利用db实例去传递span
	scope.InstanceSet(gormSpanKey, span)
	return
}

func afterV1(scope *gorm.Scope) {
	// 从GORM的DB实例中取出span
	iSpan, isExist := scope.InstanceGet(gormSpanKey)
	if !isExist {
		return
	}

	// 断言进行类型转换
	span, ok := iSpan.(opentracing.Span)
	if !ok {
		return
	}
	defer func() {
		span.Finish()
	}()

	// Error
	if scope.DB().Error != nil {
		span.LogFields(tracerLog.Error(scope.DB().Error))
	}

	// sql
	span.LogFields(tracerLog.String("sql", explainSQL(scope.SQL, `'`, scope.SQLVars...)))
	return
}

func GormV1TraceInitialize(db *gorm.DB) {
	// 开始前
	db.Callback().Create().Before("gorm:before_create").Register(callBackBeforeName, beforeV1)
	db.Callback().Query().Before("gorm:query").Register(callBackBeforeName, beforeV1)
	db.Callback().Delete().Before("gorm:before_delete").Register(callBackBeforeName, beforeV1)
	db.Callback().Update().Before("gorm:before_update").Register(callBackBeforeName, beforeV1)
	db.Callback().RowQuery().Before("gorm:before_rowQuery").Register(callBackBeforeName, beforeV1)

	// 结束后
	db.Callback().Create().After("gorm:after_create").Register(callBackAfterName, afterV1)
	db.Callback().Query().After("gorm:after_query").Register(callBackAfterName, afterV1)
	db.Callback().Delete().After("gorm:after_delete").Register(callBackAfterName, afterV1)
	db.Callback().Update().After("gorm:after_update").Register(callBackAfterName, afterV1)
	db.Callback().RowQuery().After("gorm:after_rowQuery").Register(callBackAfterName, afterV1)
	return
}

func isPrintable(s []byte) bool {
	for _, r := range s {
		if !unicode.IsPrint(rune(r)) {
			return false
		}
	}
	return true
}

func explainSQL(sql string, escaper string, avars ...interface{}) string {
	const (
		tmFmtWithMS = "2006-01-02 15:04:05.999"
		tmFmtZero   = "0000-00-00 00:00:00"
		nullStr     = "NULL"
	)
	var convertibleTypes = []reflect.Type{reflect.TypeOf(time.Time{}), reflect.TypeOf(false), reflect.TypeOf([]byte{})}
	var convertParams func(interface{}, int)
	var vars = make([]string, len(avars))

	convertParams = func(v interface{}, idx int) {
		switch v := v.(type) {
		case bool:
			vars[idx] = strconv.FormatBool(v)
		case time.Time:
			if v.IsZero() {
				vars[idx] = escaper + tmFmtZero + escaper
			} else {
				vars[idx] = escaper + v.Format(tmFmtWithMS) + escaper
			}
		case *time.Time:
			if v != nil {
				if v.IsZero() {
					vars[idx] = escaper + tmFmtZero + escaper
				} else {
					vars[idx] = escaper + v.Format(tmFmtWithMS) + escaper
				}
			} else {
				vars[idx] = nullStr
			}
		case driver.Valuer:
			reflectValue := reflect.ValueOf(v)
			if v != nil && reflectValue.IsValid() && ((reflectValue.Kind() == reflect.Ptr && !reflectValue.IsNil()) || reflectValue.Kind() != reflect.Ptr) {
				r, _ := v.Value()
				convertParams(r, idx)
			} else {
				vars[idx] = nullStr
			}
		case fmt.Stringer:
			reflectValue := reflect.ValueOf(v)
			if v != nil && reflectValue.IsValid() && ((reflectValue.Kind() == reflect.Ptr && !reflectValue.IsNil()) || reflectValue.Kind() != reflect.Ptr) {
				vars[idx] = escaper + strings.Replace(fmt.Sprintf("%v", v), escaper, "\\"+escaper, -1) + escaper
			} else {
				vars[idx] = nullStr
			}
		case []byte:
			if isPrintable(v) {
				vars[idx] = escaper + strings.Replace(string(v), escaper, "\\"+escaper, -1) + escaper
			} else {
				vars[idx] = escaper + "<binary>" + escaper
			}
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			vars[idx] = toString(v)
		case float64, float32:
			vars[idx] = fmt.Sprintf("%.6f", v)
		case string:
			vars[idx] = escaper + strings.Replace(v, escaper, "\\"+escaper, -1) + escaper
		default:
			rv := reflect.ValueOf(v)
			if v == nil || !rv.IsValid() || rv.Kind() == reflect.Ptr && rv.IsNil() {
				vars[idx] = nullStr
			} else if valuer, ok := v.(driver.Valuer); ok {
				v, _ = valuer.Value()
				convertParams(v, idx)
			} else if rv.Kind() == reflect.Ptr && !rv.IsZero() {
				convertParams(reflect.Indirect(rv).Interface(), idx)
			} else {
				for _, t := range convertibleTypes {
					if rv.Type().ConvertibleTo(t) {
						convertParams(rv.Convert(t).Interface(), idx)
						return
					}
				}
				vars[idx] = escaper + strings.Replace(fmt.Sprint(v), escaper, "\\"+escaper, -1) + escaper
			}
		}
	}

	for idx, v := range avars {
		convertParams(v, idx)
	}

	var idx int
	var newSQL strings.Builder

	for _, v := range []byte(sql) {
		if v == '?' {
			if len(vars) > idx {
				newSQL.WriteString(vars[idx])
				idx++
				continue
			}
		}
		newSQL.WriteByte(v)
	}

	sql = newSQL.String()

	return sql
}

func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	}
	return ""
}
