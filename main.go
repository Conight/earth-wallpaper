package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"

	"earth-wallpaper/icon"
	"earth-wallpaper/wallpaper"
	"earth-wallpaper/wallpaper/modes"

	"fyne.io/systray"
)

// Global variable to store the latest image date and parsed time
var (
	latestImageDate string
	latestImageTime time.Time
	latestImageMu   sync.RWMutex
)

const resolution int = 4
const tileSize = 550
const borderThickness = 300

func downloadImage(resolution, i, j int, t time.Time) image.Image {
	var year, month, day, hour, minute, second string

	if t.IsZero() {
		// fallback
		log.Printf("downloadImage: received zero time, using fallback date")
		year, month, day = "2026", "01", "10"
		hour, minute, second = "02", "00", "00"
	} else {
		year = fmt.Sprintf("%04d", t.Year())
		month = fmt.Sprintf("%02d", t.Month())
		day = fmt.Sprintf("%02d", t.Day())
		hour = fmt.Sprintf("%02d", t.Hour())
		minute = fmt.Sprintf("%02d", t.Minute())
		second = fmt.Sprintf("%02d", t.Second())
	}

	timeStr := fmt.Sprintf("%s%s%s", hour, minute, second)
	url := fmt.Sprintf("https://anzu.shinshu-u.ac.jp/himawari/img/D531106/%dd/550/%s/%s/%s/%s_%d_%d.png", resolution, year, month, day, timeStr, i, j)
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if err != nil {
		panic(err)
	}

	img, err := png.Decode(&buf)
	if err != nil {
		panic(err)
	}
	return img
}

func setWallpaper(fullImagePath string) {
	// set wallpaper
	err := wallpaper.SetWallpaper(fullImagePath)
	if err != nil {
		log.Printf("setWallpaper error: %v", err)
		return
	}
	wallpaper.SetMode(modes.FILL_ZOOM)
}

// startFetcher runs a loop to fetch latest image info immediately and then every 10s.
func startFetcher(stopCh chan bool, mLatestImageDate *systray.MenuItem) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	doFetch := func() {
		raw, err := latestImage()
		if err != nil {
			mLatestImageDate.SetTitle("Date: Error fetching")
			log.Printf("startFetcher: latestImage error: %v", err)
			return
		}

		// Try to parse returned date
		parsed, perr := time.Parse("2006-01-02 15:04:05", raw)
		if perr != nil {
			// Keep raw string for display
			latestImageMu.Lock()
			latestImageDate = raw
			latestImageMu.Unlock()
			mLatestImageDate.SetTitle(fmt.Sprintf("Date: %s", raw))
			log.Printf("startFetcher: failed to parse date '%s': %v", raw, perr)
			return
		}

		// Update stored time and process wallpaper only when newer
		latestImageMu.Lock()
		prev := latestImageTime
		if prev.IsZero() || parsed.After(prev) {
			latestImageTime = parsed
			latestImageDate = raw
			latestImageMu.Unlock()
			mLatestImageDate.SetTitle(fmt.Sprintf("Date: %s", raw))
			// New image: compose wallpaper and set it
			fullPath := processWallpaper(parsed)
			if fullPath != "" {
				setWallpaper(fullPath)
			}
			return
		}
		// not newer
		latestImageMu.Unlock()
		mLatestImageDate.SetTitle(fmt.Sprintf("Date: %s", raw))
	}

	// immediate first fetch
	doFetch()

	for {
		select {
		case <-ticker.C:
			doFetch()
		case <-stopCh:
			return
		}
	}
}

func processWallpaper(t time.Time) string {
	gridSize := resolution
	canvas := image.NewRGBA(image.Rect(0, 0, gridSize*tileSize, gridSize*tileSize))
	for i := range gridSize {
		for j := range gridSize {
			img := downloadImage(resolution, i, j, t)
			draw.Draw(canvas, image.Rect(i*tileSize, j*tileSize, (i+1)*tileSize, (j+1)*tileSize), img, image.Point{0, 0}, draw.Src)
		}
	}

	// compose final image: black background with the assembled image scaled to fit inside
	border := borderThickness
	w := gridSize * tileSize
	h := gridSize * tileSize

	final := image.NewRGBA(image.Rect(0, 0, w, h))
	// fill black background
	draw.Draw(final, final.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)

	// compute destination rect (inset by border)
	dstRect := image.Rect(border, border, w-border+50, h-border-50)
	if dstRect.Dx() <= 0 || dstRect.Dy() <= 0 {
		// border too large, fallback to no border
		dstRect = final.Bounds()
	}

	// scale assembled canvas into dstRect using high-quality scaler
	xdraw.CatmullRom.Scale(final, dstRect, canvas, canvas.Bounds(), draw.Over, nil)

	// save full image to system temp folder
	tempDir := os.TempDir()
	fullImagePath := filepath.Join(tempDir, "earth_wallpaper_full.png")
	outFile, err := os.Create(fullImagePath)
	if err != nil {
		log.Printf("processWallpaper: failed to create file: %v", err)
		return ""
	}
	defer outFile.Close()
	if err := png.Encode(outFile, final); err != nil {
		log.Printf("processWallpaper: failed to encode png: %v", err)
	}
	fmt.Println("Wallpaper save to:", fullImagePath)
	return fullImagePath
}

func addQuitItem() {
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	go func() {
		for range mQuit.ClickedCh {
			fmt.Println("Requesting quit")
			systray.Quit()
		}
	}()
}

func onReady() {
	systray.SetTemplateIcon(icon.Data, icon.Data)
	systray.SetTitle("Awesome App")
	systray.SetTooltip("Lantern")
	addQuitItem()
	systray.AddSeparator()

	// We can manipulate the systray in other goroutines
	go func() {
		systray.SetTemplateIcon(icon.Data, icon.Data)
		systray.SetTitle("Earth Wallpaper")
		systray.SetTooltip("Live wallpaper from Himawari 8 satellite")
		// Latest Image Section
		mLatestImageStatus := systray.AddMenuItem("Latest Image: Running", "Click to toggle fetching")
		mLatestImageDate := systray.AddMenuItem("Date: --", "Latest image date")

		var stopCh chan bool
		isRunning := true

		// Initialize latest image fetching
		stopCh = make(chan bool)
		go startFetcher(stopCh, mLatestImageDate)

		// Toggle latest image fetching handler
		go func() {
			for range mLatestImageStatus.ClickedCh {
				if isRunning {
					isRunning = false
					stopCh <- true
					mLatestImageStatus.SetTitle("Latest Image: Stopped")
					mLatestImageDate.SetTitle("Date: --")
				} else {
					isRunning = true
					stopCh = make(chan bool)
					mLatestImageStatus.SetTitle("Latest Image: Running")

					go startFetcher(stopCh, mLatestImageDate)
				}
			}
		}()
	}()
}

// LatestImageInfo represents the latest image information from the Himawari 8 satellite
//
// Example JSON response:
//
//	{
//	  "date": "2026-01-11 16:10:00",
//	  "file": "PI_H09_20260111_1610_TRC_FLDK_R10_PGPFD.png"
//	}
type LatestImageInfo struct {
	Date string `json:"date"`
	File string `json:"file"`
}

// latestImage fetches the latest image information from the Himawari 8 satellite
func latestImage() (string, error) {
	resp, err := http.Get("https://jh170034-1.kudpc.kyoto-u.ac.jp/himawari/img/D531106/latest.json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data = LatestImageInfo{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return "", err
	}
	fmt.Println("Latest image date:", data.Date)
	return data.Date, nil
}

func main() {
	onExit := func() {
		now := time.Now()
		fmt.Println("Exit at", now.String())
	}

	systray.Run(onReady, onExit)
	fmt.Println("Finished quitting")
}
