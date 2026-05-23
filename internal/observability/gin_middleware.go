package observability

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		ctx := c.Request.Context()
		logger := WithContext(ctx,
			zap.String("path", path),
			zap.String("method", c.Request.Method),
		)
		c.Request = c.Request.WithContext(ContextWithLogger(ctx, logger))

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		fullPath := path
		if raw != "" {
			fullPath = path + "?" + raw
		}

		fields := []zap.Field{
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.String("path", fullPath),
			zap.String("method", c.Request.Method),
		}
		if errMsg := c.Errors.ByType(gin.ErrorTypePrivate).String(); errMsg != "" {
			fields = append(fields, zap.String("errors", errMsg))
		}

		switch {
		case status >= 500:
			WithContext(c.Request.Context()).Error("http request", fields...)
		case status >= 400:
			WithContext(c.Request.Context()).Warn("http request", fields...)
		default:
			WithContext(c.Request.Context()).Info("http request", fields...)
		}
	}
}

func GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				WithContext(c.Request.Context()).Error("panic recovered",
					zap.Any("panic", rec),
					zap.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}
