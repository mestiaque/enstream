; Let phones and TVs on the home network reach ENStream: allow the server through Windows
; Firewall on private (home/work) networks only, so Windows does not ask after install.
!macro NSIS_HOOK_POSTINSTALL
  nsExec::Exec 'netsh advfirewall firewall delete rule name="ENStream"'
  nsExec::Exec 'netsh advfirewall firewall add rule name="ENStream" dir=in action=allow program="$INSTDIR\enstream-server.exe" enable=yes profile=private,domain'
!macroend

!macro NSIS_HOOK_PREUNINSTALL
  ; Stop a running copy so its files can be removed.
  nsExec::Exec 'taskkill /IM enstream.exe /F'
  nsExec::Exec 'taskkill /IM enstream-server.exe /F'
  nsExec::Exec 'netsh advfirewall firewall delete rule name="ENStream"'
!macroend
