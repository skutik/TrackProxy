package web_app

import (
	"html/template"
	"log"
	"net/http"
	"track_proxy/requests_storage"

	"github.com/gin-gonic/gin"
)

var lastFetchedId = ""
var activeRequestId = ""
var activeFilter = requests_storage.SearchFilter{}

var defaultRequestDetailType = requests_storage.RequestType
var FuncMap = template.FuncMap{
	"bytesToString": func(b []byte) string { return string(b) },
}

type CurlResponse struct {
	Curl string `json:"curl"`
}

func HandleIndex(c *gin.Context) {
	// tmpl, err := template.ParseFiles(
	// 	"templates/index.html",
	// 	"templates/requests_table.html",
	// 	"templates/toolbar.html",
	// )
	// if err != nil {
	// 	c.HTML(http.StatusInternalServerError, "error.html", gin.H{
	// 		"error": err.Error(),
	// 	})
	// 	return
	// }
	lastFetchedId = ""
	activeFilter.ResetFilter()

	c.HTML(http.StatusOK, "index.html", nil)

	// err = tmpl.ExecuteTemplate(w, "index.html", nil)
	// if err != nil {
	// 	log.Println("error rendering template:", err)
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }
}

func HandleRequestsTable(c *gin.Context) {
	requests, lastId := requests_storage.Storage.GetRequestSinceId("", activeFilter)
	lastFetchedId = lastId
	if len(requests) == 0 {
		c.Status(http.StatusNoContent)
		// w.WriteHeader(http.StatusNoContent)
		return
	}

	// tmpl, err := template.ParseFiles("templates/requests_table.html")
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	c.HTML(http.StatusOK, "table_content.html", requests)

	// err = tmpl.ExecuteTemplate(w, "tableContent", requests)
	// if err != nil {
	// 	log.Println("error rendering template:", err)
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }
}

func HandleRequestDetail(c *gin.Context) {
	requestId := c.Param("requestId")
	request, err := requests_storage.Storage.GetRequestById(requestId)
	if err != nil {
		log.Println("error when requesting detail for request", requestId, ":", err.Error())
		c.Status(http.StatusBadRequest)
	}

	// tmpl, err := template.ParseFiles("templates/requests_table.html")
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	c.HTML(http.StatusOK, "request_detail.html", request)

	// err = tmpl.ExecuteTemplate(w, "tableContent", requests)
	// if err != nil {
	// 	log.Println("error rendering template:", err)
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }
}

func HandleFilterRequests(c *gin.Context) {
	// err := r.ParseForm()
	activeFilter.UpdateFilters(c)
	requests, lastId := requests_storage.Storage.GetRequestSinceId("", activeFilter)
	lastFetchedId = lastId
	// if len(requests) == 0 {
	// 	w.WriteHeader(http.StatusNoContent)
	// 	return
	// }

	c.HTML(http.StatusOK, "table_content.html", requests)

	// tmpl, err := template.ParseFiles("templates/requests_table.html")
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	// err = tmpl.ExecuteTemplate(w, "tableContent", requests)
	// if err != nil {
	// 	log.Println("error rendering template:", err)
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }
}

func RegisterActiveRequest(c *gin.Context) {
	requestId := c.Param("requestId")
	if requestId == "" {
		c.Status(http.StatusInternalServerError)
	}
	activeRequestId = requestId
	request, err := requests_storage.Storage.GetRequestById(requestId)
	if err != nil {
		log.Println("error when requesting detail for request", requestId, ":", err.Error())
		c.Status(http.StatusBadRequest)
	}
	c.HTML(http.StatusOK, "request_detail.html", gin.H{
		"RequestRecord": request,
		"DetailType":    defaultRequestDetailType.String(),
	})
}

func UnregisterActiveRequest(c *gin.Context) {
	activeRequestId = ""
	c.Status(http.StatusNoContent)
}

func GetCurl(c *gin.Context) {

	var curlText string

	if activeRequestId == "" {
		curlText = ""

	} else {
		curlCmd, err := requests_storage.Storage.GetCurlForRequest(activeRequestId)
		if curlCmd == "" {
			curlText = err.Error()
		} else {
			curlText = curlCmd
		}
	}

	c.JSON(http.StatusOK, CurlResponse{Curl: curlText})
}

func RegisterRequestDetailType(c *gin.Context) {
	detailType := c.Param("detailType")

	if detailType == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	enumValue := requests_storage.GeRequestDetailTypeValue(detailType)
	if enumValue == -1 {
		c.Status(http.StatusBadRequest)
		return
	}

	defaultRequestDetailType = enumValue
	c.Status(http.StatusNoContent)
}
