package main

import (
    "database/sql"
    "encoding/json"
    "flag"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "regexp"
    "sync"
    "time"

    _ "github.com/mattn/go-sqlite3"
)

var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 32*1024)
    },
}

var (
    db         *sql.DB
    httpClient *http.Client
    xorKey     string
)

func init() {
    var err error
    db, err = sql.Open("sqlite3", "./data.db")
    if err != nil {
        log.Fatal(err)
    }

    createTable := `
    CREATE TABLE IF NOT EXISTS requests (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        data TEXT NOT NULL,
        decryptedData TEXT NOT NULL,
        url TEXT NOT NULL,
        userAgent TEXT NOT NULL
    );`
    _, err = db.Exec(createTable)
    if err != nil {
        log.Fatal(err)
    }

    httpClient = &http.Client{
        Transport: &http.Transport{
            MaxIdleConns:       10,
            IdleConnTimeout:    30 * time.Second,
            DisableKeepAlives:  false,
            MaxIdleConnsPerHost: 10,
        },
    }
}

func handler(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query()
    data := query.Get("data")
    if data == "" {
        if !writeHeaderOnce(w, http.StatusForbidden) {
            return
        }
        http.Error(w, "Access denied: didn't provide data", http.StatusForbidden)
        return
    }

    decryptedData := xorDecrypt(data, xorKey)
    var jsonMap map[string]string
    if err := json.Unmarshal([]byte(decryptedData), &jsonMap); err != nil {
        if !writeHeaderOnce(w, http.StatusForbidden) {
            return
        }
        http.Error(w, "Access denied", http.StatusForbidden)
        return
    }

    actualUrlStr := jsonMap["url"]
    if actualUrlStr == "" {
        if !writeHeaderOnce(w, http.StatusForbidden) {
            return
        }
        http.Error(w, "Access denied: didn't provide a link", http.StatusForbidden)
        return
    }

    actualUrl, err := url.Parse(actualUrlStr)
    if err != nil {
        if !writeHeaderOnce(w, http.StatusForbidden) {
            return
        }
        http.Error(w, "Access denied: invalid link", http.StatusForbidden)
        return
    }

    _, err = db.Exec("INSERT INTO requests (data, decryptedData, url, userAgent) VALUES (?, ?, ?, ?)", data, decryptedData, actualUrlStr, jsonMap["ua"])
    if err != nil {
        if !writeHeaderOnce(w, http.StatusInternalServerError) {
            return
        }
        http.Error(w, "Server error", http.StatusInternalServerError)
        return
    }

    proxyReq, err := http.NewRequest(r.Method, actualUrl.String(), r.Body)
    if err != nil {
        if !writeHeaderOnce(w, http.StatusInternalServerError) {
            return
        }
        http.Error(w, "Server error", http.StatusInternalServerError)
        return
    }
    proxyReq.Header = r.Header
    proxyReq.Header.Set("Host", actualUrl.Host)
    proxyReq.Header.Set("Referer", actualUrlStr)
    proxyReq.Header.Set("User-Agent", jsonMap["ua"])

    proxyRes, err := httpClient.Do(proxyReq)
    if err != nil {
        if !writeHeaderOnce(w, http.StatusInternalServerError) {
            return
        }
        http.Error(w, "Server error", http.StatusInternalServerError)
        return
    }
    defer proxyRes.Body.Close()

    for key, value := range proxyRes.Header {
        if key == "Content-Length" || key == "Content-Type" || key == "Content-Range" {
            w.Header().Set(key, value[0])
        }
    }

    contentDisposition := proxyRes.Header.Get("Content-Disposition")
    filename := "index.html"
    if contentDisposition != "" {
        re := regexp.MustCompile(`filename="([^"]+)"`)
        matches := re.FindStringSubmatch(contentDisposition)
        if len(matches) == 2 {
            filename = matches[1]
        }
    }

    w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

    if w.Header().Get("Content-Type") == "" {
        writeHeaderOnce(w, proxyRes.StatusCode)
    }

    buf := bufferPool.Get().([]byte)
    defer bufferPool.Put(buf)
    if _, err := io.CopyBuffer(w, proxyRes.Body, buf); err != nil {
        if !writeHeaderOnce(w, http.StatusInternalServerError) {
            return
        }
        http.Error(w, "Server error", http.StatusInternalServerError)
    }
}

func writeHeaderOnce(w http.ResponseWriter, statusCode int) bool {
    if _, ok := w.(interface{ WriteHeaderOnce(int) }); ok {
        return false
    }
    w.WriteHeader(statusCode)
    return true
}

func xorDecrypt(encrypted, key string) string {
    encryptedBytes := hex2Bytes(encrypted)
    keyLength := len(key)
    output := make([]byte, len(encryptedBytes))
    for i := range encryptedBytes {
        output[i] = encryptedBytes[i] ^ key[i%keyLength]
    }
    return string(output)
}

func hex2Bytes(hexStr string) []byte {
    bytes := make([]byte, len(hexStr)/2)
    for i := 0; i < len(hexStr); i += 2 {
        bytes[i/2] = hex2Byte(hexStr[i], hexStr[i+1])
    }
    return bytes
}

func hex2Byte(c1, c2 byte) byte {
    return (hexChar2Byte(c1) << 4) | hexChar2Byte(c2)
}

func hexChar2Byte(c byte) byte {
    switch {
    case '0' <= c && c <= '9':
        return c - '0'
    case 'a' <= c && c <= 'f':
        return c - 'a' + 10
    case 'A' <= c && c <= 'F':
        return c - 'A' + 10
    default:
        return 0 // 添加默认返回值
    }
}

func main() {
    port := flag.String("port", "6000", "服务器监听的端口")
    password := flag.String("password", "QazXswEdc!23", "用于解密的密码")
    flag.Parse()

    xorKey = *password

    log.Printf("服务器启动中，监听端口: %s，使用的解密密码: %s\n", *port, *password)
    http.HandleFunc("/", handler)
    log.Fatal(http.ListenAndServe(":"+*port, nil))

    host, exists := os.LookupEnv("HOST")
    if exists {
        log.Printf("环境变量 HOST: %s\n", host)
    }
}
