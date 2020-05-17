package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/nfnt/resize"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

type AddressList []*net.IPAddr

var addressList atomic.Value
var packetCount uint64

func main() {
	img := openImage()

	pixels := getRandomizedPixelPoints(img)
	fmt.Println("generating done")

	wg := sync.WaitGroup{}

	wg.Add(1)
	go generateAddresses(pixels, &wg)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go openSocket(&wg)
	}
	fmt.Println("connections done")

	go func() {
		for {
			count := atomic.SwapUint64(&packetCount, 0)

			fmt.Println(fmt.Sprintf("%d kpps", count))

			time.Sleep(1 * time.Second)
		}
	}()

	wg.Wait()
}

func generateAddresses(pixels []Pixel, w *sync.WaitGroup) {
	defer w.Done()

	var oX, oY uint32 = 0, 0
	for {
		oX, oY = oX+100, oY+0

		if oX > 500 {
			oX = 0
		}

		if oY > 500 {
			oY = 0
		}

		newAddresses := make(AddressList, len(pixels))
		for i, pixel := range pixels {
			address := fmt.Sprintf("2A06:1E81:F147:%d:%d:%x:%x:%x",
				uint32(pixel.X)+oX,
				uint32(pixel.Y)+oY,
				pixel.R, pixel.G, pixel.B)

			addr, err := net.ResolveIPAddr("ip", address)
			if err != nil {
				panic(err)
			}

			newAddresses[i] = addr
		}

		addressList.Store(newAddresses)
		fmt.Println("Stored new Data")

		time.Sleep(1 * time.Second)
	}
}

var iCMPPacket = getICMPPacket()

func getICMPPacket() []byte {
	msg := &icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{
			ID:   0,
			Seq:  0,
			Data: []byte{},
		},
	}

	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		panic(err)
	}

	return msgBytes
}

func openSocket(w *sync.WaitGroup) {
	defer w.Done()

	c, err := icmp.ListenPacket("ip6:ipv6-icmp", "")
	if err != nil {
		fmt.Printf("Error listening for ICMP packets: %s\n", err.Error())
		panic(err)
	}

	i := rand.Int()
	for {
		aInterface := addressList.Load()

		if aInterface == nil {
			continue
		}

		aList := aInterface.(AddressList)
		if len(aList) == 0 {
			continue
		}

		llen := len(aList) - 1

		i = int(math.Min(float64(i), float64(llen)))

		if i > llen {
			i -= rand.Intn(llen)
		}

		m := aList[i]

		if _, err := c.WriteTo(iCMPPacket, m); err != nil {
			if errors.Is(err, syscall.ENOBUFS) {
				continue
			}
			if err != nil {
				fmt.Println(err)
			}
		}

		atomic.AddUint64(&packetCount, 1)
		i++
	}
}

type Pixel struct {
	image.Point
	color.RGBA
}

func openImage() image.Image {
	file, err := os.OpenFile("image.png", os.O_RDONLY, 0755)
	if err != nil {
		panic(err)
	}

	img, err := png.Decode(file)
	if err != nil {
		panic(err)
	}

	return resize.Resize(500, 0, img, resize.Lanczos3)
}

func getRandomizedPixelPoints(img image.Image) []Pixel {
	var points []Pixel

	for x := 0; x < img.Bounds().Max.X; x++ {
		for y := 0; y < img.Bounds().Max.Y; y++ {
			r, g, b, a := img.At(x, y).RGBA()

			points = append(points, Pixel{
				Point: image.Point{
					X: x,
					Y: y,
				},
				RGBA: color.RGBA{
					R: uint8(r),
					G: uint8(g),
					B: uint8(b),
					A: uint8(a),
				},
			})
		}
	}

	for i := range points {
		j := rand.Intn(i + 1)
		points[i], points[j] = points[j], points[i]
	}

	return points
}
