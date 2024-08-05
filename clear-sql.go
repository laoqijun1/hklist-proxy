package main

import (
    "database/sql"
    "log"
    "os"
    "time"

    _ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

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
        createdAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`
    _, err = db.Exec(createTable)
    if err != nil {
        log.Fatal(err)
    }

    go startDatabaseSizeMonitor()
}

func startDatabaseSizeMonitor() {
    ticker := time.NewTicker(1 * time.Hour)
    for {
        select {
        case <-ticker.C:
            checkDatabaseSize()
        }
    }
}

func checkDatabaseSize() {
    fileInfo, err := os.Stat("./data.db")
    if err != nil {
        log.Printf("Failed to get database file info: %v\n", err)
        return
    }

    const maxSize = 5 * 1024 * 1024 * 1024 // 设定最大数据库大小为20MB
    if fileInfo.Size() > maxSize {
        cleanOldRecords()
    }
}

func cleanOldRecords() {
    // 删除最旧的记录
    deleteQuery := `
    DELETE FROM requests WHERE id IN (
        SELECT id FROM requests ORDER BY createdAt ASC LIMIT 1000
    );`
    _, err := db.Exec(deleteQuery)
    if err != nil {
        log.Printf("Failed to clean old records: %v\n", err)
    } else {
        log.Println("Old records cleaned successfully.")
    }
}

func main() {
    // 运行你的主程序逻辑
    select {}
}
