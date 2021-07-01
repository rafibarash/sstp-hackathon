package main

func main() {
	// Initialize router.
	r := mux.NewRouter()
	handleRoutes(r)

	// Listen and serve baby.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("Server starting on http://localhost:" + port + "...")
	http.ListenAndServe(fmt.Sprintf(":%s", port), r)
}

/*****************************************
 ** Routes
 ****************************************/

type UpstreamChangedReq struct {
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}

// Handles API routes for mux router.
func handleRoutes(r *mux.Router) {
	r.HandleFunc("/ping", Ping).Methods("GET")
	r.HandleFunc("/upstream", UpstreamChanged).Methods("POST")
}

func Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func UpstreamChanged(w http.ResponseWriter, r *http.Request) {
	var ucr UpstreamChangedReq
	if err := json.NewDecoder(r.Body).Decode(&ucr); err != nil {
		log.Println("Failed to decode UpstreamChanged request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// SELECT all rows in dependencies table where BaseRef = upstream:tag & BaseDigest != upstream:digest
	// Now we have all out of date SourceDigests
	// JOIN with images table to get all SourceTags for SourceDigests
	// Return all sources (tags/digests) that need to be changed
}
