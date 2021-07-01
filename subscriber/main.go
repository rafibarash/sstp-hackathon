package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

const (
	// Hardcode dependency and service tags for now.
	depTag  = "us-docker.pkg.dev/independency-day-mirror/base/node:latest"
	servTag = "us-docker.pkg.dev/jonjohnson-test/sstp-hackathon/frontend:latest"
)

// ARNotification is the notification payload as described in
// https://cloud.google.com/artifact-registry/docs/configure-notifications#examples.
type ARNotification struct {
	Action string `json:"action"`
	Digest string `json:"digest"`
	Tag    string `json:"tag"`
}

func main() {
	// Initialize router
	r := mux.NewRouter()
	handleRoutes(r)

	// Listen and serve baby
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("Server starting on http://localhost:" + port + "...")
	// allowedOrigins := handlers.AllowedOrigins([]string{"http://localhost:3000", "http://localhost:5000", "https://frontend-bwkgpgz7aq-uc.a.run.app"})
	// http.ListenAndServe(fmt.Sprintf(":%s", port), handlers.CORS(allowedOrigins)(r))
	http.ListenAndServe(fmt.Sprintf(":%s", port), r)
}

/*****************************************
 ** Routes
 ****************************************/

// Handles API routes for mux router.
func handleRoutes(r *mux.Router) {
	r.HandleFunc("/ping", GetPing).Methods("GET")
	r.HandleFunc("/watcher", HandleNotification).Methods("POST")
}

func GetPing(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// HandleNotification recieves notifications from pushes/deletions to AR/GCR.
// If a service (prod image) we own is updated, we ...
// If a service dependency (base image) is updated, ... and create a Cloud Build Trigger to rebuild our service's image.
func HandleNotification(w http.ResponseWriter, r *http.Request) {
	var n ARNotification
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		log.Println("Failed to decode AR notification: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Check for dependency change.
	if isChange, t := isDependencyChange(n); isChange {
		// TODO: Actually handle dep change.
		// Will want to use https://pkg.go.dev/google.golang.org/api/cloudbuild/v1.
		log.Printf("Will handle \"INSERT\" notification for base image %q, which is dependency for service %q\n", n.Digest, t)
		w.WriteHeader(http.StatusOK)
		return
	}
	if isServiceChange(n) {
		// TODO: Actually handle service change.
		log.Printf("Will handle \"INSERT\" notification for our service %q\n", n.Digest)
		w.WriteHeader(http.StatusOK)
	}
}

/*****************************************
 ** Dependencies/Service Changes
 ****************************************/

// isDependencyChange returns true if the passed in notification represents a dependency change
// for a service (prod image) we own, along with the dependent service's tag.
func isDependencyChange(n ARNotification) (bool, string) {
	if n.Action != "INSERT" {
		return false, ""
	}
	// TODO: Query db to see if tag is a dependency (base image) for a service (prod image) we own.
	// TODO: Only return true if notification's digest is different than base image digest in db.
	// For now we will just use hardcoded dependency and service and ignore digest/tags.
	if imageNameFromTag(depTag) != imageNameFromDigest(n.Digest) {
		return false, ""
	}
	return true, servTag
}

func isServiceChange(n ARNotification) bool {
	if n.Action != "INSERT" {
		return false
	}
	// TODO: Query db to see if tag is a service we own.
	// For now just use hardcoded service and ignore digest/tags.
	return imageNameFromTag(servTag) == imageNameFromDigest(n.Digest)
}

/*****************************************
 ** Misc Helpers
 ****************************************/

func imageNameFromDigest(d string) string {
	return strings.Split(d, "@")[0]
}

func imageNameFromTag(t string) string {
	return strings.Split(t, ":")[0]
}
