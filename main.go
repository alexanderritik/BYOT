package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func isHealth(h http.ResponseWriter, r *http.Request) {
	h.WriteHeader(http.StatusOK)
	h.Write([]byte("ok"))

}

func run(h http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	value := strings.TrimPrefix(path, "/run/")

	cmd := exec.Command("uploads/" + value)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	h.WriteHeader(http.StatusOK)
	h.Write([]byte(output))
}

func uploadBinary(h http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.WriteHeader(http.StatusMethodNotAllowed)
		h.Write([]byte("POST only accepted"))
		return
	}

	file, header, err := r.FormFile("binary")
	if err != nil {
		h.WriteHeader(http.StatusInternalServerError)
		h.Write([]byte("failed to read file"))
		return
	}

	dst, err := os.Create("uploads/" + header.Filename)
	if err != nil {
		h.WriteHeader(http.StatusInternalServerError)
		h.Write([]byte("failed to create file"))
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	cmd := exec.Command("chmod", "+x", "uploads/"+header.Filename)
	cmd.Run()
	h.WriteHeader(http.StatusOK)
	h.Write([]byte("binary uploaded: " + header.Filename))
}

func main() {

	// var value string

	// n, err := fmt.Scan(&value)
	// if err != nil || n == 0 {
	// 	fmt.Println("error:", err)
	// 	return
	// }
	// fmt.Println(value)

	http.HandleFunc("/health", isHealth)
	http.HandleFunc("/uploadBinary", uploadBinary)
	http.HandleFunc("/run/", run)
	http.ListenAndServe(":3000", nil)
	//
}
