package request

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var transport *http.Transport
var defaultTimeout time.Duration = time.Minute * 10

func getTransport() *http.Transport {
	if transport == nil {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		http.DefaultClient.Timeout = defaultTimeout
	}
	return transport
}

func RequestWithHeader(method, url string, head map[string]string, data []byte) ([]byte, int, http.Header, error) {
	client := &http.Client{Transport: getTransport()}

	payloadData := bytes.NewBuffer(data)

	if head["Content-Encoding"] == "gzip" {
		var compressedData bytes.Buffer
		gzipBuff := gzip.NewWriter(&compressedData)
		if _, err := gzipBuff.Write(data); err != nil {
			return nil, http.StatusExpectationFailed, nil, fmt.Errorf("gzipping body: %w", err)
		}
		gzipBuff.Close()
		payloadData = &compressedData
	}

	req, err := http.NewRequest(method, url, payloadData)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("making requester: %w", err)
	}

	//Setting Headers
	for k, v := range head {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		statusCode := http.StatusBadRequest
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return nil, statusCode, resp.Header, fmt.Errorf("[RequestWithHeader] - Error on make %s request, URL: %s, DATA: %s , ERROR: %w", method, url, string(data), err)
	}

	defer resp.Body.Close()

	var b []byte
	if resp.Uncompressed && strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		r, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, http.StatusExpectationFailed, resp.Header, fmt.Errorf("reading gzip body: %w", err)
		}
		var compressB bytes.Buffer
		_, err = compressB.ReadFrom(r)
		if err != nil {
			return nil, http.StatusExpectationFailed, resp.Header, fmt.Errorf("reading gzip bytes: %w", err)
		}
		r.Close()
		b = compressB.Bytes()

	} else {
		plainB, err := io.ReadAll(resp.Body)
		if err != nil {
			return b, resp.StatusCode, resp.Header, fmt.Errorf("[RequestWithHeader] - Error on Read Body result, URL: %s, DATA: %s , ERROR: %w", url, string(data), err)
		}
		b = plainB
	}

	return b, resp.StatusCode, resp.Header, err
}
