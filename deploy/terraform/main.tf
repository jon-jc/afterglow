terraform {
  required_version = ">= 1.8"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
provider "google-beta" {
  project = var.project_id
  region  = var.region
}

variable "project_id" { type = string }
variable "region" {
  type    = string
  default = "us-west1"
}
variable "image" {
  type        = string
  description = "Prebuilt Artifact Registry image pinned to a digest."
}
variable "sql_connection_name" {
  type        = string
  description = "Existing Cloud SQL instance, project:region:instance."
}
variable "database_secret_id" {
  type        = string
  description = "Existing Secret Manager secret containing PostgreSQL DSN with /cloudsql socket host."
}
variable "api_key_secret_id" {
  type        = string
  description = "Existing Secret Manager secret containing at least 24 random characters."
}
variable "tenant_id" { type = string }
variable "invoker" {
  type        = string
  description = "IAM member allowed to invoke the API, e.g. user:you@example.com. Never allUsers."
  validation {
    condition     = startswith(var.invoker, "user:") || startswith(var.invoker, "serviceAccount:") || startswith(var.invoker, "group:")
    error_message = "Use a named user, group or service account."
  }
}

data "google_project" "current" { project_id = var.project_id }

resource "google_project_service" "required" {
  for_each           = toset(["run.googleapis.com", "pubsub.googleapis.com", "sqladmin.googleapis.com", "secretmanager.googleapis.com"])
  service            = each.value
  disable_on_destroy = false
}
resource "google_project_service_identity" "pubsub" {
  provider   = google-beta
  project    = var.project_id
  service    = "pubsub.googleapis.com"
  depends_on = [google_project_service.required]
}
resource "google_service_account" "app" {
  for_each   = toset(["api", "worker"])
  account_id = "afterglow-${each.key}"
}
resource "google_pubsub_topic" "receipts" {
  name       = "afterglow-receipts"
  depends_on = [google_project_service.required]
}
resource "google_pubsub_topic" "dead" {
  name       = "afterglow-dead-letter"
  depends_on = [google_project_service.required]
}
resource "google_pubsub_subscription" "worker" {
  name                       = "afterglow-reconciler"
  topic                      = google_pubsub_topic.receipts.id
  ack_deadline_seconds       = 30
  message_retention_duration = "604800s"
  expiration_policy { ttl = "" }
  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "300s"
  }
  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.dead.id
    max_delivery_attempts = 10
  }
}
resource "google_pubsub_subscription" "dead" {
  name                       = "afterglow-dead-letter-review"
  topic                      = google_pubsub_topic.dead.id
  message_retention_duration = "1209600s"
  expiration_policy { ttl = "" }
}
resource "google_pubsub_topic_iam_member" "worker_publisher" {
  topic  = google_pubsub_topic.receipts.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${google_service_account.app["worker"].email}"
}
resource "google_pubsub_subscription_iam_member" "worker_subscriber" {
  subscription = google_pubsub_subscription.worker.name
  role         = "roles/pubsub.subscriber"
  member       = "serviceAccount:${google_service_account.app["worker"].email}"
}
resource "google_pubsub_topic_iam_member" "dead_letter_publisher" {
  topic  = google_pubsub_topic.dead.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${google_project_service_identity.pubsub.email}"
}
resource "google_pubsub_subscription_iam_member" "dead_letter_subscriber" {
  subscription = google_pubsub_subscription.worker.name
  role         = "roles/pubsub.subscriber"
  member       = "serviceAccount:${google_project_service_identity.pubsub.email}"
}
resource "google_project_iam_member" "sql" {
  for_each = google_service_account.app
  project  = var.project_id
  role     = "roles/cloudsql.client"
  member   = "serviceAccount:${each.value.email}"
}
resource "google_secret_manager_secret_iam_member" "db" {
  for_each  = google_service_account.app
  secret_id = var.database_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${each.value.email}"
}
resource "google_secret_manager_secret_iam_member" "key" {
  for_each  = google_service_account.app
  secret_id = var.api_key_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${each.value.email}"
}
resource "google_cloud_run_v2_service" "app" {
  for_each            = toset(["api", "worker"])
  name                = "afterglow-${each.key}"
  location            = var.region
  deletion_protection = true
  ingress             = "INGRESS_TRAFFIC_ALL"
  template {
    service_account = google_service_account.app[each.key].email
    timeout         = "30s"
    scaling {
      min_instance_count = each.key == "worker" ? 1 : 0
      max_instance_count = each.key == "worker" ? 3 : 5
    }
    volumes {
      name = "cloudsql"
      cloud_sql_instance { instances = [var.sql_connection_name] }
    }
    containers {
      image = var.image
      ports { container_port = 8080 }
      resources {
        limits = { cpu = "1", memory = "512Mi" }
        # Pull subscribers require CPU even when no HTTP request is active.
        cpu_idle = each.key == "api"
      }
      volume_mounts {
        name       = "cloudsql"
        mount_path = "/cloudsql"
      }
      dynamic "env" {
        for_each = {
          DEMO_MODE           = "false"
          ADDR                = "0.0.0.0:8080"
          ROLE                = each.key
          TRANSPORT           = "pubsub"
          GCP_PROJECT_ID      = var.project_id
          PUBSUB_TOPIC        = google_pubsub_topic.receipts.name
          PUBSUB_SUBSCRIPTION = google_pubsub_subscription.worker.name
          TENANT_ID           = var.tenant_id
        }
        content {
          name  = env.key
          value = env.value
        }
      }
      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = var.database_secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "API_KEY"
        value_source {
          secret_key_ref {
            secret  = var.api_key_secret_id
            version = "latest"
          }
        }
      }
      startup_probe {
        http_get { path = "/readyz" }
        period_seconds    = 10
        failure_threshold = 12
      }
      liveness_probe {
        http_get { path = "/healthz" }
        period_seconds = 30
      }
    }
  }
  depends_on = [google_project_service.required, google_project_iam_member.sql, google_secret_manager_secret_iam_member.db, google_secret_manager_secret_iam_member.key, google_pubsub_topic_iam_member.worker_publisher, google_pubsub_subscription_iam_member.worker_subscriber]
}
resource "google_cloud_run_v2_service_iam_member" "invoker" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.app["api"].name
  role     = "roles/run.invoker"
  member   = var.invoker
}
output "api_url" { value = google_cloud_run_v2_service.app["api"].uri }
