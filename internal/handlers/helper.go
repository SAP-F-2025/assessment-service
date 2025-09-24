package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ParseStringIDParam(c *gin.Context, param string) string {
	idStr := c.Param(param)
	idStr = strings.TrimSpace(idStr)
	if idStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid " + param,
			Details: "ID cannot be empty",
		})
		return ""
	}
	return idStr
}
