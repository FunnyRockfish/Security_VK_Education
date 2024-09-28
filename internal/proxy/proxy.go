package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"hw2/certificate"
	"hw2/domain"
)

// Структура для хранения параметров ответа
type ParsedResponse struct {
	Code    int               `bson:"code"`
	Message string            `bson:"message"`
	Headers map[string]string `bson:"headers"`
	Body    string            `bson:"body"`
}

type ProxyStorage struct {
	db *mongo.Database
}

func NewProxyStorage(db *mongo.Database) *ProxyStorage {
	return &ProxyStorage{
		db: db,
	}
}

func (p *ProxyStorage) HandleProxy(w http.ResponseWriter, r *http.Request) {
	log.Printf("(proxy-server) Received request method: %s, host: %s, URL: %s", r.Method, r.Host, r.URL.String())
	// Определяем, является ли запрос CONNECT (для HTTPS)
	if r.Method == http.MethodConnect {
		p.handleHTTPS(w, r) // Вызов функции для обработки HTTPS-запросов
	} else {
		p.handleHTTP(w, r) // Вызов функции для обработки HTTP-запросов
	}
}

// Обработка HTTPS-запросов
func (p *ProxyStorage) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	log.Printf("(proxy-server) Handling HTTPS request: %s, host: %s, URL: %s", r.Method, r.Host, r.URL.String())

	hostPort := r.Host
	if !strings.Contains(hostPort, ":") {
		hostPort += ":443" // Добавляем порт по умолчанию
	}
	host, _, err := net.SplitHostPort(hostPort)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Error hijacking connection", http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	_, err = clientConn.Write([]byte("HTTP/1.0 200 Connection established\r\n\r\n"))
	if err != nil {
		log.Println("(proxy-server) Failed to send connection established:", err)
		return
	}

	serverConn, err := net.Dial("tcp", hostPort)
	if err != nil {
		log.Println("(proxy-server) Failed to connect to destination:", err)
		return
	}
	defer serverConn.Close()

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Printf("(proxy-server) Error generating serial number: %v\n", err)
		return
	}

	certFile := fmt.Sprintf("certs/%s.crt", host)
	certKey := fmt.Sprintf("certs/%s.key", host)

	if _, err = os.Stat(certFile); os.IsNotExist(err) {
		log.Printf("(proxy-server) Certificate file %s does not exist", certFile)
		err = certificate.GenerateHostCertificate(host, serialNumber)
		if err != nil {
			log.Println("(proxy-server) Failed to generate host certificate:", err)
			return
		}
	}

	cert, err := tls.LoadX509KeyPair(certFile, certKey)
	if err != nil {
		log.Printf("(proxy-server) Failed to load certificate from %s and key from %s: %v", certFile, certKey, err)
		return
	}
	log.Printf("(proxy-server) Successfully loaded certificate from %s and key from %s", certFile, certKey)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	tlsClientConn := tls.Server(clientConn, tlsConfig)
	err = tlsClientConn.Handshake()
	if err != nil {
		log.Println("(proxy-server) TLS handshake with client failed:", err)
		return
	}
	defer tlsClientConn.Close()

	tlsServerConn := tls.Client(serverConn, &tls.Config{InsecureSkipVerify: true})
	err = tlsServerConn.Handshake()
	if err != nil {
		log.Println("(proxy-server) TLS handshake with server failed:", err)
		return
	}
	defer tlsServerConn.Close()

	// Парсинг запроса клиента
	clientReader := bufio.NewReader(tlsClientConn)
	req, err := http.ReadRequest(clientReader)
	if err != nil {
		log.Println("(proxy-server) Failed to read client request:", err)
		return
	}
	parsedReq := parseRequest(req)

	// Отправка запроса на сервер
	err = req.Write(tlsServerConn)
	if err != nil {
		log.Println("(proxy-server) Failed to forward request to server:", err)
		return
	}

	// Парсинг ответа от сервера
	serverReader := bufio.NewReader(tlsServerConn)
	resp, err := http.ReadResponse(serverReader, req)
	if err != nil {
		log.Println("(proxy-server) Failed to read server response:", err)
		return
	}
	defer resp.Body.Close()

	parsedResp := parseResponse(resp)

	// Сохранение запроса и ответа в базе данных
	p.saveRequestResponse(parsedReq, parsedResp)

	// Отправка ответа клиенту
	resp.Write(tlsClientConn)
}

// Функция для парсинга HTTP-запроса
func parseRequest(r *http.Request) domain.Request {
	getParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		getParams[key] = ensureValidUTF8(values[0])
	}

	postParams := make(map[string]string)
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if r.Method == "POST" || r.Method == "PUT" {
		if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
			r.ParseForm()
			for key, values := range r.PostForm {
				postParams[key] = ensureValidUTF8(values[0])
			}
		}
	}

	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[ensureValidUTF8(name)] = ensureValidUTF8(strings.Join(values, ", "))
	}

	cookies := make(map[string]string)
	for _, cookie := range r.Cookies() {
		cookies[ensureValidUTF8(cookie.Name)] = ensureValidUTF8(cookie.Value)
	}

	return domain.Request{
		Method:     ensureValidUTF8(r.Method),
		Path:       ensureValidUTF8(r.URL.Path),
		GetParams:  getParams,
		Headers:    headers,
		Cookies:    cookies,
		PostParams: postParams,
		Body:       ensureValidUTF8(string(bodyBytes)),
	}
}

// Функция для парсинга HTTP-ответа
func parseResponse(resp *http.Response) domain.Response {
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Распаковка gzip, если это необходимо
	if resp.Header.Get("Content-Encoding") == "gzip" {
		bodyBytes, _ = decodeGzip(bodyBytes)
	}

	headers := make(map[string]string)
	for name, values := range resp.Header {
		headers[ensureValidUTF8(name)] = ensureValidUTF8(strings.Join(values, ", "))
	}

	return domain.Response{
		Code:    resp.StatusCode,
		Message: ensureValidUTF8(resp.Status),
		Headers: headers,
		Body:    ensureValidUTF8(string(bodyBytes)),
	}
}

// Сохранение запроса и ответа в базе данных
func (p *ProxyStorage) saveRequestResponse(req domain.Request, resp domain.Response) {
	fmt.Println("REQ:", req)
	fmt.Println("RESP:", resp)

	storedReq := domain.ReqResp{
		Req:  req,
		Resp: resp,
	}
	collection := p.db.Collection("request_response")
	_, err := collection.InsertOne(context.TODO(), storedReq)
	if err != nil {
		log.Println("Ошибка при сохранении запроса и ответа:", err)
	} else {
		log.Println("Запрос и ответ успешно сохранены")
	}
}

// Обработка HTTP-запросов
func (p *ProxyStorage) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Логируем запрос
	log.Printf("(proxy-server) Handling HTTP request: %s, host: %s, URL: %s", r.Method, r.Host, r.URL.String())

	uri, err := url.Parse(r.RequestURI)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	host := uri.Host
	if host == "" {
		host = r.Host
	}
	if !strings.Contains(host, ":") {
		host += ":80"
	}

	conn, err := net.Dial("tcp", host)
	if err != nil {
		http.Error(w, "Error connecting to host", http.StatusBadGateway)
		return
	}
	defer conn.Close()

	requestLine := fmt.Sprintf("%s %s %s\r\n", r.Method, uri.RequestURI(), r.Proto)
	conn.Write([]byte(requestLine))

	for header, values := range r.Header {
		if strings.EqualFold(header, "Proxy-Connection") {
			continue
		}
		for _, value := range values {
			conn.Write([]byte(fmt.Sprintf("%s: %s\r\n", header, value)))
		}
	}

	conn.Write([]byte(fmt.Sprintf("Host: %s\r\n", host)))
	conn.Write([]byte("\r\n"))

	if r.Method == "POST" || r.Method == "PUT" {
		io.Copy(conn, r.Body)
	}

	respReader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(respReader, r)
	if err != nil {
		http.Error(w, "Error reading response", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	//storeRequest(r, resp)

	PutItemToDatabase(p.db, r, resp)

	for header, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// Функция для сохранения запроса и ответа в MongoDB
func storeRequest(r *http.Request, resp *http.Response) domain.ReqResp {
	// Парсинг GET параметров
	queryParams := r.URL.Query()
	getParams := make(map[string]string)
	for key, values := range queryParams {
		getParams[key] = values[0] // Берем первое значение, если параметр встречается несколько раз
	}

	// Парсинг POST параметров
	postParams := make(map[string]string)
	bodyBytes, err := io.ReadAll(r.Body) // Считываем тело запроса
	if err == nil {
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes))) // Восстанавливаем r.Body для последующего использования
	}

	if r.Method == "POST" || r.Method == "PUT" {
		// Если тип данных - application/x-www-form-urlencoded, парсим форму
		if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
			err := r.ParseForm()
			if err == nil {
				for key, values := range r.PostForm {
					postParams[key] = values[0] // Берем первое значение
				}
			}
		}
	}

	// Парсинг заголовков
	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[name] = strings.Join(values, ", ") // Объединяем несколько значений заголовка в одну строку
	}

	// Парсинг Cookie
	cookieParams := make(map[string]string)
	for _, cookie := range r.Cookies() {
		cookieParams[cookie.Name] = cookie.Value
	}

	parsedReq := domain.Request{
		Method:     r.Method,
		Path:       r.URL.Path,
		GetParams:  getParams,
		Headers:    headers,
		Cookies:    cookieParams,
		PostParams: postParams,
		Body:       string(bodyBytes), // Сохраняем тело запроса
	}

	parsedResp := domain.Response{}
	if resp != nil {
		// Парсинг заголовков ответа
		responseHeaders := make(map[string]string)
		for name, values := range resp.Header {
			responseHeaders[name] = strings.Join(values, ", ")
		}

		// Обрабатываем сжатие
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(strings.NewReader(string(bodyBytes))) // Восстанавливаем тело ответа для последующего использования

		// Декодируем тело, если оно сжато (gzip, deflate и т.д.)
		if resp.Header.Get("Content-Encoding") == "gzip" {
			bodyBytes, err = decodeGzip(bodyBytes)
			if err != nil {
				log.Println("Error decoding gzip:", err)
			}
		}

		//bodyString := string(bodyBytes)

		parsedResp = domain.Response{
			Code:    resp.StatusCode,
			Message: resp.Status,
			Headers: responseHeaders,
			//Body:    bodyString,
		}
	}

	// Сохраняем запрос и ответ в MongoDB
	storedReq := domain.ReqResp{
		Req:  parsedReq,
		Resp: parsedResp,
	}

	return storedReq
}

func (p *ProxyStorage) RepeatRequest(reqId string) *http.Response {
	req := getRequest(p.db, reqId)
	fmt.Println(req)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Отключаем проверку сертификатов для HTTPS-запросов
			},
		},
	}

	// Извлекаем протокол из Referer, если он указан
	referer := req.Headers["Referer"]
	var completeURL string
	if referer != "" {
		parsedURL, err := url.Parse(referer)
		if err == nil && parsedURL.Scheme != "" {
			// Строим полный URL запроса с использованием схемы и хоста из Referer
			completeURL = fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, req.Path)
		} else {
			// Если не удалось разобрать Referer, используем http по умолчанию
			completeURL = "http://" + req.Path
		}
	} else {
		// Если Referer отсутствует, используем http по умолчанию
		completeURL = "http://" + req.Path
	}

	// Формируем новый HTTP-запрос
	httpReq, err := http.NewRequest(req.Method, completeURL, strings.NewReader(req.Body)) // Добавляем тело запроса, если оно есть
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// Добавляем заголовки и куки
	for header, value := range req.Headers {
		httpReq.Header.Add(header, value)
	}
	for cookie, value := range req.Cookies {
		httpReq.AddCookie(&http.Cookie{Name: cookie, Value: value})
	}

	fmt.Println()
	fmt.Println()
	fmt.Println()

	// Отправляем запрос
	fmt.Println(httpReq)
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	fmt.Println(resp)
	return resp
}

func getRequest(db *mongo.Database, id string) domain.Request {
	collection := db.Collection("request_response")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		fmt.Println("Произошла ошибка при поиске запроса: ", err)
	}

	filter := bson.M{"_id": objectID}

	var result domain.ReqResp
	err = collection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("Документ не найден")
		} else {
			log.Fatalf("Ошибка при выполнении поиска: %v", err)
		}
		return result.Req
	}
	return result.Req
}

// Функция для декодирования gzip
func decodeGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func PutItemToDatabase(db *mongo.Database, r *http.Request, resp *http.Response) {
	var collection *mongo.Collection

	item := storeRequest(r, resp)
	collection = db.Collection("request_response")

	_, err := collection.InsertOne(context.TODO(), item)
	if err != nil {
		log.Println("Ошибка при добавлении элемента в базу данных:", err)
	} else {
		log.Println("Элемент успешно сохранен в базу данных")
	}
}

func ensureValidUTF8(s string) string {
	if !utf8.ValidString(s) {
		return string([]rune(s)) // Конвертируем в UTF-8, убирая недопустимые символы
	}
	return s
}
