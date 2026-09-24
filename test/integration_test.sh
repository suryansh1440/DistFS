#!/bin/bash
set -e

API="http://localhost:8080/api"
PASS=0
FAIL=0

green() { echo -e "\033[0;32m✓ $1\033[0m"; PASS=$((PASS+1)); }
red() { echo -e "\033[0;31m✗ $1\033[0m"; FAIL=$((FAIL+1)); }

echo "═══════════════════════════════════════════"
echo " Distributed File Storage — Integration Tests"
echo "═══════════════════════════════════════════"
echo ""

# --- Health Check ---
echo "▸ Test: Health check"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" $API/health)
[ "$STATUS" = "200" ] && green "Health check OK" || red "Health check failed ($STATUS)"

# --- List Nodes ---
echo "▸ Test: List nodes"
NODE_COUNT=$(curl -s $API/nodes | jq length)
[ "$NODE_COUNT" = "4" ] && green "4 nodes found" || red "Expected 4 nodes, got $NODE_COUNT"

# --- Create test file ---
echo "▸ Test: Upload file"
echo "Reed-Solomon test data — $(date)" > /tmp/test_upload.txt
UPLOAD_RESPONSE=$(curl -s -X POST $API/files/upload -F "file=@/tmp/test_upload.txt")
FILE_ID=$(echo $UPLOAD_RESPONSE | jq -r '.file_id')
ORIGINAL_CHECKSUM=$(echo $UPLOAD_RESPONSE | jq -r '.checksum')
[ "$FILE_ID" != "null" ] && green "File uploaded: $FILE_ID" || red "Upload failed"

# --- Download (all nodes online) ---
echo "▸ Test: Download (all nodes healthy)"
curl -s $API/files/$FILE_ID/download -o /tmp/test_download.txt
DL_CHECKSUM=$(sha256sum /tmp/test_download.txt | awk '{print $1}')
[ "$DL_CHECKSUM" = "$ORIGINAL_CHECKSUM" ] && green "Download checksum matches" || red "Checksum mismatch"

# --- Stop Node 2 ---
echo "▸ Test: Stop node-2"
curl -s -X POST $API/nodes/node-2/stop > /dev/null
NODE2_STATUS=$(curl -s $API/nodes/node-2 | jq -r '.status')
[ "$NODE2_STATUS" = "OFFLINE" ] && green "Node-2 is OFFLINE" || red "Node-2 status: $NODE2_STATUS"

# --- Download with reconstruction ---
echo "▸ Test: Download with reconstruction (node-2 offline)"
curl -s -D /tmp/headers.txt $API/files/$FILE_ID/download -o /tmp/test_reconstructed.txt
RECON_CHECKSUM=$(sha256sum /tmp/test_reconstructed.txt | awk '{print $1}')
[ "$RECON_CHECKSUM" = "$ORIGINAL_CHECKSUM" ] && green "Reconstructed checksum matches" || red "Reconstruction checksum mismatch"

# --- Stop Node 3 (should make download impossible) ---
echo "▸ Test: Stop node-3 (2 nodes offline)"
curl -s -X POST $API/nodes/node-3/stop > /dev/null
DL_STATUS=$(curl -s -o /dev/null -w "%{http_code}" $API/files/$FILE_ID/download)
[ "$DL_STATUS" = "503" ] && green "Download correctly refused (503)" || red "Expected 503, got $DL_STATUS"

# --- Restart nodes ---
echo "▸ Test: Restart nodes"
curl -s -X POST $API/nodes/node-2/start > /dev/null
curl -s -X POST $API/nodes/node-3/start > /dev/null
ONLINE_COUNT=$(curl -s $API/nodes | jq '[.[] | select(.status == "ONLINE")] | length')
[ "$ONLINE_COUNT" = "4" ] && green "All nodes back online" || red "Only $ONLINE_COUNT nodes online"

# --- Delete file ---
echo "▸ Test: Delete file"
DEL_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $API/files/$FILE_ID)
[ "$DEL_STATUS" = "200" ] && green "File deleted" || red "Delete failed ($DEL_STATUS)"

# --- Verify deletion ---
echo "▸ Test: Verify file deleted"
GET_STATUS=$(curl -s -o /dev/null -w "%{http_code}" $API/files/$FILE_ID)
[ "$GET_STATUS" = "404" ] && green "File not found (correctly deleted)" || red "File still exists ($GET_STATUS)"

echo ""
echo "═══════════════════════════════════════════"
echo " Results: $PASS passed, $FAIL failed"
echo "═══════════════════════════════════════════"

# Cleanup
rm -f /tmp/test_upload.txt /tmp/test_download.txt /tmp/test_reconstructed.txt /tmp/headers.txt

exit $FAIL
