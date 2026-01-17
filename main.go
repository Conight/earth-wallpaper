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
const border = 180 // pixels of black padding on each side; adjust as needed

// tileResult holds the result of a downloaded tile image
type tileResult struct {
	x, y int
	img  image.Image
}

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
		log.Printf("downloadImage: http get error for %s: %v", url, err)
		// return a blank placeholder tile so the final image stays complete
		return image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("downloadImage: non-200 status %d for %s", resp.StatusCode, url)
		return image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if err != nil {
		log.Printf("downloadImage: read body error for %s: %v", url, err)
		return image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	}

	img, err := png.Decode(&buf)
	if err != nil {
		log.Printf("downloadImage: png decode error for %s: %v", url, err)
		return image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	}
	return img
}

func setWallpaper(fullImagePath string) {
	// set wallpaper mode first, then apply the wallpaper
	wallpaper.SetMode(modes.FILL_ORIGINAL)
	err := wallpaper.SetWallpaper(fullImagePath)
	if err != nil {
		log.Printf("setWallpaper error: %v", err)
		return
	}
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
	// create a new blank canvas
	canvas := image.NewRGBA(image.Rect(0, 0, gridSize*tileSize, gridSize*tileSize))

	log.Printf("Start parallel download image")
	start_time := time.Now()

	results := make(chan tileResult, gridSize*gridSize)
	var wg sync.WaitGroup

	for i := 0; i < gridSize; i++ {
		for j := 0; j < gridSize; j++ {
			wg.Add(1)
			go func(x, y int) {
				defer wg.Done()
				img := downloadImage(resolution, x, y, t)
				results <- tileResult{x: x, y: y, img: img}
			}(i, j)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	count := 0
	for res := range results {
		dest := image.Rect(res.x*tileSize, res.y*tileSize, (res.x+1)*tileSize, (res.y+1)*tileSize)
		draw.Draw(canvas, dest, res.img, image.Point{0, 0}, draw.Src)
		count++
	}

	log.Printf("End download image, processed %d tiles, took %d ms", count, time.Since(start_time).Milliseconds())

	// add a uniform black border to avoid distortion when displayed
	srcW := canvas.Bounds().Dx()
	srcH := canvas.Bounds().Dy()
	dstW := srcW + border*2
	dstH := srcH + border*2
	bordered := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	// fill with black
	draw.Draw(bordered, bordered.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	// draw original canvas centered with the border offset
	draw.Draw(bordered, image.Rect(border, border, border+srcW, border+srcH), canvas, image.Point{0, 0}, draw.Src)

	// save bordered image to system temp folder
	tempDir := os.TempDir()
	fullImagePath := filepath.Join(tempDir, "earth_wallpaper_full.png")
	outFile, err := os.Create(fullImagePath)
	if err != nil {
		log.Printf("processWallpaper: failed to create file: %v", err)
		return ""
	}
	defer outFile.Close()

	enc := png.Encoder{CompressionLevel: png.NoCompression}
	if err := enc.Encode(outFile, bordered); err != nil {
		log.Printf("processWallpaper: failed to encode png: %v", err)
	}

	log.Printf("Wallpaper save to: %s", fullImagePath)
	return fullImagePath
}

func addQuitItem() {
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	go func() {
		for range mQuit.ClickedCh {
			log.Printf("Requesting quit")
			systray.Quit()
		}
	}()
}

func onReady() {
	systray.SetTemplateIcon(icon.Data, icon.Data)
	systray.SetTitle("Earth Wallpaper")
	systray.SetTooltip("Live wallpaper from Himawari 8 satellite")
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
	log.Printf("Latest image date: %s", data.Date)
	return data.Date, nil
}

func main() {
	onExit := func() {
		now := time.Now()
		log.Printf("Exit at %s", now.String())
	}

	systray.Run(onReady, onExit)
	log.Printf("Finished quitting")
}
