package requests_storage

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"track_proxy/client_hello"

	http "github.com/bogdanfinn/fhttp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	// Lock   sync.Mutex
	RwLock sync.RWMutex
)
var Storage = make(RequestStorage)

type RequestDetailType int

const (
	RequestType RequestDetailType = iota
	ResponseType
)

var requestDetailTypeNaming = [...]string{"request", "response"}

func (rdt RequestDetailType) String() string {
	return requestDetailTypeNaming[rdt]
}

func GeRequestDetailTypeValue(detailType string) RequestDetailType {
	detailType = strings.ToLower(detailType)
	for idx, typeNaming := range requestDetailTypeNaming {
		if typeNaming == detailType {
			return RequestDetailType(idx)
		}
	}
	return -1
}

type RequestStorage map[string]Request

type RequestError struct{}

type RequestRecord struct {
	Method             string                       `json:"method"`
	HttpVersion        string                       `json:"httpVersion"`
	Url                string                       `json:"url"`
	Headers            map[string][]string          `json:"headers"`
	Host               string                       `json:"host"`
	Body               []byte                       `json:"body"`
	StartTimestamp     int64                        `json:"startTimestamp"`
	FinishTimestamp    int64                        `json:"finishTimestamp"`
	Schema             string                       `json:"schema"`
	HttpSetting        map[string]string            `json:"httpSetting"`
	HttpWindowUpdate   int                          `json:"httpWindowUpdate"`
	ClientHello        client_hello.ClientHelloData `json:"clientHello"`
	HeadersOrder       []string                     `json:"headersOrder"`
	PseudoHeadersOrder []string                     `json:"presudoHeadersOrder"`
	Error              string                       `json:"error"`
}

type ResponseRecord struct {
	StatusCode  int                 `json:"statusCode"`
	HttpVersion string              `json:"httpVersion"`
	Headers     map[string][]string `json:"headers"`
	Body        []byte              `json:"body"`
	Error       string              `json:"error"`
}

type ProxyInfo struct {
	Host    string `json:"host"`
	SrcAddr string `json:"srcAddr"`
	DstAddr string `json:"dstAddr"`
}

type Request struct {
	Id        string         `json:"id"`
	Request   RequestRecord  `json:"request"`
	Response  ResponseRecord `json:"response"`
	ProxyInfo ProxyInfo      `json:"proxy"`
}

type UnknownRecord struct {
	Method             string
	HttpVersion        string
	Url                string
	Headers            map[string][]string
	Host               string
	Body               []byte
	StartTimestamp     int64
	FinishTimestamp    int64
	Schema             string
	HttpSetting        map[string]string
	HttpWindowUpdate   int
	HeadersOrder       []string
	PseudoHeadersOrder []string
	StatusCode         int
	Error              string
}

type SearchFilter struct {
	Phrase        string
	SearchUrl     bool
	SearchHeaders bool
	SearchBody    bool
}

func NewRequest() Request {
	req := Request{}
	req.Id = uuid.New().String()
	req.Request.StartTimestamp = time.Now().UnixNano()
	return req
}

func (reqStorage RequestStorage) ClearStorage() {
	RwLock.Lock()
	defer RwLock.Unlock()
	for key := range reqStorage {
		delete(reqStorage, key)
	}
}

func (reqStorage RequestStorage) AddRequestToStorage(req Request) error {
	if len(req.Id) == 0 {
		req.Id = uuid.New().String()
	}

	RwLock.Lock()
	defer RwLock.Unlock()

	_, exists := reqStorage[req.Id]
	if exists {
		return fmt.Errorf("request ID %s is already included in storage", req.Id)
	}
	reqStorage[req.Id] = req
	return nil
}

func (reqStorage RequestStorage) UpdateExistingRequest(req Request) bool {
	RwLock.Lock()
	defer RwLock.Unlock()
	if _, exists := reqStorage[req.Id]; !exists {
		return false
	}
	reqStorage[req.Id] = req
	return true
}

func (reqStorage RequestStorage) CreateRecordWithProxyInfo(proxyInfo ProxyInfo) Request {
	req := NewRequest()
	req.ProxyInfo = proxyInfo
	reqStorage.AddRequestToStorage(req)
	return req
}
func (reqStorage RequestStorage) GetRequests(filter SearchFilter) []Request {
	requests, _ := reqStorage.GetRequestSinceId("", filter)
	return requests
}

func (reqStorage RequestStorage) applySearchFilter(requests []Request, filter SearchFilter) []Request {

	if filter.Phrase == "" || (!filter.SearchBody && !filter.SearchHeaders && !filter.SearchUrl) {
		return requests
	}

	filteredRequests := []Request{}

	for _, request := range requests {
		if filter.SearchUrl {
			if !strings.Contains(string(request.Request.Url), filter.Phrase) {
				continue
			}
		}

		if filter.SearchBody {
			if !strings.Contains(string(request.Request.Body), filter.Phrase) && !strings.Contains(string(request.Response.Body), filter.Phrase) {
				continue
			}
		}

		if filter.SearchHeaders {
			if !reqStorage.headersConstainsSubstring(request.Request.Headers, filter.Phrase) && !reqStorage.headersConstainsSubstring(request.Response.Headers, filter.Phrase) {
				continue
			}
		}

		filteredRequests = append(filteredRequests, request)
	}

	return filteredRequests
}

func (reqStorage RequestStorage) headersConstainsSubstring(headers map[string][]string, subString string) bool {
	for headerName, headerValues := range headers {
		if strings.Contains(headerName, subString) {
			return true
		}

		for _, headerValue := range headerValues {
			if strings.Contains(headerValue, subString) {
				return true
			}
		}
	}

	return false
}

func (reqStorage RequestStorage) GetRequestSinceId(lastId string, filter SearchFilter) ([]Request, string) {
	requests := []Request{}
	request := Request{}
	i := 0

	RwLock.RLock()
	defer RwLock.RUnlock()

	for _, request := range reqStorage {
		requests = append(requests, request)
	}

	if len(requests) == 0 {
		return requests, ""
	}

	sort.Slice(requests, func(i, j int) bool {
		return requests[i].Request.StartTimestamp < requests[j].Request.StartTimestamp
	})

	if len(lastId) != 0 {
		for i, request = range requests {
			if request.Id == lastId {
				break
			}

		}
		if len(requests) == (i + 1) {
			return []Request{}, requests[len(requests)-1].Id
		}
		requests = requests[i+1:]

	}

	filteredRequests := reqStorage.applySearchFilter(requests, filter)
	if len(filteredRequests) > 0 {
		return filteredRequests, filteredRequests[len(filteredRequests)-1].Id
	}
	return filteredRequests, ""

}

func (reqStorage RequestStorage) GetRequestById(reqId string) (Request, error) {
	RwLock.RLock()
	defer RwLock.RUnlock()
	req, exists := reqStorage[reqId]
	if !exists {
		return Request{}, fmt.Errorf("request with ID %s not found", reqId)
	}
	return req, nil

}

func (reqStorage RequestStorage) GetCurlForRequest(reqId string) (string, error) {
	req, err := reqStorage.GetRequestById(reqId)
	if err != nil {
		return "", err
	}

	return req.Request.GetCurlCommand(), nil
}

func ResponseRecordFromUknown(unknownRecord *UnknownRecord) *ResponseRecord {
	return &ResponseRecord{
		StatusCode:  unknownRecord.StatusCode,
		HttpVersion: unknownRecord.HttpVersion,
		Headers:     unknownRecord.Headers,
		Body:        unknownRecord.Body,
		Error:       unknownRecord.Error,
	}
}

func RequestRecordFromUknown(unknownRecord *UnknownRecord) *RequestRecord {

	return &RequestRecord{
		Method:             unknownRecord.Method,
		HttpVersion:        unknownRecord.HttpVersion,
		Url:                unknownRecord.Url,
		Headers:            unknownRecord.Headers,
		Host:               unknownRecord.Host,
		Body:               unknownRecord.Body,
		Schema:             unknownRecord.Schema,
		HttpSetting:        unknownRecord.HttpSetting,
		HttpWindowUpdate:   unknownRecord.HttpWindowUpdate,
		HeadersOrder:       unknownRecord.HeadersOrder,
		PseudoHeadersOrder: unknownRecord.PseudoHeadersOrder,
		Error:              unknownRecord.Error,
	}
}

// TODO: Move request process to connection handler
func (req *RequestRecord) ProcessRequest() (*http.Response, error) {

	emptyResp := http.Response{}

	// transport := &http.Transport{}
	c := &http.Client{
		// Transport: transport,
	}

	bodyReader := bytes.NewReader(req.Body)
	r, err := http.NewRequest(req.Method, req.Url, bodyReader)
	if err != nil {
		return &emptyResp, fmt.Errorf("Cannot create new request for %s %s [%s]", req.Method, req.Url, req.HttpVersion)
	}

	r.Header = req.Headers
	r.Header[http.HeaderOrderKey] = req.HeadersOrder
	r.Header[http.PHeaderOrderKey] = req.PseudoHeadersOrder

	req.StartTimestamp = time.Now().UnixNano()
	resp, err := c.Do(r)
	req.FinishTimestamp = time.Now().UnixNano()
	if err != nil {
		return &emptyResp, fmt.Errorf("Error when processing request %s %s [%s]", req.Method, req.Url, req.HttpVersion)
	}
	return resp, nil
}

func (req *RequestRecord) GetCurlCommand() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("curl -X %s '%s'", req.Method, req.Url))
	for headerName, headerValues := range req.Headers {
		if strings.HasPrefix(headerName, ":") {
			continue
		}

		for _, headerValue := range headerValues {
			builder.WriteString(fmt.Sprintf(" -H '%s: %s'", headerName, headerValue))
		}
	}

	if len(req.Body) > 0 {
		builder.WriteString(fmt.Sprintf(" --data '%s'", req.Body))
	}

	curlHttpVersion := strings.Replace(req.HttpVersion, "/", "", 1)
	builder.WriteString(fmt.Sprintf(" --%s", strings.ToLower(curlHttpVersion)))
	return builder.String()
}

func (s *SearchFilter) formCheckboxToBool(value string) bool {
	return value == "on"
}

func (s *SearchFilter) ResetFilter() {
	s.Phrase = ""
	s.SearchBody = false
	s.SearchHeaders = false
	s.SearchUrl = false
}

func (s *SearchFilter) UpdateFilters(c *gin.Context) error {
	formValues := []string{"textFilter", "url", "headers", "body"}
	s.ResetFilter()
	for _, formValue := range formValues {
		value := c.PostForm(formValue)
		if value == "" {
			continue
		}

		switch formValue {
		case "textFilter":
			s.Phrase = value

		case "url":
			s.SearchUrl = s.formCheckboxToBool(value)
		case "headers":
			s.SearchHeaders = s.formCheckboxToBool(value)
		case "body":
			s.SearchBody = s.formCheckboxToBool(value)
		default:
			log.Println("unknown value")
		}
	}
	return nil
}

// func (s *SearchFilter) UpdateFilters(form *url.Values) error {
// 	formValues := []string{"textFilter", "url", "headers", "body"}
// 	s.ResetFilter()
// 	for _, formValue := range formValues {
// 		value := form.Get(formValue)
// 		if value == "" {
// 			continue
// 		}

// 		switch formValue {
// 		case "textFilter":
// 			s.Phrase = value

// 		case "url":
// 			s.SearchUrl = s.formCheckboxToBool(value)
// 		case "headers":
// 			s.SearchHeaders = s.formCheckboxToBool(value)
// 		case "body":
// 			s.SearchBody = s.formCheckboxToBool(value)
// 		default:
// 			log.Println("unknown value")
// 		}
// 	}
// 	return nil
// }

// func ResponseRecordFromResponse(res *Response) ResponseRecord {
// 	responseRecord := ResponseRecord{}
// 	responseRecord.StatusCode = res.StatusCode

// 	responseRecord.Headers = make(map[string][]string)
// 	for headerName, headerValue := range res.Header {
// 		responseRecord.Headers[headerName] = headerValue
// 	}
// 	defer res.Body.Close()

// 	bodyBytes, err := io.ReadAll(res.Body)
// 	if err != nil {
// 		log.Println("Error reading response body", err)
// 	}

// 	responseRecord.HttpVersion = res.Proto
// 	responseRecord.Body = bodyBytes
// 	return responseRecord
// }

// func RequestFromFrames(frames []frames_parser.Http2Frame) (RequestType, error) {
// 	parsedRequest := RequestType{}
// 	for frame := range frames {
// 		log.Println("Processing frame: ", frame)
// 	}

// 	return parsedRequest, fmt.Errorf("unable to parse request from frames")
// }
