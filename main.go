package main

import (
	"io"
	"net/http"
	"os"
)

func isHealth(h http.ResponseWriter, r *http.Request) {
	h.WriteHeader(http.StatusOK)
	h.Write([]byte("ok"))

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

	// cmd := exec.Command(value)
	// output, err := cmd.Output()
	// if err != nil {
	// 	fmt.Println("error:", err)
	// 	return
	// }
	// fmt.Println(string(output))

	http.HandleFunc("/health", isHealth)
	http.HandleFunc("/uploadBinary", uploadBinary)
	http.ListenAndServe(":3000", nil)
	//
}
