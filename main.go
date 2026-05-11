package main

import (
	"fmt"
	"nuvolaQuest/health"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	health.RegisterRoutes(r)

	return r
}

func main() {
	r := SetupRouter()

	fmt.Println("🚀 Starting Gin server on http://localhost:8080...")

	err := r.Run()
	if err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
		return
	}
}
