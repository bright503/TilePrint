; MyGo includes this after extracting TilePrint.exe. No administrator rights
; are requested: the Microsoft installer supports installation per user.
!include "LogicLib.nsh"

!macro customInstall
  DetailPrint "正在检查 Microsoft WebView2 运行库…"
  ExecWait '"$INSTDIR\TilePrint.exe" --check-webview2' $0
  ${If} $0 != 0
    InitPluginsDir
    !if "$%TILEPRINT_WEBVIEW2_OFFLINE%" != ""
      File /oname=$PLUGINSDIR\WebView2RuntimeInstaller.exe "$%TILEPRINT_WEBVIEW2_OFFLINE%"
    !endif
    tileprint_runtime_retry:
    DetailPrint "正在安装 WebView2，请稍候（联网安装可能需要几分钟）…"
    !if "$%TILEPRINT_WEBVIEW2_OFFLINE%" != ""
      ExecWait '"$INSTDIR\TilePrint.exe" --install-webview2 "$PLUGINSDIR\WebView2RuntimeInstaller.exe"' $0
    !else
      ExecWait '"$INSTDIR\TilePrint.exe" --install-webview2' $0
    !endif
    ${If} $0 != 0
      MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "WebView2 安装失败。请检查网络或系统策略后重试，也可以取消后使用 TilePrint 离线安装包。详细日志位于本地应用数据目录的 TilePrint\logs\startup.log。" /SD IDCANCEL IDRETRY tileprint_runtime_retry
      SetErrorLevel 1
      Abort "WebView2 安装未完成，TilePrint 安装未成功。"
    ${EndIf}
    DetailPrint "WebView2 已就绪。"
  ${Else}
    DetailPrint "已检测到 WebView2，无需重复安装。"
  ${EndIf}
!macroend
