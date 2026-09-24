# Step 11 — End-to-End Integration Testing

## Goal
Create a comprehensive test script and manual test procedures that verify the entire system works correctly end-to-end.

---

## Test Scenarios

### Test 1 — Happy Path Upload & Download

**Purpose**: Verify basic upload, chunking, RS encoding, distribution, and clean download.

```bash
# 1. Create a test file
echo "Hello Distributed File Storage with Reed-Solomon Erasure Coding!" > test.txt
# Or generate a larger file:
# dd if=/dev/urandom of=test_5mb.bin bs=1M count=5

# 2. Calculate original checksum
sha256sum test.txt
# → e.g., a1b2c3d4...

# 3. Upload
curl -X POST http://localhost:8080/api/files/upload \
    -F "file=@test.txt" | jq .

# Expected response:
# {
#   "file_id": "uuid-here",
#   "filename": "test.txt",
#   "file_size": 65,
#   "checksum": "a1b2c3d4...",
#   "chunks": 1,
#   "shards": 4,
#   "status": "HEALTHY"
# }

# 4. List files
curl http://localhost:8080/api/files | jq .

# 5. Download
curl http://localhost:8080/api/files/<file_id>/download -o downloaded_test.txt

# 6. Verify checksum
sha256sum test.txt downloaded_test.txt
# Both should match
```

---

### Test 2 — Node Failure & Reconstruction

**Purpose**: Stop a node, download a file, verify Reed-Solomon reconstruction works.

```bash
# 1. Upload a file first (use a larger file for more meaningful test)
curl -X POST http://localhost:8080/api/files/upload \
    -F "file=@test_5mb.bin" | jq .

# 2. Verify all nodes are online
curl http://localhost:8080/api/nodes | jq '.[].status'
# → All "ONLINE"

# 3. Stop Node 2
curl -X POST http://localhost:8080/api/nodes/node-2/stop | jq .
# → {"message": "Node node-2 is now OFFLINE", "status": "OFFLINE"}

# 4. Check cluster status
curl http://localhost:8080/api/stats | jq .
# → online_nodes: 3

# 5. Download the file (should trigger reconstruction)
curl -v http://localhost:8080/api/files/<file_id>/download -o reconstructed.bin 2>&1 | grep "X-"
# Headers should include:
# X-Reconstructed: true
# X-Missing-Shards: <number>
# X-Recovered-Shards: <number>

# 6. Verify checksum
sha256sum test_5mb.bin reconstructed.bin
# Both should match — RS reconstruction succeeded!

# 7. Bring Node 2 back
curl -X POST http://localhost:8080/api/nodes/node-2/start | jq .
```

---

### Test 3 — Multiple Node Failure (Should Fail)

**Purpose**: Stop 2 nodes, verify system correctly refuses download.

```bash
# 1. Stop Node 2 and Node 3
curl -X POST http://localhost:8080/api/nodes/node-2/stop
curl -X POST http://localhost:8080/api/nodes/node-3/stop

# 2. Check stats
curl http://localhost:8080/api/stats | jq .
# → online_nodes: 2

# 3. Attempt download (should fail with 503)
curl -v http://localhost:8080/api/files/<file_id>/download
# → HTTP 503 Service Unavailable
# → {"error": "Download not possible", "reason": "Only 2 nodes online, need at least 3", ...}

# 4. Restart nodes
curl -X POST http://localhost:8080/api/nodes/node-2/start
curl -X POST http://localhost:8080/api/nodes/node-3/start
```

---

### Test 4 — Each Node Failure

**Purpose**: Verify that stopping ANY single node still allows download.

```bash
# For each node (1-4):
for NODE in node-1 node-2 node-3 node-4; do
    echo "=== Testing failure of $NODE ==="

    # Stop the node
    curl -s -X POST http://localhost:8080/api/nodes/$NODE/stop

    # Download
    curl -s http://localhost:8080/api/files/<file_id>/download -o /tmp/test_$NODE.bin

    # Verify
    sha256sum /tmp/test_$NODE.bin
    echo "Expected: <original checksum>"

    # Restart
    curl -s -X POST http://localhost:8080/api/nodes/$NODE/start

    echo ""
done
```

---

### Test 5 — File Delete

**Purpose**: Verify file deletion removes shards from all nodes.

```bash
# 1. Upload a file
FILE_ID=$(curl -s -X POST http://localhost:8080/api/files/upload \
    -F "file=@test.txt" | jq -r '.file_id')

# 2. Verify it exists
curl -s http://localhost:8080/api/files/$FILE_ID | jq .

# 3. Delete it
curl -s -X DELETE http://localhost:8080/api/files/$FILE_ID | jq .
# → {"message": "File deleted successfully"}

# 4. Verify it's gone
curl -s http://localhost:8080/api/files/$FILE_ID
# → {"error": "file not found: ..."}
```

---

### Test 6 — Large File Test

**Purpose**: Test with a file larger than the chunk size (4MB).

```bash
# Create a 20MB file
dd if=/dev/urandom of=large_test.bin bs=1M count=20

# Upload
curl -X POST http://localhost:8080/api/files/upload \
    -F "file=@large_test.bin" | jq .
# → Should show chunks: 5 (20MB / 4MB), shards: 20 (5 chunks × 4 shards)

# Download
curl http://localhost:8080/api/files/<file_id>/download -o large_downloaded.bin

# Verify
sha256sum large_test.bin large_downloaded.bin
# Must match
```

---

### Test 7 — Dashboard UI Test

**Purpose**: Verify the React dashboard works correctly through manual testing.

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Open http://localhost:5173 | Dashboard loads, shows 4 nodes ONLINE |
| 2 | Drag & drop a file onto upload zone | Upload progress shows, then success toast |
| 3 | File appears in file list | Shows filename, size, chunks, HEALTHY status |
| 4 | Click "⬇ Download" | File downloads, success toast |
| 5 | Click "⏹ Stop Node" on Node 2 | Node 2 card turns red/OFFLINE, header shows DEGRADED |
| 6 | Click "⬇ Download" again | File downloads with reconstruction toast: "X shard(s) reconstructed via Reed-Solomon" |
| 7 | Click "⏹ Stop Node" on Node 3 | Node 3 also goes OFFLINE, header shows CRITICAL |
| 8 | Click "⬇ Download" | Error toast: "Download impossible: Only 2 nodes online, need at least 3" |
| 9 | Click "▶ Start Node" on Node 2 & 3 | Both come back ONLINE, header shows HEALTHY |
| 10 | Click "🗑" delete button | Confirm dialog → file removed from list |

---

## Automated Test Script

### `test/integration_test.sh`

```bash
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
HEADERS=$(curl -s -D - $API/files/$FILE_ID/download -o /tmp/test_reconstructed.txt 2>&1)
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
rm -f /tmp/test_upload.txt /tmp/test_download.txt /tmp/test_reconstructed.txt

exit $FAIL
```

---

## Verification

```bash
# Start the system
docker-compose up -d

# Wait for all services to be ready (~10 seconds)
sleep 10

# Run integration tests
chmod +x test/integration_test.sh
./test/integration_test.sh

# Expected output:
# ✓ Health check OK
# ✓ 4 nodes found
# ✓ File uploaded: <uuid>
# ✓ Download checksum matches
# ✓ Node-2 is OFFLINE
# ✓ Reconstructed checksum matches
# ✓ Download correctly refused (503)
# ✓ All nodes back online
# ✓ File deleted
# ✓ File not found (correctly deleted)
#
# Results: 10 passed, 0 failed
```

---

## What This Step Achieves
- Comprehensive test coverage for all critical flows
- Automated test script for CI/CD
- Manual test procedures for dashboard verification
- Verification of Reed-Solomon reconstruction under node failure
- Validation of the failsafe mechanism (checksum verification + download refusal)
