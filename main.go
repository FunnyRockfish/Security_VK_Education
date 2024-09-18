package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

func CreateProxyServer() {
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(handleProxy)
	fmt.Println("Proxy-сервер запущен на порту 8081")
	err := http.ListenAndServe(":8081", router)
	if err != nil {
		log.Fatal("Ошибка при запуске сервера:", err)
	}
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	targetURL := r.RequestURI
	if targetURL == "" {
		http.Error(w, "Целевой URL не указан", http.StatusBadRequest)
		return
	}

	parsedTargetUrl, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Неправильный URL", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Ошибка при создании запроса", http.StatusInternalServerError)
		log.Println("Ошибка при создании запроса:", err)
		return
	}

	req.Header = r.Header.Clone()
	req.Host = parsedTargetUrl.Host

	req.Header.Del("Proxy-Connection")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Ошибка при отправке запроса на целевой сервер", http.StatusBadGateway)
		log.Println("Ошибка при отправке запроса на сервер:", err)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Println("Ошибка при отправке ответа клиенту:", err)
	}
}

func main() {
	CreateProxyServer()
}
