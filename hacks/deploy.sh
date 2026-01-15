#!/bin/bash
# Deploy the discordian Discord bot to Google Cloud Run
#
# usage:
#   ./hacks/deploy.sh              - deploy with default app name
#   APP_NAME=my-bot ./hacks/deploy.sh  - deploy with custom name
set -eux -o pipefail

cd "$(dirname "$0")/.."

PROJECT=${GCP_PROJECT:=chat-bot-army}
REGISTRY="${PROJECT}"
REGION="us-central1"

# App name defaults to 'discordian'
APP_NAME=${APP_NAME:-discordian}

APP_USER="${APP_NAME}@${PROJECT}.iam.gserviceaccount.com"
APP_IMAGE="gcr.io/${REGISTRY}/${APP_NAME}"

# Create service account if it doesn't exist
gcloud iam service-accounts list --project "${PROJECT}" | grep -q "${APP_USER}" ||
	gcloud iam service-accounts create "${APP_NAME}" --project "${PROJECT}"

echo "Deploying app: ${APP_NAME}"
echo "Service account: ${APP_USER}"
echo "Image: ${APP_IMAGE}"

export KO_DOCKER_REPO="${APP_IMAGE}"
gcloud run deploy "${APP_NAME}" \
	--image="$(ko publish ./cmd/server)" \
	--region="${REGION}" \
	--service-account="${APP_USER}" \
	--project "${PROJECT}" \
	--allow-unauthenticated
