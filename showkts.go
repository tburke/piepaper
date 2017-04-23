package main

import (
	"encoding/csv"
	"errors"
	"github.com/llgcode/draw2d"
	"github.com/llgcode/draw2d/draw2dimg"
	"github.com/llgcode/draw2d/draw2dkit"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"./netpbm"
	"golang.org/x/image/draw"
	"image"
	"image/color"
)

type Point [2]float64

func DegmintoFloat(val string, dir string) float64 {
	p := strings.Index(val, ".")
	if p < 2 {
		return math.NaN()
	}
	deg, err := strconv.ParseFloat(val[0:p-2], 64)
	if err != nil {
		return math.NaN()
	}
	min, err := strconv.ParseFloat(val[p-2:], 64)
	if err != nil {
		return math.NaN()
	}
	deg += min / 60
	if dir[0] == 'S' || dir[0] == 'W' {
		deg = -deg
	}
	return deg
}

func (p *Point) FromGPS(lat, latdir, lon, londir string) error {
	p[0] = DegmintoFloat(lat, latdir)
	p[1] = DegmintoFloat(lon, londir)
	return nil
}

func TimeFromGPS(gtime, date string) time.Time {
	var yr, dy int
	var mo time.Month
	if len(date) > 5 {
		dy, _ = strconv.Atoi(date[0:2])
		month, _ := strconv.Atoi(date[2:4])
		mo = time.Month(month)
		yr, _ = strconv.Atoi(date[4:6])
		yr += 2000
	} else {
		t := time.Now()
		yr = t.Year()
		mo = t.Month()
		dy = t.Day()
	}
	hr, _ := strconv.Atoi(gtime[0:2])
	mn, _ := strconv.Atoi(gtime[2:4])
	sc, _ := strconv.Atoi(gtime[4:6])
	return time.Date(yr, mo, dy, hr, mn, sc, 0, time.UTC)
}

type GPRMC struct {
	Time      time.Time
	Validity  byte
	LatLong   Point
	Speed     float32
	Course    float32
	Variation float32
	EastWest  byte
}

func (g *GPRMC) FromNMEA(s []string) error {
	if s[0] != "$GPRMC" {
		return errors.New("Record is not a GPRMC record")
	}
	if len(s) != 13 {
		return errors.New("Record does not have 13 fields")
	}
	g.Time = TimeFromGPS(s[1], s[9])
	g.Validity = s[2][0]
	g.LatLong.FromGPS(s[3], s[4], s[5], s[6])
	f, _ := strconv.ParseFloat(s[7], 32)
	g.Speed = float32(f)
	f, _ = strconv.ParseFloat(s[8], 32)
	g.Course = float32(f)
	f, _ = strconv.ParseFloat(s[10], 32)
	g.Variation = float32(f)
	if len(s[11]) > 0 {
		g.EastWest = s[10][0]
	}
	return nil
}

func epd_write(i image.Image) {
	cmd, err := os.OpenFile("/dev/epd/command", os.O_RDWR, 0666)
	if err != nil {
		return
	}
	img, err := os.OpenFile("/dev/epd/LE/display", os.O_RDWR, 0666)
	if err != nil {
		return
	}
	cmd.Write([]byte("C"))
	netpbm.PbmData(img, i)
	cmd.Write([]byte("P"))
	img.Close()
	cmd.Close()
}

func show_data(name, value string) image.Image {
	draw2d.SetFontFolder("/usr/share/fonts/truetype/roboto/")
	i := image.NewRGBA(image.Rect(0, 0, 264, 176))
	draw.Src.Draw(i, i.Bounds(), image.White, image.ZP)
	gc := draw2dimg.NewGraphicContext(i)
	gc.Save()
	gc.SetStrokeColor(color.Black)
	gc.SetFillColor(color.Black)
	draw2dkit.Rectangle(gc, 10, 10, 50, 50)
	gc.FillStroke()

	gc.SetFontData(draw2d.FontData{Name: "Roboto", Family: draw2d.FontFamilyMono, Style: draw2d.FontStyleNormal})
	gc.SetFontSize(52)
	gc.FillStringAt(name, 60, 80)
	gc.FillStringAt(value, 60, 160)

	gc.Restore()
	return i
}

func show_speed() {
	f, _ := os.Open("/dev/ttyUSB0")
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if rec[0] == "$GPRMC" {
			var g GPRMC
			g.FromNMEA(rec)
			img := show_data("Speed", strconv.FormatFloat(float64(g.Speed), 'f', -1, 32))
			epd_write(img)
			break
		}
	}
	f.Close()
}

func main() {
	for {
		show_speed()
		time.Sleep(1 * time.Minute)
	}
}
