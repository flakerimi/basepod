package main

import (
	"net/http"
	"net/http/cookiejar"
	"time"
)

func defaultHTTP() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

func defaultHTTPWithJar() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Timeout: 60 * time.Second, Jar: jar}
}
