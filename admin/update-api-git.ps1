# Update API code using Git
# Usage: .\update-api-git.ps1

Write-Host "=== API Code Update Script ===" -ForegroundColor Cyan
Write-Host ""

# 1. Check git status
Write-Host "[1/6] Checking git status..." -ForegroundColor Yellow
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
Write-Host "[2/6] Creating backup commit..." -ForegroundColor Yellow
git add .
git commit -m "BACKUP: Before API code update - $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
Write-Host "OK: Backup commit created" -ForegroundColor Green

# 3. Generate code
Write-Host "[3/6] Generating API code..." -ForegroundColor Yellow
goctl api go -api .\admin.api -dir .\internal --style=goZero

if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to generate code!" -ForegroundColor Red
    git reset --soft HEAD~1
    exit 1
}
Write-Host "OK: Code generated successfully" -ForegroundColor Green

# 4. Check changes
Write-Host "[4/6] Checking changes..." -ForegroundColor Yellow
$changedFiles = git diff --name-only
Write-Host "Changed files:" -ForegroundColor Cyan
$changedFiles | ForEach-Object { Write-Host "  - $_" -ForegroundColor Gray }

# 5. Restore logic files
Write-Host "[5/6] Restoring logic files..." -ForegroundColor Yellow
$logicFiles = git diff --name-only | Where-Object { $_ -like "*/logic/*" }
if ($logicFiles) {
    Write-Host "Restoring logic files:" -ForegroundColor Cyan
    $logicFiles | ForEach-Object { 
        Write-Host "  - $_" -ForegroundColor Gray
        git checkout HEAD~1 -- $_
    }
    Write-Host "OK: Logic files restored" -ForegroundColor Green
} else {
    Write-Host "No logic files to restore" -ForegroundColor Yellow
}

# 6. Build verification
Write-Host "[6/6] Building verification..." -ForegroundColor Yellow
go build -o admin.exe

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "=== Success ===" -ForegroundColor Green
    Write-Host "API code updated successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Cyan
    Write-Host "1. Review the changes: git diff" -ForegroundColor White
    Write-Host "2. If everything is OK, commit: git add . && git commit -m 'Update API code'" -ForegroundColor White
    Write-Host "3. If there are issues, rollback: git reset --hard HEAD~1" -ForegroundColor White
} else {
    Write-Host ""
    Write-Host "=== Error ===" -ForegroundColor Red
    Write-Host "Build failed! Please check the errors." -ForegroundColor Red
    Write-Host ""
    Write-Host "To rollback: git reset --hard HEAD~1" -ForegroundColor Yellow
    exit 1
}
