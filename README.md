# Earth Wallpaper

Live desktop wallpaper that fetches Himawari-8 satellite imagery, assembles tiles, and sets the system wallpaper.

## Key features

- Periodically fetches latest image metadata from the Himawari server.
- Assembles tiled images into a full wallpaper, scales the image to fit and pads with a black border.
- System tray control to start/stop fetching and to adjust border thickness at runtime.
- Packaging scripts for Windows and Unix (no extra GUI frameworks are installed by the packager).

## Requirements

- Go 1.20+ (module-aware workflow)
- Internet access to fetch Himawari images

## Build & Run (development)

1. Download modules and build:

    ```bash
    go mod download
    # Unix
    go build -o earth-wallpaper
    # Windows
    go build -ldflags -H=windowsgui -o earth-wallpaper.exe
    ```

2. Run locally (shows tray icon):

    ```bash
    ./earth-wallpaper
    ```

On Windows run the produced `earth-wallpaper.exe`.

## System tray usage

- Right-click the tray icon to open the menu.
- `Latest Image: Running / Stopped`: toggle automatic fetching.
- `Date: ...`: displays last fetched image timestamp.

## Configuration & files

- The assembled wallpaper is saved to the system temp folder as `earth_wallpaper_full.png` before being applied.

## Troubleshooting

- If build fails, ensure your `GOPATH` and `GOROOT` are correct and `go env` shows a usable toolchain.
- If packaging fails on Windows, run the PowerShell script from an elevated prompt only if required for filesystem permissions.

## License & Notes

This project bundles public Himawari tiles fetched from the provider. Respect the data provider's terms of use.
