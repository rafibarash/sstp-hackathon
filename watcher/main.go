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

var (
	watchset = make(map[string]void)
	member   void
)

const (
	// Hardcode dependency and service tags for now.
	depTag  = "us-docker.pkg.dev/independency-day-mirror/base/node:latest"
	servTag = "us-docker.pkg.dev/jonjohnson-test/sstp-hackathon/frontend:latest"
)

type void struct{}

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

type WatchReq struct {
	Tag string `json:"tag"`
}

// Notification is the GCR/AR notification payload as described in
// https://cloud.google.com/artifact-registry/docs/configure-notifications#examples.
type Notification struct {
	Action string `json:"action"`
	Digest string `json:"digest"`
	Tag    string `json:"tag"`
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
	// TODO: Query for base ref in dependencies table.
	// If not present, add new row to dependencies table
	watchset[wr.Tag] = member
	fmt.Printf("Added tag %q to watchset...\n", wr.Tag)
	fmt.Printf("Watching for tags: %v\n", watchset)
	w.WriteHeader(http.StatusCreated)
	return
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
		w.WriteHeader(status.StatusOK)
		return
	}
	// INSERT into images
	// Get all image digests that need be rebuilt (images that depend on upstream)

	w.WriteHeader(http.StatusOK)
}

// // isDependencyChange returns true if the passed in notification represents a dependency change
// // for a service (prod image) we own, along with the dependent service's tag.
// func isDependencyChange(n Notification) (bool, string) {
// 	if n.Action != "INSERT" {
// 		return false, ""
// 	}
// 	// TODO: Query db to see if tag is a dependency (base image) for a service (prod image) we own.
// 	// TODO: Only return true if notification's digest is different than base image digest in db.
// 	// For now we will just use hardcoded dependency and service and ignore digest/tags.
// 	if imageNameFromTag(depTag) != imageNameFromDigest(n.Digest) {
// 		return false, ""
// 	}
// 	return true, servTag
// }

// func isServiceChange(n Notification) bool {
// 	if n.Action != "INSERT" {
// 		return false
// 	}
// 	// TODO: Query db to see if tag is a service we own.
// 	// For now just use hardcoded service and ignore digest/tags.
// 	return imageNameFromTag(servTag) == imageNameFromDigest(n.Digest)
// }

/*****************************************
 ** Misc Helpers
 ****************************************/

// func imageNameFromDigest(d string) string {
// 	return strings.Split(d, "@")[0]
// }

// func imageNameFromTag(t string) string {
// 	return strings.Split(t, ":")[0]
// }
