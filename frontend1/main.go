package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
)

type PageData struct {
	PageHeader string
	IsRed      bool
	IsGreen    bool
	IsDefault  bool
}

func main() {
	tmpl := template.Must(template.ParseFiles("layout.html"))
	c := os.Getenv("DEMO_COLOR")
	data := PageData{
		PageHeader: "Happy In-Dependency Day!",
	}

	if c == "red" {
		data.IsRed = true
	} else if c == "green" {
		data.IsGreen = true
	} else {
		data.IsDefault = true
	}

	fmt.Printf("page data: %v\n", data)

	// Template route.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, data)
	})

	// Listen and serve baby.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("Server starting on http://localhost:" + port + "...")
	http.ListenAndServe(":8080", nil)
}
