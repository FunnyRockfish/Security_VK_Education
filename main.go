package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	db "Security_VK_Education/database"
	"Security_VK_Education/domain"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	db := db.ConnectToMongoDataBase()
	CreateProxyServer(db)
}

func CreateProxyServer(db *mongo.Database) {
	err := genCertificate("./gen_ca.sh", "", 0)
	if err != nil {
		log.Fatal("Ошибка при генерации корневого сертификата:", err)
	}

	server := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handleProxy(w, r, db)
		}),
	}
	fmt.Println("Сервер запущен и слушает на порту 8080")
	rand.Seed(time.Now().UnixNano())
	err = server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}

func handleProxy(w http.ResponseWriter, r *http.Request, database *mongo.Database) {
	fmt.Println("Proxy request:", r.Method, r.URL)
	if r.Method == http.MethodConnect {
		handleHTPPS(w, r, database)
	} else if r.Method == http.MethodPost || r.Method == http.MethodHead || r.Method == http.MethodGet || r.Method == http.MethodPut {
		handleHTTP(w, r, database)
	}
}

func handleHTTP(w http.ResponseWriter, r *http.Request, database *mongo.Database) {
	request := domain.Request{}
	targetURL := r.URL.String()
	if !strings.HasPrefix(targetURL, "http") {
		http.Error(w, "Целевой URL должен начинаться с http", http.StatusBadRequest)
		return
	}

	parsedTargetUrl, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Неправильный URL", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest(r.Method, parsedTargetUrl.RequestURI(), r.Body)
	if err != nil {
		http.Error(w, "Ошибка при создании запроса", http.StatusInternalServerError)
		log.Println("Ошибка при создании запроса:", err)
		return
	}

	req.Header = r.Header.Clone()
	req.Host = parsedTargetUrl.Host
	req.URL.Scheme = parsedTargetUrl.Scheme
	req.URL.Host = parsedTargetUrl.Host

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

func handleHTPPS(w http.ResponseWriter, r *http.Request, database *mongo.Database) {
	targetHostWithPort := r.Host
	targetHostWithoutPort := strings.Split(targetHostWithPort, ":")[0]

	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Не удалось получить управление сокетом", http.StatusInternalServerError)
		log.Println("Не удалось получить управление сокетом")
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Println("Ошибка при захвате соединения:", err)
		return
	}
	randomNumber := rand.Int63n(1000000000)
	err = genCertificate("./gen_cert.sh", targetHostWithoutPort, randomNumber)
	if err != nil {
		log.Fatal(err)
		return
	}

	serverConn, err := net.Dial("tcp", targetHostWithPort)
	if err != nil {
		log.Fatal("Не удалось установить TLS-соединение с сервером")
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		TransferData(serverConn, clientConn)
	}()

	go func() {
		defer wg.Done()
		TransferData(clientConn, serverConn)
	}()

	wg.Wait()

	clientConn.Close()
	serverConn.Close()
}

func TransferData(target, source net.Conn) {
	_, err := io.Copy(target, source)
	if err != nil {
		log.Println("Ошибка при передаче данных:", err)
	}
}

func genCertificate(scriptPath, host string, serialNumber int64) error {
	var cmd *exec.Cmd
	if host == "" {
		if _, err := os.Stat("./certs"); !os.IsNotExist(err) {
			fmt.Println("корневной сертификат уже создан")
			return nil
		}
		cmd = exec.Command("bash", scriptPath)
		fmt.Println(cmd)
	} else {
		cmd = exec.Command("bash", scriptPath, host, strconv.FormatInt(serialNumber, 10))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ошибка выполнения скрипта: %s, вывод: %s", err, string(output))
	}
	fmt.Println("Корневой сертификат сгенерирован успешно")
	return nil
}

func PutItemToDatabase(db *mongo.Database, item interface{}, itemType string) {

}
