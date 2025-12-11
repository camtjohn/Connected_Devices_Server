# Build Instructions

## Production Build
```powershell
go build -o server_app.exe
```
Uses production topics:
- `dev_bootup`
- `dev_heartbeat`
- `device_offline`
- `weather{zipcode}`

## Debug Build
```powershell
go build -tags debug -o server_app_debug.exe
```
Uses debug topics (prefixed with `debug_`):
- `debug_dev_bootup`
- `debug_dev_heartbeat`
- `debug_device_offline`
- `debug_weather{zipcode}`

## Running
Production: `.\server_app.exe`
Debug: `.\server_app_debug.exe`

The debug build won't interfere with production MQTT messages since it uses different topic names.

## Device Configuration
Make sure your test devices also publish to debug topics when testing:
- Bootup messages → `debug_dev_bootup`
- Heartbeat → `debug_dev_heartbeat`
- LWT → `debug_device_offline`
