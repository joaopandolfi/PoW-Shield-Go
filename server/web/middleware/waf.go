package middleware

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/models/domain"
	"regexp"
	"strings"
)

var wafHeader []domain.Waf
var wafBody []domain.Waf
var wafUrl []domain.Waf
var allWafs []domain.Waf

func loadWafList() {
	if allWafs != nil {
		return
	}
	err := json.Unmarshal([]byte(config.Get().Waf.RawWafs), &allWafs)
	if err != nil {
		log.Println("[!][Middleware][Waf] error on loading waf list: ", err.Error())
		allWafs = []domain.Waf{}
		return
	}

	for _, w := range allWafs {
		regexp, err := regexp.Compile(w.Reg)
		if err != nil {
			log.Println("[!][Middleware][Waf] error on compiling regex", w.ID)
			continue
		}
		w.Regex = regexp
		if !contains(config.Get().Waf.WhiteListBodyRules, w.ID) {
			wafBody = append(wafBody, w)
		}

		if !contains(config.Get().Waf.WhiteListHeaderRules, w.ID) {
			wafHeader = append(wafHeader, w)
		}

		if !contains(config.Get().Waf.WhiteListURLRules, w.ID) {
			wafUrl = append(wafUrl, w)
		}
	}
}

func wafDetect(match string, wafs []domain.Waf) []int {
	var result []int
	for _, v := range wafs {
		if v.Regex.MatchString(match) {
			result = append(result, v.ID)
			return result
		}
	}
	return result
}

func Waf(next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !config.Get().Waf.Active {
			next(w, r)
			return
		}

		log.Println("[.]][Middleware][WAF] ", r.URL.String())
		loadWafList()

		url := r.URL.String()
		detecteds := wafDetect(url, wafUrl)
		if len(detecteds) > 0 {
			log.Println("[*][Middleware][Waf] URL WAF RULE TRIGGERED: ", detecteds, "on: ", url)
			blockRequest(w)
			return
		}

		for k, v := range r.Header {
			header := k + ": " + strings.Join(v, ",")
			detecteds = wafDetect(header, wafHeader)
			if len(detecteds) > 0 {
				log.Println("[*][Middleware][Waf] HEADER RULE TRIGGERED: ", detecteds, "on: ", url)
				blockRequest(w)
				return
			}
		}

		// wafdetect for body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("[!][Middleware][Waf] READING BODY ERROR: ", err.Error())
			blockRequest(w)
			return
		}
		r.Body.Close()
		detecteds = wafDetect(string(body), wafBody)
		if len(detecteds) > 0 {
			log.Println("[*][Middleware][Waf] BODY RULE TRIGGERED: ", detecteds, "on: ", url)
			blockRequest(w)
			return
		}

		next(w, r)
	}
}
