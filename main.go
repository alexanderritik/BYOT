package main

import (
	"fmt"
	"net/http"

	"github.com/alexanderritik/mini-lambda/handler"
)

func main() {
	http.HandleFunc("/health", handler.IsHealth)
	http.HandleFunc("/uploadBinary", handler.UploadBinary)
	http.HandleFunc("/run/", handler.Run)
	if err := http.ListenAndServe(":3000", nil); err != nil {
		fmt.Println("server error:", err)
	}
}
