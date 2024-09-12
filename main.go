package main

import (
    "log"
    "net/http"
    "os"

    _ "google.golang.org/api/cloudresourcemanager/v1"
)

var logger *log.Logger

func init() {
    // Set up logging
    logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        log.Fatal("Failed to open log file:", err)
    }
    logger = log.New(logFile, "", log.Ldate|log.Ltime|log.Lshortfile)
}

func main() {
    http.HandleFunc("/", landingHandler)
    http.HandleFunc("/home", homeHandler)
    http.HandleFunc("/cost-recommendations", costRecommendationsHandler)

    port := ":8080"
    logger.Println("Server starting at http://localhost" + port)
    if err := http.ListenAndServe(port, nil); err != nil {
        logger.Fatalf("Error starting server: %v", err)
    }
}