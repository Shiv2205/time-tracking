$ErrorActionPreference = 'Stop'
$repoPath = Split-Path -Parent $PSScriptRoot
Push-Location $repoPath
try {
    $goCommand = Get-Command go -ErrorAction SilentlyContinue
    $goExecutable = if ($goCommand) { $goCommand.Source } else { Join-Path $repoPath '.tools/go/bin/go.exe' }
    if (-not (Test-Path -LiteralPath $goExecutable)) { throw 'Install Go 1.26+ before building.' }
    if (-not $goCommand) {
        $env:GOPATH = Join-Path $repoPath '.cache/gopath'
        $env:GOCACHE = Join-Path $repoPath '.cache/go-build'
    }
    npm run build --prefix web
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
    New-Item -ItemType Directory -Force dist | Out-Null
    & $goExecutable build -o dist/timetracker.exe ./cmd/timetracker
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed.' }
} finally {
    Pop-Location
}
