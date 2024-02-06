package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/rs/cors"
	"github.com/rus-sharafiev/photo-converter/common/auth"
	"github.com/rus-sharafiev/photo-converter/common/exception"
	"github.com/rus-sharafiev/photo-converter/upload"
)

func main() {

	url := flag.String("url", "", "URL to submit saved image location")
	saveLocation := flag.String("save-location", "static", "root directory where to save uploaded images (should exist!)")
	port := flag.String("port", "55555", "PORT to run http handler")
	redirectUrl := flag.String("redirect-url", "", "URL to redirect on root request")
	flag.Parse()

	router := http.NewServeMux()

	// Handle and serve uploads
	router.Handle("/upload/", upload.Controller{
		UploadDir: *saveLocation,
		SubmitUrl: *url,
	})

	// Handle root location request
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if len(*redirectUrl) == 0 {
			exception.NotFound(w)
		} else {
			http.Redirect(w, r, *redirectUrl, http.StatusSeeOther)
		}
	})

	handler := auth.Guard(router)
	handler = cors.New(cors.Options{
		AllowedOrigins:   []string{"http://192.168.190.9:5555", "http://192.168.190.9:8000", "http://localhost:8000"},
		AllowedHeaders:   []string{"Content-Type", "Fingerprint", "Authorization"},
		AllowCredentials: true,
		Debug:            true,
	}).Handler(handler)

	fmt.Println("Photo converter is running...")
	log.Fatal(http.ListenAndServe(":"+*port, handler))
}
