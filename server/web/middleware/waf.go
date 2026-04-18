package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/metrics"
	"pow-shield-go/models/domain"
	"regexp"
	"strconv"
	"strings"
)

var wafHeader []domain.Waf
var wafBody []domain.Waf
var wafUrl []domain.Waf
var allWafs []domain.Waf
var wafTypes map[int]string

func InitWaf() {
	if !config.Get().Waf.Active {
		return
	}
	if allWafs != nil {
		return
	}
	err := json.Unmarshal([]byte(config.Get().Waf.RawWafs), &allWafs)
	if err != nil {
		log.Println("[!][Middleware][Waf] error on loading waf list: ", err.Error())
		allWafs = []domain.Waf{}
		return
	}
	wafTypes = map[int]string{}
	if err := json.Unmarshal([]byte(config.Get().Waf.RawWafTypes), &wafTypes); err != nil {
		rawTypes := map[string]string{}
		if err := json.Unmarshal([]byte(config.Get().Waf.RawWafTypes), &rawTypes); err == nil {
			for k, v := range rawTypes {
				id, convErr := strconv.Atoi(k)
				if convErr == nil {
					wafTypes[id] = v
				}
			}
		}
	}

	for _, w := range allWafs {
		regexp, err := regexp.Compile(w.Reg)
		if err != nil {
			log.Println("[!][Middleware][Waf] error on compiling regex", w.ID)
			continue
		}
		w.Regex = regexp
		w.TypeName = wafTypes[w.Type]
		if !allowWafType(w.TypeName) {
			continue
		}
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

func allowWafType(typeName string) bool {
	cfg := config.Get().Waf
	if len(cfg.AllowTypes) > 0 {
		allowed := false
		for _, allowedType := range cfg.AllowTypes {
			if strings.EqualFold(allowedType, typeName) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	for _, blockedType := range cfg.BlockTypes {
		if strings.EqualFold(blockedType, typeName) {
			return false
		}
	}

	return true
}

func wafDetect(match string, wafs []domain.Waf) []domain.Waf {
	var result []domain.Waf
	for _, v := range wafs {
		if v.Regex.MatchString(match) {
			result = append(result, v)
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

		log.Println("[.][Middleware][WAF] ", r.URL.String())

		url := r.URL.String()
		detecteds := wafDetect(url, wafUrl)
		if len(detecteds) > 0 {
			metrics.IncWAFBlocked("url", detecteds[0].TypeName)
			log.Println("[*][Middleware][Waf] URL WAF RULE TRIGGERED:", detecteds[0].ID, detecteds[0].TypeName, "on:", url)
			blockRequest(w)
			return
		}

		for k, v := range r.Header {
			header := k + ": " + strings.Join(v, ",")
			detecteds = wafDetect(header, wafHeader)
			if len(detecteds) > 0 {
				metrics.IncWAFBlocked("header", detecteds[0].TypeName)
				log.Println("[*][Middleware][Waf] HEADER RULE TRIGGERED:", detecteds[0].ID, detecteds[0].TypeName, "on:", url)
				blockRequest(w)
				return
			}
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("[!][Middleware][Waf] READING BODY ERROR: ", err.Error())
			blockRequest(w)
			return
		}
		r.Body.Close()
		detecteds = wafDetect(string(body), wafBody)
		if len(detecteds) > 0 {
			metrics.IncWAFBlocked("body", detecteds[0].TypeName)
			log.Println("[*][Middleware][Waf] BODY RULE TRIGGERED:", detecteds[0].ID, detecteds[0].TypeName, "on:", url)
			blockRequest(w)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		next(w, r)
	}
}
