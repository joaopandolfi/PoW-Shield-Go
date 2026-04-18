package handler

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"log"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/web"
	"strings"
)

// marshaler
var marshaler func(v interface{}) ([]byte, error) = json.Marshal

var activeZipOnResponse bool = false

// ActiveZipOnResponse active compression protocol on response payload
func ActiveZipOnResponse() {
	activeZipOnResponse = true
}

func RespondDefaultError(w http.ResponseWriter, status int) {
	RespondJson(w, web.DefaultErrorForStatus(status), status)
}

func RespondBlock(w http.ResponseWriter) {
	cfg := config.Get().Reply
	w.Header().Set("Content-Type", cfg.BlockContentType)
	w.WriteHeader(cfg.BlockStatus)
	w.Write([]byte(cfg.BlockBody))
}

// RespondJson - send json result
func RespondJson(w http.ResponseWriter, resp interface{}, status int) {
	w.Header().Add("Content-Type", "application/json")

	b, err := marshaler(resp)
	if err != nil {
		log.Printf("RespondJson: marshaling response payload: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error on marshal"))
		return
	}
	if activeZipOnResponse && strings.Contains(w.Header().Get("Accept-Encoding"), "gzip") {
		w.Header().Add("Content-Encoding", "gzip")
		var compressedData bytes.Buffer
		gzipBuff := gzip.NewWriter(&compressedData)
		defer gzipBuff.Close()
		if _, err := gzipBuff.Write(b); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(""))
			return
		}

		b = compressedData.Bytes()
	}

	w.WriteHeader(status)
	w.Write(b)
}
