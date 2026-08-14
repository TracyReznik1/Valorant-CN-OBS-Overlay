@echo off
chcp 65001 >nul
cd /d "%~dp0"
if not exist "ValorantCNOverlay.exe" (
  echo [错误] 找不到 ValorantCNOverlay.exe。
  echo 请确认压缩包已经完整解压，不要直接在压缩包内运行。
  pause
  exit /b 2
)
"ValorantCNOverlay.exe"
set "CODE=%ERRORLEVEL%"
if not "%CODE%"=="0" (
  echo.
  echo 程序退出，错误码：%CODE%
  pause
)
exit /b %CODE%
