#!/usr/bin/env bash
# Manage demo GCP resources for recording gcon GIFs.
# Usage: ./resources.sh setup | teardown
# Reads configuration from .envrc (via direnv or manual source).
set -euo pipefail

ACTION="${1:-}"
if [[ "$ACTION" != "setup" && "$ACTION" != "teardown" ]]; then
    echo "Usage: $0 <setup|teardown>"
    exit 1
fi

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

info()    { echo -e "    ${BLUE}$*${NC}"; }
success() { echo -e "    ${GREEN}$*${NC}"; }
warn()    { echo -e "    ${YELLOW}$*${NC}"; }
error()   { echo -e "    ${RED}$*${NC}"; }
header()  { echo -e "${BOLD}>>> $*${NC}"; }
banner()  { echo -e "\n${BOLD}${BLUE}=== $* ===${NC}\n"; }

# Verify required env vars
for var in DEMO_PROJECT DEMO_REGION DEMO_ZONE DEMO_SA_NAME DEMO_BUCKET \
           DEMO_SQL_INSTANCE DEMO_VM DEMO_CLOUDRUN_SVC DEMO_FIREWALL \
           DEMO_ROUTE DEMO_CUSTOM_ROLE; do
    if [[ -z "${!var:-}" ]]; then
        error "Error: $var is not set. Copy .envrc.example to .envrc and fill in values."
        exit 1
    fi
done

PROJECT="$DEMO_PROJECT"
REGION="$DEMO_REGION"
ZONE="$DEMO_ZONE"
SA_EMAIL="${DEMO_SA_NAME}@${PROJECT}.iam.gserviceaccount.com"

# Resource management uses your active gcloud credentials (needs write access).
# The SA key (gcon-sa.json) is only used by VHS tapes for gcon auth during recording.
gcloud config set project "$PROJECT" --quiet

# --- Helper: check if resource exists ---
exists() {
    "$@" &>/dev/null
}

# --- Enable required APIs (setup only) ---
enable_apis() {
    header "Enabling required APIs"
    gcloud services enable \
        compute.googleapis.com \
        run.googleapis.com \
        sqladmin.googleapis.com \
        storage.googleapis.com \
        iam.googleapis.com \
        cloudresourcemanager.googleapis.com \
        monitoring.googleapis.com \
        logging.googleapis.com \
        --project="$PROJECT" \
        --quiet
    success "APIs enabled."
}

# --- Resource definitions ---
# Each function handles both setup and teardown for its resource.

resource_vm() {
    local desc="e2-micro VM: $DEMO_VM"
    if [[ "$ACTION" == "setup" ]]; then
        header "Creating $desc"
        if exists gcloud compute instances describe "$DEMO_VM" --zone="$ZONE" --project="$PROJECT"; then
            warn "Already exists, skipping."
        else
            gcloud compute instances create "$DEMO_VM" \
                --zone="$ZONE" \
                --machine-type=e2-micro \
                --image-family=debian-12 \
                --image-project=debian-cloud \
                --boot-disk-size=10GB \
                --tags=http-server \
                --labels=env=demo,app=gcon \
                --project="$PROJECT" \
                --quiet
            success "Created."
        fi
    else
        header "Deleting $desc"
        if exists gcloud compute instances describe "$DEMO_VM" --zone="$ZONE" --project="$PROJECT"; then
            gcloud compute instances delete "$DEMO_VM" \
                --zone="$ZONE" --project="$PROJECT" --quiet
            success "Deleted."
        else
            warn "Not found, skipping."
        fi
    fi
}

resource_bucket() {
    local desc="GCS bucket: $DEMO_BUCKET"
    if [[ "$ACTION" == "setup" ]]; then
        header "Creating $desc"
        if exists gcloud storage buckets describe "gs://$DEMO_BUCKET" --project="$PROJECT"; then
            warn "Already exists, skipping."
        else
            gcloud storage buckets create "gs://$DEMO_BUCKET" \
                --location="$REGION" \
                --default-storage-class=STANDARD \
                --uniform-bucket-level-access \
                --project="$PROJECT" \
                --quiet
            success "Created."
        fi
        header "Uploading sample objects"
        echo "# gcon Demo Bucket" | gcloud storage cp - "gs://$DEMO_BUCKET/readme.md" --quiet
        echo '{"name": "sample", "version": 1}' | gcloud storage cp - "gs://$DEMO_BUCKET/data/sample.json" --quiet
        echo "2026-03-27 INFO Application started" | gcloud storage cp - "gs://$DEMO_BUCKET/logs/app.log" --quiet
        success "Uploaded 3 objects."
    else
        header "Deleting $desc"
        if exists gcloud storage buckets describe "gs://$DEMO_BUCKET" --project="$PROJECT"; then
            gcloud storage rm -r "gs://$DEMO_BUCKET" --quiet
            success "Deleted."
        else
            warn "Not found, skipping."
        fi
    fi
}

resource_firewall() {
    local desc="firewall rule: $DEMO_FIREWALL"
    if [[ "$ACTION" == "setup" ]]; then
        header "Creating $desc"
        if exists gcloud compute firewall-rules describe "$DEMO_FIREWALL" --project="$PROJECT"; then
            warn "Already exists, skipping."
        else
            gcloud compute firewall-rules create "$DEMO_FIREWALL" \
                --network=default \
                --allow=tcp:80,tcp:443 \
                --source-ranges=0.0.0.0/0 \
                --target-tags=http-server \
                --description="Allow HTTP/HTTPS for gcon demo" \
                --project="$PROJECT" \
                --quiet
            success "Created."
        fi
    else
        header "Deleting $desc"
        if exists gcloud compute firewall-rules describe "$DEMO_FIREWALL" --project="$PROJECT"; then
            gcloud compute firewall-rules delete "$DEMO_FIREWALL" \
                --project="$PROJECT" --quiet
            success "Deleted."
        else
            warn "Not found, skipping."
        fi
    fi
}

resource_route() {
    local desc="static route: $DEMO_ROUTE"
    if [[ "$ACTION" == "setup" ]]; then
        header "Creating $desc"
        if exists gcloud compute routes describe "$DEMO_ROUTE" --project="$PROJECT"; then
            warn "Already exists, skipping."
        else
            gcloud compute routes create "$DEMO_ROUTE" \
                --network=default \
                --destination-range=10.99.0.0/16 \
                --next-hop-gateway=default-internet-gateway \
                --priority=900 \
                --description="Demo static route for gcon" \
                --project="$PROJECT" \
                --quiet
            success "Created."
        fi
    else
        header "Deleting $desc"
        if exists gcloud compute routes describe "$DEMO_ROUTE" --project="$PROJECT"; then
            gcloud compute routes delete "$DEMO_ROUTE" \
                --project="$PROJECT" --quiet
            success "Deleted."
        else
            warn "Not found, skipping."
        fi
    fi
}

resource_service_account() {
    local desc="service account: $DEMO_SA_NAME"
    if [[ "$ACTION" == "setup" ]]; then
        header "Creating $desc"
        if exists gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT"; then
            warn "Already exists, skipping."
        else
            gcloud iam service-accounts create "$DEMO_SA_NAME" \
                --display-name="gcon Demo Service Account" \
                --description="Service account for gcon demo recordings" \
                --project="$PROJECT" \
                --quiet
            success "Created."
        fi
        header "Adding IAM binding: roles/viewer for $SA_EMAIL"
        gcloud projects add-iam-policy-binding "$PROJECT" \
            --member="serviceAccount:$SA_EMAIL" \
            --role="roles/viewer" \
            --condition=None \
            --quiet >/dev/null
        success "Binding added."
    else
        header "Removing IAM binding for: $SA_EMAIL"
        if exists gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT"; then
            gcloud projects remove-iam-policy-binding "$PROJECT" \
                --member="serviceAccount:$SA_EMAIL" \
                --role="roles/viewer" \
                --quiet >/dev/null 2>&1 || true
            success "Binding removed."
        fi
        header "Deleting $desc"
        if exists gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT"; then
            gcloud iam service-accounts delete "$SA_EMAIL" \
                --project="$PROJECT" --quiet
            success "Deleted."
        else
            warn "Not found, skipping."
        fi
    fi
}

resource_custom_role() {
    local desc="custom role: $DEMO_CUSTOM_ROLE"
    if [[ "$ACTION" == "setup" ]]; then
        header "Creating $desc"
        # Custom roles are soft-deleted in GCP; undelete if previously deleted
        local role_info
        if role_info=$(gcloud iam roles describe "$DEMO_CUSTOM_ROLE" --project="$PROJECT" 2>/dev/null); then
            if echo "$role_info" | grep -q "deleted: true"; then
                info "Soft-deleted, undeleting..."
                gcloud iam roles undelete "$DEMO_CUSTOM_ROLE" \
                    --project="$PROJECT" --quiet
                success "Undeleted."
            else
                warn "Already exists, skipping."
            fi
        else
            gcloud iam roles create "$DEMO_CUSTOM_ROLE" \
                --project="$PROJECT" \
                --title="gcon Demo Viewer" \
                --description="Custom role for gcon demo recordings" \
                --permissions=compute.instances.get,compute.instances.list,storage.buckets.list \
                --stage=GA \
                --quiet
            success "Created."
        fi
    else
        header "Deleting $desc"
        # Check role exists and is not already soft-deleted
        local role_info
        if role_info=$(gcloud iam roles describe "$DEMO_CUSTOM_ROLE" --project="$PROJECT" 2>/dev/null); then
            if echo "$role_info" | grep -q "deleted: true"; then
                warn "Already deleted, skipping."
            else
                gcloud iam roles delete "$DEMO_CUSTOM_ROLE" \
                    --project="$PROJECT" --quiet
                success "Deleted."
            fi
        else
            warn "Not found, skipping."
        fi
    fi
}

resource_cloudrun() {
    local desc="Cloud Run service: $DEMO_CLOUDRUN_SVC"
    if [[ "$ACTION" == "setup" ]]; then
        header "Deploying $desc"
        if exists gcloud run services describe "$DEMO_CLOUDRUN_SVC" --region="$REGION" --project="$PROJECT"; then
            warn "Already exists, skipping."
        else
            gcloud run deploy "$DEMO_CLOUDRUN_SVC" \
                --image=us-docker.pkg.dev/cloudrun/container/hello \
                --region="$REGION" \
                --allow-unauthenticated \
                --cpu=1 \
                --memory=256Mi \
                --max-instances=2 \
                --project="$PROJECT" \
                --quiet
            success "Deployed."
        fi
    else
        header "Deleting $desc"
        if exists gcloud run services describe "$DEMO_CLOUDRUN_SVC" --region="$REGION" --project="$PROJECT"; then
            gcloud run services delete "$DEMO_CLOUDRUN_SVC" \
                --region="$REGION" --project="$PROJECT" --quiet
            success "Deleted."
        else
            warn "Not found, skipping."
        fi
    fi
}

resource_cloudsql() {
    local desc="Cloud SQL instance: $DEMO_SQL_INSTANCE"
    if [[ "$ACTION" == "setup" ]]; then
        header "Creating $desc (this takes a few minutes)"
        if exists gcloud sql instances describe "$DEMO_SQL_INSTANCE" --project="$PROJECT"; then
            warn "Already exists, skipping."
        else
            gcloud sql instances create "$DEMO_SQL_INSTANCE" \
                --database-version=POSTGRES_17 \
                --tier=db-f1-micro \
                --region="$REGION" \
                --storage-size=10GB \
                --storage-type=HDD \
                --assign-ip \
                --project="$PROJECT" \
                --quiet
            success "Created."
            header "Creating demo database"
            gcloud sql databases create "gcon_demo_db" \
                --instance="$DEMO_SQL_INSTANCE" \
                --project="$PROJECT" \
                --quiet
            success "Database created."
        fi
    else
        header "Deleting $desc"
        if exists gcloud sql instances describe "$DEMO_SQL_INSTANCE" --project="$PROJECT"; then
            gcloud sql instances delete "$DEMO_SQL_INSTANCE" \
                --project="$PROJECT" --quiet
            success "Deleted."
        else
            warn "Not found, skipping."
        fi
    fi
}

# --- Orchestrate ---
banner "${ACTION^} demo resources in project: $PROJECT"

if [[ "$ACTION" == "setup" ]]; then
    enable_apis
    echo ""
    resource_vm
    echo ""
    resource_bucket
    echo ""
    resource_firewall
    echo ""
    resource_route
    echo ""
    resource_service_account
    echo ""
    resource_custom_role
    echo ""
    resource_cloudrun
    echo ""
    resource_cloudsql

    banner "Demo setup complete"
    info "Resources created:"
    echo -e "  VM:              ${GREEN}$DEMO_VM${NC} ($ZONE)"
    echo -e "  Bucket:          ${GREEN}gs://$DEMO_BUCKET${NC}"
    echo -e "  Firewall:        ${GREEN}$DEMO_FIREWALL${NC}"
    echo -e "  Route:           ${GREEN}$DEMO_ROUTE${NC}"
    echo -e "  Service Account: ${GREEN}$SA_EMAIL${NC}"
    echo -e "  Custom Role:     ${GREEN}$DEMO_CUSTOM_ROLE${NC}"
    echo -e "  Cloud Run:       ${GREEN}$DEMO_CLOUDRUN_SVC${NC} ($REGION)"
    echo -e "  Cloud SQL:       ${GREEN}$DEMO_SQL_INSTANCE${NC} ($REGION)"
    echo ""
    info "Wait 10-15 minutes for metrics/logs to accumulate, then run: make demos"
else
    # Teardown slowest first
    resource_cloudsql
    echo ""
    resource_cloudrun
    echo ""
    resource_vm
    echo ""
    resource_route
    echo ""
    resource_firewall
    echo ""
    resource_bucket
    echo ""
    resource_custom_role
    echo ""
    resource_service_account

    banner "Demo teardown complete"
fi
