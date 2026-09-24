# Distributed File Storage Integration Test (PowerShell)
$API = "http://localhost:8080/api"
$PASS = 0
$FAIL = 0

function Green($msg) {
    Write-Host "[PASS] $msg" -ForegroundColor Green
    $script:PASS++
}

function Red($msg) {
    Write-Host "[FAIL] $msg" -ForegroundColor Red
    $script:FAIL++
}

Write-Host "===========================================" -ForegroundColor Cyan
Write-Host " Distributed File Storage - Test Suite    " -ForegroundColor Cyan
Write-Host "===========================================" -ForegroundColor Cyan

# 1. Health check
try {
    $res = Invoke-RestMethod -Uri "$API/health" -Method Get
    if ($res.status -eq "healthy") { Green "Health check OK" } else { Red "Health check failed" }
} catch {
    Red "Health check request error: $_"
}

# 2. List nodes
try {
    $nodes = Invoke-RestMethod -Uri "$API/nodes" -Method Get
    if ($nodes.Count -eq 4) { Green "4 storage nodes verified in cluster" } else { Red "Expected 4 nodes, got $($nodes.Count)" }
} catch {
    Red "Nodes check error: $_"
}

# 3. Create test file & upload via curl
$testFile = "$env:TEMP\distfs_test.txt"
[System.IO.File]::WriteAllText($testFile, "Reed-Solomon Erasure Coding Test Payload - " + (Get-Date))
$origHash = (Get-FileHash $testFile -Algorithm SHA256).Hash.ToLower()

try {
    $uploadJson = curl.exe -s -X POST "$API/files/upload" -F "file=@$testFile"
    $upload = $uploadJson | ConvertFrom-Json
    $fileID = $upload.file_id
    if ($fileID) {
        Green "Uploaded file ($fileID, $($upload.chunks) chunks, $($upload.shards) shards)"
    } else {
        Red "Upload response missing file_id: $uploadJson"
    }
} catch {
    Red "Upload failed: $_"
}

# 4. Download (All nodes online)
$dlFile = "$env:TEMP\distfs_dl.txt"
try {
    curl.exe -s "$API/files/$fileID/download" -o $dlFile
    $dlHash = (Get-FileHash $dlFile -Algorithm SHA256).Hash.ToLower()
    if ($dlHash -eq $origHash) { Green "Download with all nodes online: SHA-256 matches perfectly!" } else { Red "Checksum mismatch on direct download" }
} catch {
    Red "Download failed: $_"
}

# 5. Stop Node 2 (Simulate node failure)
try {
    $stopRes = Invoke-RestMethod -Uri "$API/nodes/node-2/stop" -Method Post
    if ($stopRes.status -eq "OFFLINE") { Green "Node-2 successfully set to OFFLINE" } else { Red "Failed to stop Node-2" }
} catch {
    Red "Stop Node-2 failed: $_"
}

# 6. Download with 1 node offline (Should reconstruct via Reed-Solomon)
$reconFile = "$env:TEMP\distfs_recon.txt"
try {
    curl.exe -s "$API/files/$fileID/download" -o $reconFile
    $reconHash = (Get-FileHash $reconFile -Algorithm SHA256).Hash.ToLower()
    if ($reconHash -eq $origHash) {
        Green "Reconstruction succeeded! 1 node offline, SHA-256 validated!"
    } else {
        Red "Reconstructed checksum mismatch: Expected $origHash, Got $reconHash"
    }
} catch {
    Red "Reconstruction download failed: $_"
}

# 7. Stop Node 3 (2 nodes offline -> should fail)
try {
    Invoke-RestMethod -Uri "$API/nodes/node-3/stop" -Method Post | Out-Null
    $failHeaders = curl.exe -s -i "$API/files/$fileID/download"
    if ($failHeaders -match "503 Service Unavailable") {
        Green "Download correctly refused (503 Service Unavailable) when 2 nodes offline"
    } else {
        Red "Expected 503 error, but received different response"
    }
} catch {
    Red "Stop Node-3 test failed: $_"
}

# 8. Restore nodes
try {
    Invoke-RestMethod -Uri "$API/nodes/node-2/start" -Method Post | Out-Null
    Invoke-RestMethod -Uri "$API/nodes/node-3/start" -Method Post | Out-Null
    $nodes = Invoke-RestMethod -Uri "$API/nodes" -Method Get
    $online = ($nodes | Where-Object { $_.status -eq "ONLINE" }).Count
    if ($online -eq 4) { Green "All nodes restored to ONLINE" } else { Red "Only $online nodes online" }
} catch {
    Red "Restore nodes failed: $_"
}

# 9. Delete file
try {
    $del = Invoke-RestMethod -Uri "$API/files/$fileID" -Method Delete
    Green "File deleted from cluster"
} catch {
    Red "File delete failed: $_"
}

# Cleanup
Remove-Item -Force -ErrorAction SilentlyContinue $testFile, $dlFile, $reconFile

Write-Host "===========================================" -ForegroundColor Cyan
Write-Host " Results: $PASS Passed, $FAIL Failed       " -ForegroundColor Cyan
Write-Host "===========================================" -ForegroundColor Cyan
