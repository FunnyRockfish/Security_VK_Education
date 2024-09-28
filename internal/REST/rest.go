package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"hw2/domain"
	"hw2/internal/proxy"
)

type RequestHandler struct {
	db *mongo.Database
	p  proxy.ProxyStorage
}

func NewRequestHandler(db *mongo.Database, storage proxy.ProxyStorage) *RequestHandler {
	return &RequestHandler{
		db: db,
		p:  storage,
	}
}

func (rh *RequestHandler) HandleRequests(w http.ResponseWriter, r *http.Request) {
	collection := rh.db.Collection("request_response")
	cursor, err := collection.Find(context.TODO(), bson.D{})
	if err != nil {
		log.Fatal("Ошибка при получении документов:", err)
	}
	defer cursor.Close(context.TODO())

	var requests []domain.Request
	for cursor.Next(context.TODO()) {
		var reqResp domain.ReqResp
		err = cursor.Decode(&reqResp)
		if err != nil {
			log.Println("Ошибка при декодировании документа:", err)
		}
		reqResp.Req.ID = reqResp.ID
		requests = append(requests, reqResp.Req)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(requests)
	if err != nil {
		http.Error(w, "Ошибка при кодировании ответа в JSON", http.StatusInternalServerError)
		return
	}
}

func (rh *RequestHandler) HandleRequestByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/requests/")
	objectID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Некорректный формат ID", http.StatusBadRequest)
		return
	}

	collection := rh.db.Collection("request_response")

	filter := bson.M{"_id": objectID}
	var reqResp domain.ReqResp
	err = collection.FindOne(context.TODO(), filter).Decode(&reqResp)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Запрос не найден", http.StatusNotFound)
		} else {
			log.Println("Ошибка при получении документа:", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		}
		return
	}
	reqResp.Req.ID = reqResp.ID

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(reqResp.Req)
	if err != nil {
		http.Error(w, "Ошибка при кодировании ответа в JSON", http.StatusInternalServerError)
		return
	}
}

func (rh *RequestHandler) HandleRepeatRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	fmt.Println(idStr)

	resp := rh.p.RepeatRequest(idStr)
	w.WriteHeader(resp.StatusCode)

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	_, err := io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "Failed to copy response body", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
}
