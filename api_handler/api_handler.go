package api_handler

import (
	"log"
	"net/http"
	"track_proxy/requests_storage"

	"github.com/gin-gonic/gin"
)

const REQUESTS_STORAGE = "requests_storage"

type ApiError struct {
	Message string `json:"message"`
}

func GetRequests(c *gin.Context) {
	requests := requests_storage.Storage.GetRequests(requests_storage.SearchFilter{})
	log.Println("found requests in storage:", len(requests))
	c.JSON(http.StatusOK, requests)
}

func GetRequestById(c *gin.Context) {
	reqId := c.Param("requestId")
	request, err := requests_storage.Storage.GetRequestById(reqId)
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, request)
}

func GetCurl(c *gin.Context) {
	reqId := c.Param("requestId")
	request, err := requests_storage.Storage.GetCurlForRequest(reqId)
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, request)
}

func Clear(c *gin.Context) {
	requests_storage.Storage.ClearStorage()
	c.JSON(http.StatusOK, nil)
}

func Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ping": "pong",
	})
}
