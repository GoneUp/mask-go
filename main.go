package main

import (
	"bufio"
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	"crypto/aes"
	"encoding/binary"
	"encoding/hex"

	log "github.com/sirupsen/logrus"
	"tinygo.org/x/bluetooth"
)

type uploadBuffer struct {
	bitmap         []byte
	colorArray     []byte
	completeBuffer []byte
	totalLen       uint16
	bytesSent      uint16
	packetCount    byte
}

var currentUpload uploadBuffer

var adapter = bluetooth.DefaultAdapter
var btDevice *bluetooth.Device
var genralBtChar *bluetooth.DeviceCharacteristic
var uploadNotificationBtChar *bluetooth.DeviceCharacteristic
var uploadBtChar *bluetooth.DeviceCharacteristic

var (
	msg01 = EncryptAes128Hex("054d4f444503661a65c58086978c1e6e")
	msg02 = EncryptAes128Hex("054d4f4445040cc05ea463180d2461ea")
	msg03 = EncryptAes128Hex("054d4f444502dd80fd1cb279fd43ede6")
)

//found device: 00:4A:00:04:C3:11 -78 MASK-04C311

func init() {
	log.SetLevel(log.DebugLevel)
}

//text3 qgQZ´
func main() {
	drawMode := flag.Bool("draw", false, "enables drawmode")
	text := flag.String("text", "test", "string to draw")
	flag.Parse()

	if *drawMode {
		log.Info("drawing ", *text)
		GetTextImage(*text)
		return
	}

	log.SetReportCaller(true)
	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	// Start scanning.
	ch := make(chan bluetooth.ScanResult, 1)

	log.Info("scanning...")
	err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		log.Info("found device:", device.Address.String(), device.RSSI, device.LocalName())

		if strings.HasPrefix(device.LocalName(), "MASK") {
			log.Info("found mask", device.LocalName())
			adapter.StopScan()
			ch <- device
		}
	})
	must("start scan", err)

	select {
	case result := <-ch:
		btDevice, err = adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err != nil {
			log.Error("connection abort: ", err.Error())
			return
		}

		log.Info("connected to ", result.Address.String())
	}

	// get services
	log.Info("discovering services/characteristics")

	//mask service
	maskUUID, err := bluetooth.ParseUUID("0000fff0-0000-1000-8000-00805f9b34fb")
	must("ParseUUID", err)

	srvcs, err := btDevice.DiscoverServices([]bluetooth.UUID{maskUUID})
	must("discover services", err)
	maskService := srvcs[0]
	log.Info("mask svcs", maskService.String())

	//setup needed chars
	generalUUID, err := bluetooth.ParseUUID("d44bc439-abfd-45a2-b575-925416129600")
	must("ParseUUID", err)
	uploadNotifyUUID, err := bluetooth.ParseUUID("d44bc439-abfd-45a2-b575-925416129601")
	must("ParseUUID", err)
	uploadUUID, err := bluetooth.ParseUUID("d44bc439-abfd-45a2-b575-92541612960a")
	must("ParseUUID", err)

	chars, err := maskService.DiscoverCharacteristics(nil)
	if err != nil {
		log.Error("disc chars: ", err)
	}

	for i := range chars {
		dc := &chars[i]

		log.Info(chars[i].String())
		if dc.UUID().String() == generalUUID.String() {
			genralBtChar = dc

		} else if dc.UUID().String() == uploadNotifyUUID.String() {
			uploadNotificationBtChar = dc

		} else if dc.UUID().String() == uploadUUID.String() {
			uploadBtChar = dc
		}
	}

	if genralBtChar == nil || uploadBtChar == nil || uploadNotificationBtChar == nil {
		panic("one of the bt chars is nil!")
	}
	log.Info("generall ", genralBtChar.String())

	err = uploadNotificationBtChar.EnableNotifications(MaskUploadCallback)
	must("mask notify", err)

	doDemoControlLoop()
}

func doDemoControlLoop() {
	var err error
	//simple cmd struct
	for {
		log.Info("Input cmd please:")
		reader := bufio.NewReader(os.Stdin)
		cmd, _ := reader.ReadString('\n')
		cmdSplit := strings.Split(strings.Trim(cmd, "\n"), " ")

		switch cmdSplit[0] {
		case "mode":
			for i := 1; i < 5; i++ {
				log.Infof("trying to send mode %d\n", i)
				MaskSetMode(byte(i))
				time.Sleep(5 * time.Second)
			}

		case "mode2":
			log.Info("trying to send play 01")
			sendStuff(genralBtChar, msg01)
			time.Sleep(5 * time.Second)

			log.Info("trying to send play 02")
			sendStuff(genralBtChar, msg02)
			time.Sleep(5 * time.Second)

			log.Info("trying to send play 03")
			sendStuff(genralBtChar, msg03)
			time.Sleep(5 * time.Second)

		case "light":
			val, err := strconv.ParseUint(cmdSplit[1], 10, 8)
			must("ParseInt", err)

			MaskSetLight(byte(val))

		case "image":
			val, err := strconv.ParseUint(cmdSplit[1], 10, 8)
			must("ParseInt", err)

			MaskSetImage(byte(val))

		case "speed":
			val, err := strconv.ParseUint(cmdSplit[1], 10, 8)
			must("ParseInt", err)

			MaskSetSpeed(byte(val))

		case "color":
			val, err := strconv.ParseUint(cmdSplit[1], 10, 8)
			must("ParseInt", err)

			MaskSetTextColorMode(1, byte(val))

		case "text":
			/*
				UPLOAD PROCESS:
				DATS > Mask
				Mask > DATSOKP
				per packet
					Upload ...
					Mask > REOKOKP
					DATCP > Mask
				Mask > DATCPOK
			*/
			currentUpload = uploadBuffer{
				bitmap: []byte{0xFF, 0xFF, 0x00, 0x00, 0xFF, 0xFF, 0x00, 0x00, 0xFF, 0xFF, 0x00, 0x00},
				colorArray: []byte{0xFF, 0x00, 0x00, 0xFF, 0x00, 0x00,
					0x00, 0xFF, 0x00, 0x00, 0xFF, 0x00,
					0xFF, 0x00, 0x00, 0xFF, 0x00, 0x00,
					0x00, 0xFF, 0x00, 0x00, 0xFF, 0x00},
			}

			//test str

			MaskInitUpload()
		case "text2":
			currentUpload = uploadBuffer{}
			currentUpload.bitmap, err = hex.DecodeString("020002003ff83ffc020402040000000000f001f8034c0244034401cc00c80000018803cc024402640224033c01180000020002003ff83ffc0204020400000000")
			must("decode", err)
			currentUpload.colorArray, err = hex.DecodeString("fffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffcfffffc")
			must("decode", err)
			//test str

			MaskInitUpload()

		case "text3":
			//text3 jonasjemmy hat Köngisrose x1 gesendet 
			text := strings.TrimPrefix(cmd, "text3 ")

			pixelMap := GetTextImage(text)
			bitmap, err := EncodeBitmapForMask(pixelMap)
			must("encode", err)
			colorArray := EncodeColorArrayForMask(len(pixelMap))

			log.Infof("For text %s: bitmap len %d, color array len %d", text, len(bitmap), len(colorArray))

			currentUpload = uploadBuffer{}
			currentUpload.bitmap = bitmap
			currentUpload.colorArray = colorArray

			MaskInitUpload()

		case "exit":
			btDevice.Disconnect()
			time.Sleep(1 * time.Second)
			os.Exit(0)

		default:
			log.Info("Unknown cmd")
		}
	}
}

//Sets the scroll mode
func MaskSetMode(mode byte) {
	modeStr := []byte("MODE")

	buf := []byte{}
	buf = append(buf, 5) //len
	buf = append(buf, modeStr...)
	buf = append(buf, mode)

	buf = padByteArray(buf, 16)
	log.Debug("out: ", buf)

	sendStuff(genralBtChar, EncryptAes128(buf))
}

//Sets how bright the thing is
func MaskSetLight(brightness byte) {
	modeStr := []byte("LIGHT")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, brightness)

	buf = padByteArray(buf, 16)
	log.Debug("out: ", buf)

	sendStuff(genralBtChar, EncryptAes128(buf))
}

//Sets how bright the thing is
func MaskSetImage(image byte) {
	modeStr := []byte("IMAG")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, image)

	buf = padByteArray(buf, 16)
	log.Debug("out: ", buf)

	sendStuff(genralBtChar, EncryptAes128(buf))
}

//Sets anim
func MaskSetAnim(image byte) {
	modeStr := []byte("ANIM")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, image)

	buf = padByteArray(buf, 16)
	log.Debug("out: ", buf)

	sendStuff(genralBtChar, EncryptAes128(buf))
}

//Sets speed, range 0-255
func MaskSetSpeed(speed byte) {
	modeStr := []byte("SPEED")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, speed)

	buf = padByteArray(buf, 16)
	log.Debug("out: ", buf)

	sendStuff(genralBtChar, EncryptAes128(buf))
}

//Sets speed, range 0-255
func MaskSetTextColorMode(enable byte, mode byte) {
	modeStr := []byte("M")

	buf := []byte{}
	buf = append(buf, 3) //len
	buf = append(buf, modeStr...)
	buf = append(buf, enable)
	buf = append(buf, mode)

	buf = padByteArray(buf, 16)
	log.Debug("out: ", buf)

	sendStuff(genralBtChar, EncryptAes128(buf))
}

//UPLOAD
func MaskInitUpload() {
	//prep struct
	currentUpload.totalLen = uint16(len(currentUpload.bitmap) + len(currentUpload.colorArray))
	currentUpload.bytesSent = 0
	currentUpload.completeBuffer = make([]byte, 0)
	currentUpload.completeBuffer = append(currentUpload.completeBuffer, currentUpload.bitmap...)
	currentUpload.completeBuffer = append(currentUpload.completeBuffer, currentUpload.colorArray...)
	log.Info(currentUpload.bitmap)
	log.Info(currentUpload.colorArray)
	log.Info(currentUpload.completeBuffer)

	//09DATS - 2 byte total len - 2 byte bitmap len
	modeStr := []byte("DATS")

	buf := []byte{}
	buf = append(buf, 9)
	buf = append(buf, modeStr...)

	intBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(intBytes, currentUpload.totalLen)
	buf = append(buf, intBytes...)

	binary.BigEndian.PutUint16(intBytes, uint16(len(currentUpload.bitmap)))
	buf = append(buf, intBytes...)

	buf = append(buf, byte(0))

	buf = padByteArray(buf, 16)
	log.Infof("Init upload totlen %d, bit len %d out: %v", currentUpload.totalLen, len(currentUpload.bitmap), buf)

	sendStuff(genralBtChar, EncryptAes128(buf))
}

func MaskUploadCallback(encBuffer []byte) {
	buf := DecryptAes128Ecb(encBuffer)
	strLen := buf[0]
	resp := string(buf[1 : strLen+1])

	log.Infof("data: %v, hex: %s, parsed resp %s", buf, hex.EncodeToString(buf), resp)

	if resp == "DATSOK" {
		//ok, we can start to send
		MaskUploadPart()
	} else if resp == "REOK" {
		if currentUpload.bytesSent < currentUpload.totalLen {
			MaskUploadPart()
		} else {
			MaskFinishUpload()
		}
	} else if resp == "DATCPOK" {
		MaskSetMode(3)
		//do nothing
	} else {
		log.Warn("unknown notify response")
	}

}

func MaskUploadPart() {
	if currentUpload.bytesSent == currentUpload.totalLen {
		return
	}

	var maxSize uint16 = 18
	var data []byte

	var bytesToSend byte
	if currentUpload.bytesSent+maxSize < currentUpload.totalLen {
		bytesToSend = byte(maxSize)
	} else {
		bytesToSend = byte(currentUpload.totalLen - currentUpload.bytesSent)
	}
	data = make([]byte, bytesToSend)
	copy(data, currentUpload.completeBuffer[currentUpload.bytesSent:currentUpload.bytesSent+uint16(bytesToSend)])

	buf := []byte{}
	buf = append(buf, bytesToSend+1) //len
	buf = append(buf, currentUpload.packetCount)
	buf = append(buf, data...)

	//buf = padByteArray(buf, 100)

	log.Infof("upload data pkt %d: len %d, data %s", currentUpload.packetCount, bytesToSend, hex.EncodeToString(buf))
	sendStuff(uploadBtChar, buf)

	currentUpload.bytesSent += uint16(bytesToSend)
	currentUpload.packetCount += 1
}

func MaskFinishUpload() {
	modeStr := []byte("DATCP")

	buf := []byte{}
	buf = append(buf, 5) //len
	buf = append(buf, modeStr...)

	buf = padByteArray(buf, 16)
	log.Debug("finish upload out: ", buf)

	sendStuff(genralBtChar, EncryptAes128(buf))
}

//always returns a byte array with specified len
func padByteArray(array []byte, len byte) []byte {
	//filling with zeros are a bad pratice, but then again the mask is using aes-ecb lol
	out := make([]byte, len)
	copy(out, array)
	return out
}

func sendStuff(device *bluetooth.DeviceCharacteristic, sendbuf []byte) error {
	log.Debugf("sendStuff (len %d): %v\n", len(sendbuf), sendbuf)
	// Send the sendbuf after breaking it up in pieces.
	for len(sendbuf) != 0 {
		// Chop off up to 20 bytes from the sendbuf.
		partlen := 20
		if len(sendbuf) < 20 {
			partlen = len(sendbuf)
		}
		part := sendbuf[:partlen]
		sendbuf = sendbuf[partlen:]
		// This performs a "write command" aka "write without response".
		_, err := device.WriteWithoutResponse(part)
		if err != nil {
			log.Info("could not send:", err.Error())
			return err
		}
	}

	return nil
}

//Takes hex string, len must be divsiable by 16
func EncryptAes128Hex(hexstring string) []byte {
	data, err := hex.DecodeString(hexstring)
	must("key", err)

	return EncryptAes128(data)
}

//Len must be divsiable by 16
//AES-ECB
func EncryptAes128(data []byte) []byte {
	//validate
	blockSize := 16
	if len(data) != blockSize {
		panic(0)
	}

	// create cipher
	key, err := hex.DecodeString("32672f7974ad43451d9c6c894a0e8764")
	must("key", err)
	c, err := aes.NewCipher(key)
	must("cipher", err)

	// allocate space for ciphered data
	out := make([]byte, blockSize)

	// encrypt
	c.Encrypt(out, data)

	return out
}

//Len must be divsiable by 16
//AES-ECB
func DecryptAes128Ecb(data []byte) []byte {
	key, err := hex.DecodeString("32672f7974ad43451d9c6c894a0e8764")
	must("key", err)
	cipher, err := aes.NewCipher([]byte(key))
	must("cipher", err)

	decrypted := make([]byte, len(data))
	size := 16

	for bs, be := 0, size; bs < len(data); bs, be = bs+size, be+size {
		cipher.Decrypt(decrypted[bs:be], data[bs:be])
	}

	return decrypted
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
