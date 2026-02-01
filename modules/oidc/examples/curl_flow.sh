#!/bin/bash

# OIDC Authorization Code Flow Example
# This script demonstrates the complete OIDC flow using curl

set -e

# Configuration
BASE_URL="${OIDC_BASE_URL:-http://localhost:8080}"
CLIENT_ID="${OIDC_CLIENT_ID:-test-client}"
CLIENT_SECRET="${OIDC_CLIENT_SECRET:-}"
REDIRECT_URI="${OIDC_REDIRECT_URI:-http://localhost:3000/callback}"
SCOPES="openid profile email"
USE_PKCE="${OIDC_USE_PKCE:-true}"

echo "======================================"
echo "OIDC Authorization Code Flow Example"
echo "======================================"
echo ""
echo "Configuration:"
echo "  Base URL: $BASE_URL"
echo "  Client ID: $CLIENT_ID"
echo "  Redirect URI: $REDIRECT_URI"
echo "  Scopes: $SCOPES"
echo "  Use PKCE: $USE_PKCE"
echo ""

# Step 1: Generate PKCE parameters (if enabled)
if [ "$USE_PKCE" = "true" ]; then
    echo "Step 1: Generating PKCE parameters..."

    # Generate code verifier (43-128 character random string)
    CODE_VERIFIER=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-43)
    echo "  Code Verifier: $CODE_VERIFIER"

    # Generate code challenge (SHA256 hash of verifier, base64url encoded)
    CODE_CHALLENGE=$(echo -n "$CODE_VERIFIER" | shasum -a 256 | awk '{print $1}' | xxd -r -p | base64 | tr -d "=+/" | tr -d "\n")
    echo "  Code Challenge: $CODE_CHALLENGE"
    echo ""
else
    CODE_VERIFIER=""
    CODE_CHALLENGE=""
    echo "Step 1: PKCE disabled, skipping..."
    echo ""
fi

# Step 2: Generate state parameter
echo "Step 2: Generating state parameter..."
STATE=$(openssl rand -hex 16)
echo "  State: $STATE"
echo ""

# Step 3: Build authorization URL
echo "Step 3: Building authorization URL..."
AUTH_URL="$BASE_URL/oidc/authorize"
AUTH_URL+="?client_id=$(echo -n "$CLIENT_ID" | jq -sRr @uri)"
AUTH_URL+="&redirect_uri=$(echo -n "$REDIRECT_URI" | jq -sRr @uri)"
AUTH_URL+="&response_type=code"
AUTH_URL+="&scope=$(echo -n "$SCOPES" | jq -sRr @uri)"
AUTH_URL+="&state=$STATE"

if [ "$USE_PKCE" = "true" ]; then
    AUTH_URL+="&code_challenge=$CODE_CHALLENGE"
    AUTH_URL+="&code_challenge_method=S256"
fi

echo "  Authorization URL:"
echo "  $AUTH_URL"
echo ""

# Step 4: User authorization (manual step)
echo "Step 4: User Authorization Required"
echo "----------------------------------------"
echo "Please open the following URL in your browser and complete the login:"
echo ""
echo "  $AUTH_URL"
echo ""
echo "After successful login, you will be redirected to:"
echo "  $REDIRECT_URI?code=AUTHORIZATION_CODE&state=$STATE"
echo ""
read -p "Enter the authorization code from the redirect URL: " AUTH_CODE
echo ""

# Validate state (in real application)
echo "Step 5: Validating state parameter..."
echo "  (In production, verify state matches $STATE)"
echo ""

# Step 6: Exchange authorization code for tokens
echo "Step 6: Exchanging authorization code for tokens..."

TOKEN_REQUEST_DATA="grant_type=authorization_code"
TOKEN_REQUEST_DATA+="&code=$AUTH_CODE"
TOKEN_REQUEST_DATA+="&redirect_uri=$(echo -n "$REDIRECT_URI" | jq -sRr @uri)"

if [ "$USE_PKCE" = "true" ]; then
    TOKEN_REQUEST_DATA+="&code_verifier=$CODE_VERIFIER"
    TOKEN_REQUEST_DATA+="&client_id=$CLIENT_ID"
    AUTH_HEADER=""
else
    # Use client secret authentication
    if [ -n "$CLIENT_SECRET" ]; then
        AUTH_HEADER="-u $CLIENT_ID:$CLIENT_SECRET"
    else
        TOKEN_REQUEST_DATA+="&client_id=$CLIENT_ID"
        AUTH_HEADER=""
    fi
fi

echo "  Making token request..."
TOKEN_RESPONSE=$(curl -s -X POST "$BASE_URL/oidc/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    $AUTH_HEADER \
    -d "$TOKEN_REQUEST_DATA")

echo "  Token Response:"
echo "$TOKEN_RESPONSE" | jq '.'
echo ""

# Extract tokens
ACCESS_TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.access_token')
REFRESH_TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.refresh_token')
ID_TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.id_token')

if [ "$ACCESS_TOKEN" = "null" ] || [ -z "$ACCESS_TOKEN" ]; then
    echo "Error: Failed to obtain access token"
    echo "Response: $TOKEN_RESPONSE"
    exit 1
fi

echo "  Access Token: ${ACCESS_TOKEN:0:50}..."
echo "  Refresh Token: ${REFRESH_TOKEN:0:50}..."
echo "  ID Token: ${ID_TOKEN:0:50}..."
echo ""

# Step 7: Get user info
echo "Step 7: Fetching user info..."
USERINFO_RESPONSE=$(curl -s -X GET "$BASE_URL/oidc/userinfo" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

echo "  User Info:"
echo "$USERINFO_RESPONSE" | jq '.'
echo ""

# Step 8: Decode ID token (optional)
echo "Step 8: Decoding ID token..."
echo "  Header:"
echo "$ID_TOKEN" | cut -d. -f1 | base64 -d 2>/dev/null | jq '.' || echo "  (Unable to decode header)"
echo ""
echo "  Payload:"
echo "$ID_TOKEN" | cut -d. -f2 | base64 -d 2>/dev/null | jq '.' || echo "  (Unable to decode payload)"
echo ""

# Step 9: Refresh access token
echo "Step 9: Refreshing access token..."
if [ "$REFRESH_TOKEN" != "null" ] && [ -n "$REFRESH_TOKEN" ]; then
    REFRESH_REQUEST_DATA="grant_type=refresh_token"
    REFRESH_REQUEST_DATA+="&refresh_token=$REFRESH_TOKEN"

    if [ -n "$CLIENT_SECRET" ]; then
        AUTH_HEADER="-u $CLIENT_ID:$CLIENT_SECRET"
    else
        REFRESH_REQUEST_DATA+="&client_id=$CLIENT_ID"
        AUTH_HEADER=""
    fi

    REFRESH_RESPONSE=$(curl -s -X POST "$BASE_URL/oidc/token" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        $AUTH_HEADER \
        -d "$REFRESH_REQUEST_DATA")

    echo "  Refresh Response:"
    echo "$REFRESH_RESPONSE" | jq '.'
    echo ""

    NEW_ACCESS_TOKEN=$(echo "$REFRESH_RESPONSE" | jq -r '.access_token')
    echo "  New Access Token: ${NEW_ACCESS_TOKEN:0:50}..."
else
    echo "  No refresh token available (offline_access scope may not be granted)"
fi
echo ""

# Step 10: Revoke refresh token
echo "Step 10: Revoking refresh token (optional)..."
read -p "Revoke refresh token? (y/n): " REVOKE_CHOICE
if [ "$REVOKE_CHOICE" = "y" ]; then
    REVOKE_REQUEST_DATA="token=$REFRESH_TOKEN"
    REVOKE_REQUEST_DATA+="&token_type_hint=refresh_token"

    if [ -n "$CLIENT_SECRET" ]; then
        AUTH_HEADER="-u $CLIENT_ID:$CLIENT_SECRET"
    else
        REVOKE_REQUEST_DATA+="&client_id=$CLIENT_ID"
        AUTH_HEADER=""
    fi

    REVOKE_RESPONSE=$(curl -s -w "\nHTTP Status: %{http_code}\n" -X POST "$BASE_URL/oidc/revoke" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        $AUTH_HEADER \
        -d "$REVOKE_REQUEST_DATA")

    echo "  Revoke Response:"
    echo "$REVOKE_RESPONSE"
    echo ""
fi

echo "======================================"
echo "OIDC Flow Complete!"
echo "======================================"
echo ""
echo "Summary:"
echo "  ✓ Authorization code obtained"
echo "  ✓ Tokens exchanged successfully"
echo "  ✓ User info retrieved"
echo "  ✓ ID token decoded"
if [ "$REFRESH_TOKEN" != "null" ]; then
    echo "  ✓ Token refresh successful"
fi
echo ""
echo "Tokens saved to environment variables:"
echo "  export OIDC_ACCESS_TOKEN='$ACCESS_TOKEN'"
echo "  export OIDC_REFRESH_TOKEN='$REFRESH_TOKEN'"
echo "  export OIDC_ID_TOKEN='$ID_TOKEN'"
