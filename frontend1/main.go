package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
)

type PageData struct {
	PageHeader string
	DemoColor  string
}

func main() {
	tmpl := template.Must(template.ParseFiles("layout.html"))
	c := os.Getenv("DEMO_COLOR")
	fmt.Printf("demo color: %q\n", c)

	// Template route.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := PageData{
			PageHeader: "Happy In-Dependency Day!",
			DemoColor:  c,
		}
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
