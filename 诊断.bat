@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo === Valorant CN OBS Overlay 诊断 ===
if not exist "ValorantCNOverlay.exe" (
  echo [失败] ValorantCNOverlay.exe 不存在。
  pause
  exit /b 2
)
"ValorantCNOverlay.exe" --diagnose
if errorlevel 1 (
  echo.
  echo [失败] 诊断发现异常，请截图上方完整信息。
) else (
  echo.
  echo [通过] 基础诊断全部通过。
  echo 如运行 Overlay 仍失败，请双击“启动段位Overlay.bat”并截图完整错误信息。
)
pause
