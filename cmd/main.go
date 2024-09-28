package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"hw2/database"
	rest "hw2/internal/REST"
	p "hw2/internal/proxy"
)

func main() {
	db := database.ConnectToMongoDataBase()
	pHandler := p.NewProxyStorage(db)
	rHandler := rest.NewRequestHandler(db, *pHandler)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		createProxyServer(db, pHandler)
	}()

	go func() {
		defer wg.Done()
		createAPIServer(db, rHandler)
	}()

	wg.Wait()
}

func createProxyServer(db *mongo.Database, ps *p.ProxyStorage) {

	server := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(ps.HandleProxy),
	}
	fmt.Println("Proxy-сервер запущен на порту 8080")
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}

func createAPIServer(db *mongo.Database, rh *rest.RequestHandler) {
	r := mux.NewRouter()
	r.HandleFunc("/requests", rh.HandleRequests).Methods("GET")
	r.HandleFunc("/requests/{id}", rh.HandleRequestByID).Methods("GET")
	r.HandleFunc("/repeat/{id}", rh.HandleRepeatRequest).Methods("GET")
	//r.HandleFunc("/scan/{id}", rest.HandleRepeatRequest).Methods("GET")

	log.Println("API запущен на порту :8000")
	log.Fatal(http.ListenAndServe(":8000", r))
}
