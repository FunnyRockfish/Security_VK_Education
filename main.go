package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	CreateProxyServer()
}

func CreateProxyServer() {
	/*
		err := genCertificate("./gen_ca.sh", "", 0)
		if err != nil {
			log.Fatal("Ошибка при запуске сервера:", err)
		}
		fmt.Println("Корневой сертификат сгенерирован")
	*/
	server := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handleProxy),
	}
	fmt.Println("Сервер запущен и слушает на порту 8080")
	rand.Seed(time.Now().UnixNano())
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Proxy request:", r.Method, r.URL)
	if r.Method == http.MethodConnect {
		handleConnect(w, r)
	}
}

func handleConnect(w http.ResponseWriter, r *http.Request) {
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
		cmd = exec.Command("bash", scriptPath)
		fmt.Println(cmd)
	} else {
		cmd = exec.Command("bash", scriptPath, host, strconv.FormatInt(serialNumber, 10))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ошибка выполнения скрипта: %s, вывод: %s", err, string(output))
	}
	return nil
}
