package handlers

import (
	"net/http"

	"github.com/SAP-F-2025/assessment-service/internal/config"
	"github.com/gin-gonic/gin"
)

// ServiceAuthMiddleware provides authentication for internal services
type ServiceAuthMiddleware struct {
	config           config.ServiceAuthConfig
	validServiceKeys map[string]string // key -> service name
}

// NewServiceAuthMiddleware creates a new service authentication middleware
func NewServiceAuthMiddleware(cfg config.ServiceAuthConfig) *ServiceAuthMiddleware {
	return &ServiceAuthMiddleware{
		config:           cfg,
		validServiceKeys: cfg.Keys,
	}
}

// ServiceAuthOrJWTMiddleware allows either:
// 1. X-Service-Key header for internal service calls
// 2. Bearer JWT token for user calls (handled by CasdoorAuthMiddleware)
func (m *ServiceAuthMiddleware) ServiceAuthOrJWTMiddleware(casdoorAuth *CasdoorAuthMiddleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if service auth is enabled
		if !m.config.Enabled {
			// Fall back to JWT auth
			casdoorAuth.AuthMiddleware()(c)
			return
		}

		// Check for service key first
		serviceKey := c.GetHeader("X-Service-Key")
		if serviceKey != "" {
			if serviceName, ok := m.validServiceKeys[serviceKey]; ok {
				// Valid service key - set context and continue
				c.Set("service_name", serviceName)
				c.Set("is_service_call", true)
				c.Set("user_id", "system:"+serviceName) // Use service name as user_id for logging
				c.Next()
				return
			}

			// Invalid service key
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid service key",
			})
			return
		}

		// No service key, fall back to JWT auth
		casdoorAuth.AuthMiddleware()(c)
	}
}

// OptionalServiceAuthMiddleware checks for service key without requiring it
func (m *ServiceAuthMiddleware) OptionalServiceAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Enabled {
			c.Next()
			return
		}

		serviceKey := c.GetHeader("X-Service-Key")
		if serviceKey != "" {
			if serviceName, ok := m.validServiceKeys[serviceKey]; ok {
				c.Set("service_name", serviceName)
				c.Set("is_service_call", true)
			}
		}
		c.Next()
	}
}

// IsServiceCall checks if the current request is from an internal service
func IsServiceCall(c *gin.Context) bool {
	isService, exists := c.Get("is_service_call")
	if !exists {
		return false
	}
	return isService.(bool)
}

// GetServiceName gets the calling service name from context
func GetServiceName(c *gin.Context) string {
	serviceName, exists := c.Get("service_name")
	if !exists {
		return ""
	}
	name, ok := serviceName.(string)
	if !ok {
		return ""
	}
	return name
}
