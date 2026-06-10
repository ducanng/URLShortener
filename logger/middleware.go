package logger

import "github.com/gin-gonic/gin"

// TraceHeader is the canonical HTTP header used to carry trace_id between
// services. Lower-case form is also accepted on incoming requests because
// http.Header lookups are case-insensitive.
const TraceHeader = "X-Trace-Id"

// TraceIDMiddleware is a Gin middleware that:
//   - Reads X-Trace-Id from the incoming request, or generates a fresh UUID
//     when absent.
//   - Stores the trace_id in the request's context.Context so downstream
//     handlers/storage code can pick it up via TraceIDFromContext.
//   - Echoes the trace_id back to the client via the response header so a
//     curl/postman caller can correlate their request with server logs.
//
// Mount this once at the top of the Gin router, before any business handlers
// or grpc-gateway, so every request carries a trace_id.
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(TraceHeader)
		if traceID == "" {
			traceID = NewTraceID()
		}
		ctx := WithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)
		c.Header(TraceHeader, traceID)
		c.Next()
	}
}
