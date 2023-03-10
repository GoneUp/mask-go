package main

import (
	"bufio"
	"flag"
	"mask-go/mask"
	"os"
	"strconv"
	"strings"
	"time"

	"encoding/hex"

	"github.com/icza/gox/imagex/colorx"
	log "github.com/sirupsen/logrus"
)

type uploadBuffer struct {
	bitmap         []byte
	colorArray     []byte
	completeBuffer []byte
	totalLen       uint16
	bytesSent      uint16
	packetCount    byte
}

var (
	msg01 = mask.EncryptAes128Hex("054d4f444503661a65c58086978c1e6e")
	msg02 = mask.EncryptAes128Hex("054d4f4445040cc05ea463180d2461ea")
	msg03 = mask.EncryptAes128Hex("054d4f444502dd80fd1cb279fd43ede6")
)

//found device: 00:4A:00:04:C3:11 -78 MASK-04C311

func init() {
	log.SetLevel(log.DebugLevel)
}

// text3 qgQZ´
func main() {
	drawMode := flag.Bool("draw", false, "enables drawmode")
	text := flag.String("text", "test", "string to draw")
	flag.Parse()

	if *drawMode {
		log.Info("drawing ", *text)
		mask.GetTextImage(*text)
		return
	}

	log.SetReportCaller(true)
	log.Info("MaskCmd started. Connecting to mask...")
	err := mask.InitAndConnect()
	if err != nil {
		log.Fatalf("Init failed with error %v", err)
	}

	doDemoControlLoop()
}

func doDemoControlLoop() {
	//simple cmd struct
	for {
		log.Info("Input cmd please:")
		reader := bufio.NewReader(os.Stdin)
		cmd, _ := reader.ReadString('\n')
		cmdSplit := strings.Split(strings.Trim(cmd, "\n"), " ")

		switch cmdSplit[0] {
		case "connect":
			err := mask.InitAndConnect()
			if err != nil {
				log.Fatalf("Init failed with error %v", err)
			}
		

		case "allmode":
			for i := 1; i < 5; i++ {
				log.Infof("trying to send mode %d\n", i)
				mask.MaskSetMode(byte(i))
				time.Sleep(5 * time.Second)
			}

		case "mode":
			val, err := strconv.ParseUint(cmdSplit[1], 10, 8)
			must("ParseInt", err)

			mask.MaskSetMode(byte(val))

		case "mode2":
			log.Info("trying to send play 01")
			mask.SendRawData(msg01)
			time.Sleep(5 * time.Second)

			log.Info("trying to send play 02")
			mask.SendRawData(msg02)
			time.Sleep(5 * time.Second)

			log.Info("trying to send play 03")
			mask.SendRawData(msg03)
			time.Sleep(5 * time.Second)

		case "light":
			val, err := strconv.ParseUint(cmdSplit[1], 10, 8)
			must("ParseInt", err)

			mask.MaskSetLight(byte(val))

		case "image":
			val, err := strconv.ParseUint(cmdSplit[1], 10, 8)
			must("ParseInt", err)

			mask.MaskSetImage(byte(val))

		case "speed":
			val, err := strconv.ParseUint(cmdSplit[1], 10, 8)
			must("ParseInt", err)

			mask.MaskSetSpeed(byte(val))

		case "color":
			val, err := strconv.ParseUint(cmdSplit[1], 10, 8)
			must("ParseInt", err)

			mask.MaskSetTextColorMode(1, byte(val))

		case "fg":
			c, err := colorx.ParseHexColor(cmdSplit[1])
			if err == nil {
				mask.MaskSetTextFrontColor(1, c.R, c.G, c.B)
			} else {
				log.Warn("wrong format, use #FFFFFF")
			}

		case "bg":
			c, err := colorx.ParseHexColor(cmdSplit[1])
			must("ParseHexColor", err)

			if err == nil {
				mask.MaskSetTextBackgroundColor(1, c.R, c.G, c.B)
			} else {
				log.Warn("wrong format, use #FFFFFF")
			}

		case "text":
			//simple red white stripes
			bitmap := []byte{0xFF, 0xFF, 0x00, 0x00, 0xFF, 0xFF, 0x00, 0x00, 0xFF, 0xFF, 0x00, 0x00}
			colorArray := []byte{0xFF, 0x00, 0x00, 0xFF, 0x00, 0x00,
				0x00, 0xFF, 0x00, 0x00, 0xFF, 0x00,
				0xFF, 0x00, 0x00, 0xFF, 0x00, 0x00,
				0x00, 0xFF, 0x00, 0x00, 0xFF, 0x00}

			mask.MaskInitUpload(bitmap, colorArray)
		case "text2":
			//decoded from bt dump, sets test as text
			bitmap, err := hex.DecodeString("020002003ff83ffc020402040000000000f001f8034c0244034401cc00c80000018803cc024402640224033c01180000020002003ff83ffc0204020400000000")
			must("decode", err)
			colorArray, err := hex.DecodeString("fffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffc")
			must("decode", err)

			mask.MaskInitUpload(bitmap, colorArray)

		case "text3":
			//text3 jonasjemmy hat Köngisrose x1 gesendet
			text := strings.TrimPrefix(cmd, "text3 ")
			text = strings.TrimSpace(text)

			mask.MaskSetText(text)

		case "exit":
			mask.Shutdown()
			os.Exit(0)

		default:
			log.Info("Unknown cmd")
		}
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
