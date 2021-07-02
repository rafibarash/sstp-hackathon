package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/spanner"
	"github.com/gorilla/mux"
)

var (
	dependencyCols = []string{"SourceDigest", "BaseDigest", "BaseRef"}
	imageCols      = []string{"Repository", "Tag", "Digest"}

	dbClient  *spanner.Client
	differURL string
)

const (
	// Hardcode dependency and service tags for now.
	depTag  = "us-docker.pkg.dev/independency-day-mirror/base/node:latest"
	servTag = "us-docker.pkg.dev/jonjohnson-test/sstp-hackathon/frontend:latest"
)

func main() {
	ctx := context.Background()

	// Initialize router.
	r := mux.NewRouter()
	handleRoutes(r)

	// Initialize db client.
	dbName := os.Getenv("DB")
	if dbName == "" {
		dbName = "projects/jonjohnson-test/instances/independency/databases/day"
	}
	var err error
	dbClient, err = spanner.NewClient(ctx, dbName)
	defer dbClient.Close()
	if err != nil {
		fmt.Printf("Failed to connect to db %q: %v", dbName, err)
		return
	}

	// Set differ url.
	differURL = os.Getenv("DIFFER")
	if differURL == "" {
		differURL = "http://localhost:8081"
	}

	// Listen and serve baby.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("Server starting on http://localhost:" + port + "...")
	http.ListenAndServe(fmt.Sprintf(":%s", port), r)
}

/***************************************************
 ** Routes
 **************************************************/

type WatchReq struct {
	SourceDigest string `json:"source_digest"`
	BaseDigest   string `json:"base_digest"`
	BaseRef      string `json:"base_ref"`
}

// Notification is the GCR/AR notification payload as described in
// https://cloud.google.com/artifact-registry/docs/configure-notifications#examples.
type Notification struct {
	Action string `json:"action"`
	Digest string `json:"digest"`
	Tag    string `json:"tag"`
}

type Image struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
}

// Handles API routes for mux router.
func handleRoutes(r *mux.Router) {
	r.HandleFunc("/ping", Ping).Methods("GET")
	r.HandleFunc("/watch", WatchDependency).Methods("POST")
	r.HandleFunc("/notification", HandleNotification).Methods("POST")
}

func Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func WatchDependency(w http.ResponseWriter, r *http.Request) {
	var wr WatchReq
	if err := json.NewDecoder(r.Body).Decode(&wr); err != nil {
		log.Println("Failed to decode Watch request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// INSERT into dependencies table.
	if err := addDependency(r.Context(), wr); err != nil {
		fmt.Printf("Failed to add dependency %v to spanner: %v", wr, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Printf("Added dependency %v to spanner...\n", wr)
	w.WriteHeader(http.StatusCreated)
	return
}

// addDependency inserts into the Spanner Dependency table.
func addDependency(ctx context.Context, wr WatchReq) error {
	m := spanner.Insert("Dependencies", dependencyCols, []interface{}{wr.SourceDigest, wr.BaseDigest, wr.BaseRef})
	_, err := dbClient.Apply(ctx, []*spanner.Mutation{m})
	return err
}

// addImage inserts into the Spanner Images table.
func addImage(ctx context.Context, i Image) error {
	m := spanner.Insert("Images", imageCols, []interface{}{i.Repository, i.Tag, i.Digest})
	_, err := dbClient.Apply(ctx, []*spanner.Mutation{m})
	return err
}

// HandleNotification recieves notifications from pushes/deletions to AR/GCR.
// If a service (prod image) we own is updated, we ...
// If a service dependency (base image) is updated, ... and create a Cloud Build Trigger to rebuild our service's image.
func HandleNotification(w http.ResponseWriter, r *http.Request) {
	var n Notification
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		log.Println("Failed to decode AR notification: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// Only handle INSERT notifications.
	if n.Action != "INSERT" {
		w.WriteHeader(http.StatusOK)
		return
	}
	// INSERT into images table.
	i := Image{Repository: "", Tag: n.Tag, Digest: n.Digest}
	if err := addImage(r.Context(), i); err != nil {
		fmt.Printf("Failed to add image %v to spanner: %v", i, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Printf("Added image %v to spanner...\n", i)
	// Call differ to get all image digests that need be rebuilt (images that depend on upstream).
	params := fmt.Sprintf("tag=%s&digest=%s", n.Tag, n.Digest)
	path := fmt.Sprintf("%s?%s", differURL, params)
	resp, err := http.Get(path)
	if err != nil {
		fmt.Printf("Diff request %q failed: %v\n", path, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read body from diff request %q: %v\n", path, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Printf("Successful diff request %q returned: %v", path, body)
	// Send build trigger requests for all out of date images
	w.WriteHeader(http.StatusOK)
}
