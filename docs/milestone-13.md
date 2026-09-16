# Milestone 13: double-click launch

Start Afterglow.cmd builds the Go app, starts a hidden local process, waits for readiness and opens the browser. Repeat launches reuse a healthy existing demo. A per-port mutex prevents simultaneous launch races. Occupied ports fail visibly without stopping other applications. The local launcher explicitly selects SQLite, local transport and demo mode; deployment database credentials are not used.

Stop Afterglow.cmd only stops a listener whose executable is an afterglow executable inside this checkout's bin directory. No data is deleted. Desktop Start/Stop shortcuts were created on this machine; the repository contains the portable command files.

Validated in Windows PowerShell 5.1: existing-instance reuse, fresh build/start on an isolated port and database, repeat launch, browser rendering with no JS errors, stop, repeated stop, occupied-port refusal, and refusal to stop an unrelated listener. The regular demo database was preserved. Go 1.27+ is needed for a fresh start. Errors stay visible in the double-click window.
