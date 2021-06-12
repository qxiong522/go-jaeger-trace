package tracemid

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// getContext 兼容其他类型 context，例如 gin.Context
func getContext(iCtx interface{}) context.Context {
	var ctx context.Context
	switch c := iCtx.(type) {
	case *gin.Context:
		ctx = c.Request.Context()
	case context.Context:
		v := c.Value(_HTTP_FRAME_CTX_KEY)
		if v == nil {
			ctx = c
		} else {
			frame := v.(string)
			switch frame {
			case _HTTP_FRAME_GIN:
				iReqCtx := c.Value(0)
				reqCtx := iReqCtx.(*http.Request)
				ctx = reqCtx.Context()
			default:
				ctx = c
			}
		}
	default:
		return context.Background()
	}
	return ctx
}
