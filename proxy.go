package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type Config struct {
	Port string `json:"port"`
	Password string `json:"password"`
}

var config Config
var mu sync.Mutex

func loadConfig() {
	file, err := os.Open("config.json")
	if err != nil {
		if os.IsNotExist(err) {
			config = Config{
				Port: "8080", // 默认端口
				Password: "QazXswEdc!56", // 默认密码
			}
			saveConfig()
		} else {
			log.Fatalf("Error opening config file: %v", err)
		}
	} else {
		defer file.Close()
		decoder := json.NewDecoder(file)
		err = decoder.Decode(&config)
		if err != nil {
			log.Fatalf("Error decoding config file: %v", err)
		}
	}
}

func saveConfig() {
	file, err := os.Create("config.json")
	if err != nil {
		log.Fatalf("Error creating config file: %v", err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(&config)
	if err != nil {
		log.Fatalf("Error encoding config file: %v", err)
	}
}

func hexToBytes(hexStr string) ([]byte, error) {
	return hex.DecodeString(hexStr)
}

func xorDecrypt(encrypted string, key string) (string, error) {
	encryptedBytes, err := hexToBytes(encrypted)
	if err != nil {
		return "", err
	}

	keyBytes := []byte(key)
	keyLength := len(keyBytes)
	output := make([]byte, len(encryptedBytes))

	for i := range encryptedBytes {
		output[i] = encryptedBytes[i] ^ keyBytes[i % keyLength]
	}

	return string(output), nil
}

func fetchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, HEAD, OPTIONS")
		w.WriteHeader(http.StatusOK)
		return
	}

	data := r.URL.Query().Get("data")
	if data == "" {
		http.Error(w, "Access denied: didn't provide data", http.StatusForbidden)
		return
	}

	var jsonData map[string]string
	var decryptedData string
	var err error

	decryptedData, err = xorDecrypt(data, config.Password)
	if err != nil || json.Unmarshal([]byte(decryptedData), &jsonData) != nil {
		decryptedData, err = xorDecrypt(data, "download")
		if err != nil || json.Unmarshal([]byte(decryptedData), &jsonData) != nil {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	actualUrlStr, ok := jsonData["url"]
	if !ok {
		http.Error(w, "Access denied: didn't provide a link", http.StatusForbidden)
		return
	}

	actualUrl := actualUrlStr
	newRequest, err := http.NewRequest(r.Method, actualUrl, nil)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		if strings.ToLower(key) != "host" && strings.ToLower(key) != "referer" && !strings.HasPrefix(strings.ToLower(key), "cf-") {
			for _, value := range values {
				newRequest.Header.Add(key, value)
			}
		}
	}

	newRequest.Header.Set("User-Agent", jsonData["ua"])
	client := &http.Client{}
	response, err := client.Do(newRequest)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	// 处理可能的重定向
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		newUrl := response.Header.Get("Location")
		if newUrl != "" {
			newRequest, err = http.NewRequest(r.Method, newUrl, nil)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			response, err = client.Do(newRequest)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			defer response.Body.Close()
		}
	}

	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
}

func main() {
	loadConfig()
	http.HandleFunc("/", fetchHandler)
	fmt.Printf("Server starting at port %s...\n", config.Port)
	if err := http.ListenAndServe(":" + config.Port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
