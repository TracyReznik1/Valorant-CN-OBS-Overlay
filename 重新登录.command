#!/bin/bash
cd "$(dirname "$0")" || exit 1
case "$(uname -m)" in
  arm64) bin="ValorantCNOverlay-macos-arm64" ;;
  *) bin="ValorantCNOverlay-macos-amd64" ;;
esac
if [ ! -x "$bin" ]; then
  echo "找不到 $bin，请确认已完整解压 macOS 包。"
  read -r -p "按回车退出…" _
  exit 2
fi
"./$bin" --reset-login "$@"
status=$?
echo
echo "程序已退出（错误码 $status）。按回车关闭窗口。"
read -r _
exit "$status"
