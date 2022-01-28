package routes

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/teandresmith/url-shortener/api/database"
	"github.com/teandresmith/url-shortener/api/helpers"
)

func SetUpRoutes(incomingRoutes *gin.Engine) {
	incomingRoutes.GET("/:id", UrlResolver())
	incomingRoutes.POST("/", UrlShortener())
}

var validate = validator.New()

type Request struct{
	URL					string			`bson:"url" json:"url" validate:"required,url"`
	CustomShortURL		string			`bson:"custom-short-url" json:"custom-short-url"`
	Expiry				time.Duration	`bson:"expiry" json:"expiry"`
}

type Response struct{
	URL						string			`bson:"url" json:"url"`
	CustomShortURL			string			`bson:"custom-short-url" json:"custom-short-url"`
	XRateLimit				int				`bson:"x-rate-limit" json:"x-rate-limit"`
	XRateResetDurationMin	time.Duration	`bson:"x-rate-reset-duration-min" json:"x-rate-reset-duration-min"`
}

func UrlShortener() gin.HandlerFunc{
	return func(c *gin.Context) {
		req := Request{}

		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "There was an error while binding the request body data.",
				"error": err.Error(),
			})
			return
		}

		if validateErr := validate.Struct(req); validateErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "There was an error while validating request body data.",
				"error": validateErr.Error(),
			})
			return
		}

		if helpers.CheckIfUrlContainsDomain(req.URL) {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "The URL cannot include the domain url",
			})
			return
		}

		if req.Expiry == 0 {
			req.Expiry = 24*time.Hour
		}

		rdb := database.CreateRedisClient(0)
		defer rdb.Close()

		numberOfCallsLeft, err := rdb.Get(database.Ctx, c.ClientIP()).Result()
		if err == redis.Nil{
			setErr := rdb.Set(database.Ctx, c.ClientIP(), 10, 30*time.Minute).Err()
			if setErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "There was an error while setting key value pair in database",
					"error": setErr.Error(),
				})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "There was an error while getting the key value pair from the database",
				"error": err.Error(),
			})
			return
		} else {
			numberOfCallsLeftInt, err := strconv.Atoi(numberOfCallsLeft)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "There was an error while parsing the limit rate to an int",
					"error": err.Error(),
				})
				return
			}
			if numberOfCallsLeftInt <= 0 {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Max API calls reached for the time period",
				})
				return
			}
		}
		
		value, err := rdb.Decr(database.Ctx, c.ClientIP()).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "There was an error while setting x-rate-limit in the database",
				"error": err.Error(),
			})
			return
		}

		id := uuid.NewString()[:5]

		timeLeft, err := rdb.TTL(database.Ctx, c.ClientIP()).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "There was an error while getting the time left until the x-rate-limit resets",
				"err": err.Error(),
			})
			return
		}

		customShortURL := os.Getenv("DOMAIN") + "/" + id
		setErr := rdb.Set(database.Ctx, customShortURL, req.URL, req.Expiry).Err()
		if setErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "There was an error while setting key-value pair in the database",
				"error": setErr.Error(),
			})
			return
		}

		resp := Response{
			URL: req.URL,
			CustomShortURL: customShortURL,
			XRateLimit: int(value),
			XRateResetDurationMin: timeLeft / time.Minute,
		}

		c.JSON(http.StatusOK, resp)
	}
}

func UrlResolver() gin.HandlerFunc{
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Please provide a parameter",
			})
			return
		}

		rdb := database.CreateRedisClient(0)
		defer rdb.Close()
		key := os.Getenv("DOMAIN") + "/" + id
		
		redirectLink, err := rdb.Get(database.Ctx, key).Result()
		if err == redis.Nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "This URL redirect was not valid.",
			})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "There was an error while querying the database",
				"error": err.Error(),
			})
			return
		}

		c.Redirect(http.StatusPermanentRedirect, redirectLink)
	}
}