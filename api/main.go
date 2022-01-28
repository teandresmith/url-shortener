package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/teandresmith/url-shortener/api/routes"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Panic(err)
	}
	router := gin.Default()
	routes.SetUpRoutes(router)

	
	log.Fatal(router.Run(":8080"))
}