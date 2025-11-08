package request_parser

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"track_proxy/requests_storage"
)

func splitLines(buffer *bytes.Buffer) ([]string, error) {
	lines := strings.Split(buffer.String(), "\r\n")
	linesCount := len(lines)
	if linesCount < 2 {
		return []string{}, fmt.Errorf("unexpected lines count %d", linesCount)
	}
	description := strings.Split(lines[0], " ")
	if len(description) < 3 {
		return []string{}, fmt.Errorf("unexpected description format '%s'", description)
	}
	return lines, nil
}

func stringDataToRecord(lines []string) *requests_storage.UnknownRecord {
	rec := &requests_storage.UnknownRecord{}
	headersEnded := false
	headers := make(map[string][]string)
	var bodyLines []string

	for _, line := range lines {
		if len(line) == 0 {
			headersEnded = true
			continue
		}

		if !headersEnded {
			headerData := strings.SplitN(line, ": ", 2)
			if len(headerData) != 2 {
				log.Println("skipped parsing of invalid header:", headerData)
				continue
			}

			headerName, headerValue := headerData[0], headerData[1]
			if headerName == "Host" {
				rec.Host = headerValue
			}
			_, ok := headers[headerName]
			if !ok {
				headers[headerName] = []string{headerValue}
			} else {
				headers[headerName] = append(headers[headerName], headerValue)
			}
		} else {
			// Accumulate all body lines
			bodyLines = append(bodyLines, line)
		}
	}

	// Join all body lines with newlines
	if len(bodyLines) > 0 {
		rec.Body = []byte(strings.Join(bodyLines, "\r\n"))
	}

	rec.Headers = headers
	return rec
}

func ParseHttpResponse(buffer *bytes.Buffer) *requests_storage.ResponseRecord {
	res := &requests_storage.ResponseRecord{}
	lines, err := splitLines(buffer)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	descParts := strings.Split(lines[0], " ")
	rec := stringDataToRecord(lines[1:])
	rec.HttpVersion = descParts[0]
	statusCode, err := strconv.Atoi(descParts[1])
	if err != nil {
		rec.Error = err.Error()
	} else {
		rec.StatusCode = statusCode
	}
	return requests_storage.ResponseRecordFromUknown(rec)
}

func ParseHttpRequest(buffer *bytes.Buffer) *requests_storage.RequestRecord {
	req := &requests_storage.RequestRecord{}
	lines, err := splitLines(buffer)
	if err != nil {
		req.Error = err.Error()
		return req
	}
	descParts := strings.Split(lines[0], " ")
	reqUrl := &url.URL{}
	reqUrl.Scheme = "https"
	reqUrl.Path = descParts[1]
	rec := stringDataToRecord(lines[1:])
	rec.HttpVersion = descParts[2]
	rec.Method = descParts[0]
	reqUrl.Host = rec.Host

	rec.Url = reqUrl.String()
	return requests_storage.RequestRecordFromUknown(rec)
}
