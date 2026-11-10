@echo off
rem Build ENStream for Windows (needs Go 1.25+ from https://go.dev/dl/).
setlocal
cd /d "%~dp0.."
if "%1"=="" (set VERSION=dev) else (set VERSION=%1)
if not exist dist mkdir dist
set CGO_ENABLED=0
go build -trimpath -ldflags "-s -w -X enstream/internal/app.Version=%VERSION%" -o dist\enstream-windows-amd64.exe .\cmd\enstream || exit /b 1
echo Built dist\enstream-windows-amd64.exe
