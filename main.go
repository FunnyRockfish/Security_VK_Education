package main

import (
	"fmt"
	"log"
	"net/http"
	url2 "net/url"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func main() {
	CreateServer()
}

func CreateServer() {
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(HandleProxy)
	fmt.Println("Starting proxy server on port 8080")
	err := http.ListenAndServe(":8080", router)
	if err != nil {
		log.Fatal(err)
	}
}

func HandleProxy(w http.ResponseWriter, r *http.Request) {
	logger := CreateLogger()
	logger.Info(r)
	url, err := url2.Parse(r.RequestURI)
	if err != nil {
		logger.Error(err)
	}
	logger.Info("Proxying request to URL: ", url.String()[0:len(url.String())])
	client := &http.Client{}
	proxyReq, err := http.NewRequest(r.Method, url.String()[0:len(url.String())], r.Body)
	if err != nil {
		logger.Error("Failed to create request:")
		logger.Error(err)
	}

	proxyReq.Header = r.Header
	resp, err := client.Do(proxyReq)
	if err != nil {
		logger.Error("Failed to do request:")
		logger.Error(err)
	}
	defer resp.Body.Close()

	//fmt.Println(resp)
}

func CreateLogger() *zap.SugaredLogger {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	sugar := logger.Sugar()
	return sugar
}
