provider "google" {
    project = "independency-day"
}

resource "google_artifact_registry_repository" "my-repo" {
  provider = google-beta
  
  project = "independency-day"
  location = "us-central1"
  repository_id = "my-repository"
  description = "testing to see if terraform can create an AR repo"
  format = "DOCKER"
}
