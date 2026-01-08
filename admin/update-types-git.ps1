# Update types file only using Git
# Usage: .\update-types-git.ps1

Write-Host "=== Types File Update Script ===" -ForegroundColor Cyan
Write-Host ""

# 1. Check git status
Write-Host "[1/4] Checking git status..." -ForegroundColor Yellow
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
Write-Host "[2/4] Creating backup commit..." -ForegroundColor Yellow
git add .
git commit -m "BACKUP: Before types update - $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
Write-Host "OK: Backup commit created" -ForegroundColor Green

# 3. Generate code
Write-Host "[3/4] Generating types..." -ForegroundColor Yellow
goctl api go -api .\admin.api -dir .\internal --style=goZero

if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to generate code!" -ForegroundColor Red
    git reset --soft HEAD~1
    exit 1
}
Write-Host "OK: Types generated successfully" -ForegroundColor Green

# 4. Restore logic files
Write-Host "[4/4] Restoring logic files..." -ForegroundColor Yellow
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

# Build verification
Write-Host ""
Write-Host "Building verification..." -ForegroundColor Yellow
go build -o admin.exe

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "=== Success ===" -ForegroundColor Green
    Write-Host "Types file updated successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Cyan
    Write-Host "1. Review changes: git diff" -ForegroundColor White
    Write-Host "2. If OK, commit: git add . && git commit -m 'Update types'" -ForegroundColor White
    Write-Host "3. If issues, rollback: git reset --hard HEAD~1" -ForegroundColor White
} else {
    Write-Host ""
    Write-Host "=== Error ===" -ForegroundColor Red
    Write-Host "Build failed! Please check errors." -ForegroundColor Red
    Write-Host ""
    Write-Host "To rollback: git reset --hard HEAD~1" -ForegroundColor Yellow
    exit 1
}
