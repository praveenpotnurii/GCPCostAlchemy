package main

import (
	"net/http"
)

func main() {
	// Set up static file server
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Set up route handlers
	http.HandleFunc("/", loginHandler)

	port := ":8080"
	println("Server starting at http://localhost" + port)
	if err := http.ListenAndServe(port, nil); err != nil {
		println("Error starting server:", err.Error())
	}
}

