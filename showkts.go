package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/golang/freetype/truetype"
	"github.com/tburke/netpbm"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/math/fixed"

	"image"
)

type Point [2]float64

func DegmintoFloat(val string, dir string) float64 {
	p := strings.Index(val, ".")
	if p < 2 {
		return 0
	}
	deg, err := strconv.ParseFloat(val[0:p-2], 64)
	if err != nil {
		return 0
	}
	min, err := strconv.ParseFloat(val[p-2:], 64)
	if err != nil {
		return 0
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

func epd_write(i image.Image) error {
	cmd, err := os.OpenFile("/dev/epd/command", os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	img, err := os.OpenFile("/dev/epd/LE/display", os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	cmd.Write([]byte("C"))
	netpbm.PbmData(img, i)
	cmd.Write([]byte("P"))
	img.Close()
	cmd.Close()
	return nil
}

func show_data(name, value string) image.Image {
	i := image.NewRGBA(image.Rect(0, 0, 264, 176))
	draw.Src.Draw(i, i.Bounds(), image.White, image.ZP)
	tt, _ := truetype.Parse(gomedium.TTF)
	face := truetype.NewFace(tt, &truetype.Options{
		Size:    52,
		Hinting: font.HintingNone,
		DPI:     72,
	})

	d := &font.Drawer{
		Dst:  i,
		Src:  image.Black,
		Face: face,
	}
	d.Dot = fixed.P(60, 80)
	d.DrawString(name)
	d.Dot = fixed.P(60, 160)
	d.DrawString(value)

	return i
}

func show_speed() error {
	f, _ := os.Open("/dev/ttyUSB0")
	setbaud(f.Fd())
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		fmt.Printf("USB0 Read: %v\n", rec)
		if rec[0] == "$GPRMC" {
			var g GPRMC
			g.FromNMEA(rec)
			img := show_data("Speed", strconv.FormatFloat(float64(g.Speed), 'f', -1, 32))
			epd_write(img)
			break
		}
	}
	f.Close()
	return nil
}

func setbaud(fd uintptr) error {
	t := syscall.Termios{}

	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&t)),
		0,
		0,
		0,
	)
	t.Cflag = 0xcbc
	_, _, errno = syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&t)),
		0,
		0,
		0,
	)
	return errno
}

func main() {
	for {
		err := show_speed()
		/*
			fmt.Println("Show data")
			img := show_data("Speed", "6.3")
			fmt.Println("Write image")
			err := epd_write(img)
		*/
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(1 * time.Minute)
	}
}

/*

http://www.catb.org/gpsd/installation.html
https://github.com/tarm/serial/blob/master/serial_linux.go
Set baud
cflagToUse := syscall.CREAD | syscall.CLOCAL | syscall.B4800
t := syscall.Termios{
		Iflag:  syscall.IGNPAR,
		Cflag:  cflagToUse,
		Cc:     [32]uint8{syscall.VMIN: vmin, syscall.VTIME: vtime},
		Ispeed: syscall.B4800,
		Ospeed: syscall.B4800,
	}

if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&t)),
		0,
		0,
		0,
	); errno != 0 {
		return nil, errno
	}

*/
