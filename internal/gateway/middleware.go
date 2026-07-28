package gateway

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ctxUserID keys the authenticated user's id in the Gin request context.
const ctxUserID = "userID"

// AuthRequired verifies the bearer token and stores the caller's user id in the
// request context for downstream handlers. It aborts with 401 on any problem.
func (s *Server) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		const prefix = "Bearer "
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse("missing or malformed Authorization header"))
			return
		}

		userID, err := s.tokens.Parse(strings.TrimPrefix(header, prefix))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse("invalid or expired token"))
			return
		}

		c.Set(ctxUserID, userID)
		c.Next()
	}
}

// RateLimit throttles requests per client IP using the injected Limiter. On
// limiter errors we fail open (allow the request) rather than lock everyone out.
func (s *Server) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, err := s.limiter.Allow(c.Request.Context(), "ratelimit:"+c.ClientIP())
		if err == nil && !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, errorResponse("rate limit exceeded, try again shortly"))
			return
		}
		c.Next()
	}
}

// userIDFrom extracts the authenticated user id set by AuthRequired. It is only
// valid on routes protected by that middleware.
func userIDFrom(c *gin.Context) uuid.UUID {
	v, ok := c.Get(ctxUserID)
	if !ok {
		return uuid.Nil
	}
	id, _ := v.(uuid.UUID)
	return id
}
