package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/spanner"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/gorilla/mux"
)

var (
	dependencyCols = []string{"SourceDigest", "BaseDigest", "BaseRef"}
	imageCols      = []string{"Repository", "Tag", "Digest"}

	dbClient     *spanner.Client
	pubsubClient *pubsub.Client
	topic        *pubsub.Topic
	differURL    string
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
	topicName := os.Getenv("BUILD_TOPIC")
	if topicName == "" {
		topicName = "frontend-build"
	}

	var err error
	dbClient, err = spanner.NewClient(ctx, dbName)
	defer dbClient.Close()
	if err != nil {
		fmt.Printf("Failed to connect to db %q: %v", dbName, err)
		return
	}

	pubsubClient, err = pubsub.NewClient(ctx, "jonjohnson-test")
	defer pubsubClient.Close()
	if err != nil {
		fmt.Printf("Failed to connect to pubsub %q: %v", dbName, err)
		return
	}
	topic = pubsubClient.Topic(topicName)

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

type PubsubMessage struct {
	// Attributes: Attributes for this message. If this field is empty, the
	// message must contain non-empty data. This can be used to filter
	// messages on the subscription.
	Attributes map[string]string `json:"attributes,omitempty"`

	// Data: The message data field. If this field is empty, the message
	// must contain at least one attribute.
	Data []byte `json:"data,omitempty"`

	// MessageId: ID of this message, assigned by the server when the
	// message is published. Guaranteed to be unique within the topic. This
	// value may be read by a subscriber that receives a `PubsubMessage` via
	// a `Pull` call or a push delivery. It must not be populated by the
	// publisher in a `Publish` call.
	MessageId string `json:"messageId,omitempty"`

	// OrderingKey: If non-empty, identifies related messages for which
	// publish order should be respected. If a `Subscription` has
	// `enable_message_ordering` set to `true`, messages published with the
	// same non-empty `ordering_key` value will be delivered to subscribers
	// in the order in which they are received by the Pub/Sub system. All
	// `PubsubMessage`s published in a given `PublishRequest` must specify
	// the same `ordering_key` value.
	OrderingKey string `json:"orderingKey,omitempty"`

	// PublishTime: The time at which the message was published, populated
	// by the server when it receives the `Publish` call. It must not be
	// populated by the publisher in a `Publish` call.
	PublishTime string `json:"publishTime,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Attributes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Attributes") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

type PushNotification struct {
	Message      PubsubMessage `json:"message"`
	Subscription string        `json:"subscription"`
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
		log.Printf("Failed to decode Watch request: %v", err)
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
	if err != nil && status.Code(err) == codes.AlreadyExists {
		log.Printf("dep %s already exists", wr.SourceDigest)
	}
	return err
}

// addImage inserts into the Spanner Images table.
func addImage(ctx context.Context, i Image) error {
	m := spanner.InsertOrUpdate("Images", imageCols, []interface{}{i.Repository, i.Tag, i.Digest})
	_, err := dbClient.Apply(ctx, []*spanner.Mutation{m})
	return err
}

func deleteImage(ctx context.Context, i Image) error {
	m := spanner.Delete("Images", spanner.Key{i.Repository, i.Tag})
	_, err := dbClient.Apply(ctx, []*spanner.Mutation{m})
	return err
}

// HandleNotification recieves notifications from pushes/deletions to AR/GCR.
// If a service (prod image) we own is updated, we ...
// If a service dependency (base image) is updated, ... and create a Cloud Build Trigger to rebuild our service's image.
func HandleNotification(w http.ResponseWriter, r *http.Request) {
	var pn PushNotification
	if err := json.NewDecoder(r.Body).Decode(&pn); err != nil {
		log.Printf("Failed to decode Pubsub notification: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var n Notification
	if err := json.Unmarshal(pn.Message.Data, &n); err != nil {
		log.Printf("Failed to decode AR payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	i := Image{Repository: "", Tag: n.Tag, Digest: n.Digest}

	if i.Digest == "" || i.Tag == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check to see if this image has any dependencies.
	ref, err := name.ParseReference(i.Digest)
	if err != nil {
		log.Printf("parse(%s) = %v", i.Digest, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("DEMO: %s@%s", n.Tag, ref.Identifier())

	// Only handle INSERT notifications.
	if n.Action != "INSERT" {
		if n.Action == "DELETE" {
			if err := deleteImage(r.Context(), i); err != nil {
				fmt.Printf("Failed to delete image %v: %v", i, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// INSERT into images table.
	if err := addImage(r.Context(), i); err != nil {
		fmt.Printf("Failed to add image %v to spanner: %v", i, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Printf("Added image %v to spanner...\n", i)
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(google.Keychain))
	if err != nil {
		log.Printf("pull(%s) = %v", ref, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	m, err := img.Manifest()
	if err != nil {
		log.Printf("img.Manifest() = %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(m.Annotations) != 0 {
		if baseDigest, ok := m.Annotations["org.opencontainers.image.base.digest"]; ok {
			if baseName, ok := m.Annotations["org.opencontainers.image.base.name"]; ok {
				baseRef, err := name.ParseReference(baseName)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				log.Printf("DEMO: %s depends on %s@%s", ref, baseRef, baseDigest)
				// TODO: Set up a pubsub subscription (if needed for cross-project stuff).
				wr := WatchReq{
					SourceDigest: ref.Identifier(),
					BaseDigest:   baseDigest,
					BaseRef:      baseName,
				}

				if err := addDependency(r.Context(), wr); err != nil {
					fmt.Printf("Failed to add dependency %v to spanner: %v", wr, err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				fmt.Printf("Added dependency %v to spanner...\n", wr)

				// TODO: See if this new image depends on an old tag.
				desc, err := remote.Head(baseRef, remote.WithAuthFromKeychain(google.Keychain))
				if err != nil {
					log.Printf("head(%s) = %v", ref, err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				if got, want := desc.Digest.String(), baseDigest; got != want {
					log.Printf("DEMO %s depends on %s but is out of date; got %s want %s", ref, baseRef, got, want)
					if ref.String() == servTag {
						res := topic.Publish(r.Context(), &pubsub.Message{Data: []byte(baseRef.String())})
						id, err := res.Get(r.Context())
						if err != nil {
							log.Printf("Publish(%s): %v", baseRef, err)
						}
						log.Printf("Published %s", id)
					}
				}
			}
		}
	}

	// Get all images that depend on this tag with the wrong digest.
	stmt := spanner.NewStatement("SELECT i.Tag, i.Digest FROM Images i JOIN Dependencies d ON d.BaseRef = i.Tag WHERE d.BaseRef = @baseRef AND d.BaseDigest != @baseDigest")
	stmt.Params["baseRef"] = i.Tag
	stmt.Params["baseDigest"] = ref.Identifier()
	iter := dbClient.Single().Query(r.Context(), stmt)
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			w.WriteHeader(http.StatusInternalServerError)
			break
		}
		if err != nil {
			fmt.Printf("iter.Next() = %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var tag, digest string
		if err := row.Columns(&tag, &digest); err != nil {
			fmt.Printf("row.Columns() = %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		got, want := digest, ref.Identifier()
		log.Printf("DEMO: %s depends on %s but is out of date; got %s want %s", tag, i.Tag, got, want)
		if tag == servTag {
			res := topic.Publish(r.Context(), &pubsub.Message{Data: []byte(i.Tag)})
			id, err := res.Get(r.Context())
			if err != nil {
				log.Printf("Publish(%s): %v", i.Tag, err)
			}
			log.Printf("Published %s", id)
		}
	}
	defer iter.Stop()

	w.WriteHeader(http.StatusOK)
}
