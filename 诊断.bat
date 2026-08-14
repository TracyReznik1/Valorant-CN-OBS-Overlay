@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo === Valorant CN OBS Overlay 诊断 ===
if not exist "ValorantCNOverlay.exe" (
  echo [失败] ValorantCNOverlay.exe 不存在。
  pause
  exit /b 2
)
"ValorantCNOverlay.exe" --version
if errorlevel 1 (
  echo [失败] EXE 无法启动。请确认系统为 Windows 10/11 64 位，并检查杀毒软件拦截记录。
) else (
  echo [通过] EXE 可以启动。
  echo 如运行 Overlay 仍失败，请双击“启动段位Overlay.bat”并截图完整错误信息。
)
pause
