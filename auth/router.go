package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine) {
	r.POST("/auth/register", RegisterHandler)
	r.POST("/auth/login", LoginHandler)
}
