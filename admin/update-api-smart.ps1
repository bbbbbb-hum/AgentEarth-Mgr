# Smart API Code Update Script
# Usage: .\update-api-smart.ps1

Write-Host "=== Smart API Code Update ===" -ForegroundColor Cyan
Write-Host ""

# 1. Check git status
Write-Host "[1/7] Checking git status..." -ForegroundColor Yellow
$gitStatus = git status --porcelain
if ($gitStatus) {
    Write-Host "Warning: You have uncommitted changes!" -ForegroundColor Red
    Write-Host "Please commit or stash your changes first."
    Write-Host ""
    Write-Host "Uncommitted changes:" -ForegroundColor Yellow
    git status --short
    exit 1
}
Write-Host "OK: Working directory is clean" -ForegroundColor Green

# 2. Create backup commit
Write-Host "[2/7] Creating backup commit..." -ForegroundColor Yellow
git add .
git commit -m "BACKUP: Before API update - $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
$backupCommit = git rev-parse HEAD
Write-Host "OK: Backup commit created ($backupCommit)" -ForegroundColor Green

# 3. Delete files to be regenerated
Write-Host "[3/7] Preparing for code generation..." -ForegroundColor Yellow
Write-Host "  Deleting types and handler files..." -ForegroundColor Gray

# Delete types file (we want to regenerate it)
if (Test-Path "internal/types/types.go") {
    Remove-Item "internal/types/types.go" -Force
    Write-Host "  - Deleted internal/types/types.go" -ForegroundColor Gray
}

# Delete handler files (they are auto-generated)
$handlerFiles = Get-ChildItem -Path "internal/handler" -Filter "*.go" -Recurse
foreach ($file in $handlerFiles) {
    Remove-Item $file.FullName -Force
}
Write-Host "  - Deleted handler files" -ForegroundColor Gray

# 4. Generate code
Write-Host "[4/7] Generating API code..." -ForegroundColor Yellow
goctl api go -api .\admin.api -dir . --style=goZero

if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to generate code!" -ForegroundColor Red
    git reset --hard HEAD~1
    exit 1
}
Write-Host "OK: Code generated successfully" -ForegroundColor Green

# 5. Check logic file changes
Write-Host "[5/7] Analyzing logic file changes..." -ForegroundColor Yellow
$logicChanges = git diff --name-only HEAD~1 HEAD | Where-Object { $_ -like "*/logic/*" }

if ($logicChanges) {
    Write-Host "Logic files changed:" -ForegroundColor Cyan
    $logicChanges | ForEach-Object { Write-Host "  - $_" -ForegroundColor Gray }
    
    # Check if method signatures changed
    Write-Host ""
    Write-Host "Checking method signatures..." -ForegroundColor Yellow
    
    $signatureChanged = $false
    foreach ($file in $logicChanges) {
        $oldContent = git show HEAD~1:$file
        $newContent = Get-Content $file -Raw
        
        # Simple check: if file is significantly different, signature might have changed
        if ($oldContent -and $newContent) {
            $oldLines = $oldContent.Split("`n").Count
            $newLines = $newContent.Split("`n").Count
            
            # If line count difference is large, signature likely changed
            if ([Math]::Abs($oldLines - $newLines) -gt 10) {
                Write-Host "  Warning: $file - Signature may have changed!" -ForegroundColor Red
                $signatureChanged = $true
            }
        }
    }
    
    if ($signatureChanged) {
        Write-Host ""
        Write-Host "⚠️  Method signatures may have changed!" -ForegroundColor Red
        Write-Host "Please review the following files manually:" -ForegroundColor Yellow
        $logicChanges | ForEach-Object { Write-Host "  - $_" -ForegroundColor Gray }
        Write-Host ""
        Write-Host "Options:" -ForegroundColor Cyan
        Write-Host "1. Review changes: git diff HEAD~1 HEAD -- internal/logic/" -ForegroundColor White
        Write-Host "2. If signatures changed, manually update logic files" -ForegroundColor White
        Write-Host "3. If signatures unchanged, restore: git checkout HEAD~1 -- internal/logic/" -ForegroundColor White
        Write-Host ""
        $response = Read-Host "Do you want to restore logic files? (y/n)"
        if ($response -eq 'y') {
            Write-Host "Restoring logic files..." -ForegroundColor Yellow
            git checkout HEAD~1 -- internal/logic/
            Write-Host "OK: Logic files restored" -ForegroundColor Green
        } else {
            Write-Host "Logic files NOT restored. Please update manually." -ForegroundColor Yellow
        }
    } else {
        Write-Host ""
        Write-Host "Method signatures likely unchanged." -ForegroundColor Green
        Write-Host "Restoring logic files..." -ForegroundColor Yellow
        git checkout HEAD~1 -- internal/logic/
        Write-Host "OK: Logic files restored" -ForegroundColor Green
    }
} else {
    Write-Host "No logic files changed" -ForegroundColor Yellow
}

# 5. Build verification
Write-Host "[5/7] Building verification..." -ForegroundColor Yellow
go build -o admin.exe

if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    Write-Host "=== Build Failed ===" -ForegroundColor Red
    Write-Host "Build errors detected. This might be due to:" -ForegroundColor Yellow
    Write-Host "1. Method signature changes" -ForegroundColor White
    Write-Host "2. Missing imports" -ForegroundColor White
    Write-Host "3. Type mismatches" -ForegroundColor White
    Write-Host ""
    Write-Host "To rollback: git reset --hard $backupCommit" -ForegroundColor Yellow
    Write-Host "To review changes: git diff HEAD~1 HEAD" -ForegroundColor Yellow
    exit 1
}

Write-Host "OK: Build successful" -ForegroundColor Green

# 6. Show summary
Write-Host ""
Write-Host "[6/7] Summary:" -ForegroundColor Yellow
$changedFiles = git diff --name-only HEAD~1 HEAD
Write-Host "Changed files: $($changedFiles.Count)" -ForegroundColor Cyan

$typesChanged = $changedFiles | Where-Object { $_ -like "*/types/*" }
$handlersChanged = $changedFiles | Where-Object { $_ -like "*/handler/*" }
$logicChanged = $changedFiles | Where-Object { $_ -like "*/logic/*" }

Write-Host "  Types: $($typesChanged.Count)" -ForegroundColor Gray
Write-Host "  Handlers: $($handlersChanged.Count)" -ForegroundColor Gray
Write-Host "  Logic: $($logicChanged.Count)" -ForegroundColor Gray

# 7. Final instructions
Write-Host ""
Write-Host "=== Success ===" -ForegroundColor Green
Write-Host "API code updated successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "1. Review changes: git diff HEAD~1 HEAD" -ForegroundColor White
Write-Host "2. If OK, commit: git add . && git commit -m 'Update API code'" -ForegroundColor White
Write-Host "3. If issues, rollback: git reset --hard $backupCommit" -ForegroundColor White
Write-Host ""
Write-Host "Backup commit: $backupCommit" -ForegroundColor Gray
