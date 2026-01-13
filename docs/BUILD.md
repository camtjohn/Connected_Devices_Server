# Build Instructions

## Production Build (Linux)
```powershell
$env:GOOS = "linux"; $env:GOARCH = "amd64"; go build -o server_app
```

Uses production topics:
- `dev_bootup`
- `dev_heartbeat`
- `device_offline`
- `weather/<zipcode>`

## Debug Build (Linux)
```powershell
$env:GOOS = "linux"; $env:GOARCH = "amd64"; go build -tags debug -o server_app_debug
```

Uses debug topics (prefixed with `debug_`):
- `debug_dev_bootup`
- `debug_dev_heartbeat`
- `debug_device_offline`
- `debug_weather/<zipcode>`

## Running on Oracle VM
Copy the binary to your Oracle VM Linux instance and execute:
```bash
./server_app          # Production
./server_app_debug    # Debug
```

The debug build won't interfere with production MQTT messages since it uses different topic names.

## Device Configuration
Make sure your test devices also publish to debug topics when testing:
- Bootup messages → `debug_dev_bootup`
- Heartbeat → `debug_dev_heartbeat`
- LWT → `debug_device_offline`
