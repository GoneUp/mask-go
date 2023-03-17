package mask

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	logrus "github.com/sirupsen/logrus"
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

var uploadRunning bool
var currentUpload uploadBuffer

var adapter = bluetooth.DefaultAdapter
var btDevice *bluetooth.Device
var genralBtChar *bluetooth.DeviceCharacteristic
var uploadNotificationBtChar *bluetooth.DeviceCharacteristic
var uploadBtChar *bluetooth.DeviceCharacteristic

const btMaxPacketSize int = 100 //packets above this size are not accepted by the mask, relevant for text upload
const btPaddedPacketSize byte = 16

var log logrus.Logger

func InitAndConnect(MoreLogging bool) error {
	log = *logrus.New()
	if MoreLogging {
		log.SetLevel(logrus.InfoLevel)
	} else {
		log.SetLevel(logrus.WarnLevel)
	}

	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	// Start scanning.
	ch := make(chan bluetooth.ScanResult, 1)

	log.Info("scanning...")
	err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		log.Trace("found device: ", device.Address.String(), device.RSSI, device.LocalName())

		if strings.HasPrefix(device.LocalName(), "MASK") {
			log.Info("found mask: ", device.LocalName())
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
			return err
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
		return err
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
		log.Error("one of the bt chars is nil!")
		return fmt.Errorf("bt char is nil")
	}
	log.Info("generall ", genralBtChar.String())

	err = uploadNotificationBtChar.EnableNotifications(maskUploadCallback)
	must("mask notify", err)

	//reset state
	uploadRunning = false
	currentUpload = uploadBuffer{}
	return nil
}

// disconnects the bt device
func Shutdown() {
	if btDevice != nil {
		btDevice.Disconnect()
		time.Sleep(1 * time.Second)
	}
}

func IsConnected() bool {
	return btDevice != nil
}

// Sets the scroll mode
// 01 = steady
// 02 = blink
// 03 = scroll left
// 04 = scroll right
// 05 = steady
func SetMode(mode byte) error {
	modeStr := []byte("MODE")

	buf := []byte{}
	buf = append(buf, 5) //len
	buf = append(buf, modeStr...)
	buf = append(buf, mode)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("out: ", buf)

	return SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// Sets how bright the thing is
func SetLight(brightness byte) error {
	modeStr := []byte("LIGHT")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, brightness)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("out: ", buf)

	return SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// Sets a static predefined image
func SetImage(image byte) error {
	modeStr := []byte("IMAG")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, image)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("out: ", buf)

	return SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// Sets static predefined animation
func SetAnimation(image byte) error {
	modeStr := []byte("ANIM")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, image)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("out: ", buf)

	return SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// Sets DIY iamge
func SetDIYImage(image byte) error {
	modeStr := []byte("PLAY")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, byte(1))
	buf = append(buf, image)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("out: ", buf)

	return SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// Sets speed, range 0-255
func SetTextSpeed(speed byte) error {
	modeStr := []byte("SPEED")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, speed)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("out: ", buf)

	return SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// For text mode: sets special backgrounds
// mode:
// 00-03= text gradients ()
// 04-07= background image (4 = x mask, 5 = christmas, 6 = love, 7 = scream)
func SetTextColorMode(enable byte, mode byte) error {
	modeStr := []byte("M")

	buf := []byte{}
	buf = append(buf, 3) //len
	buf = append(buf, modeStr...)
	buf = append(buf, enable)
	buf = append(buf, mode)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("out: ", buf)

	return SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// Sets a foreground text color in RGB
func SetTextFrontColor(enable byte, r byte, g byte, b byte) error {
	modeStr := []byte("FC")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, enable)
	buf = append(buf, r)
	buf = append(buf, g)
	buf = append(buf, b)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("out: ", buf)

	return SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// Sets a background text color in RGB
func SetTextBackgroundColor(enable byte, r byte, g byte, b byte) error {
	modeStr := []byte("BG")

	buf := []byte{}
	buf = append(buf, 6) //len
	buf = append(buf, modeStr...)
	buf = append(buf, enable)
	buf = append(buf, r)
	buf = append(buf, g)
	buf = append(buf, b)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("out: ", buf)

	return SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// UPLOAD
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

// Comfort function that generates an image/bitmap from an string and starts the upload to the mask
// Text Color=White
func SetText(text string) error {
	pixelMap := GetTextImage(text)
	bitmap, err := EncodeBitmapForMask(pixelMap)
	if err != nil {
		return err
	}

	colorArray := EncodeColorArrayForMask(len(pixelMap))
	log.Infof("For text %s: bitmap len %d, color array len %d", text, len(bitmap), len(colorArray))

	return InitUpload(bitmap, colorArray)
}

func InitUpload(bitmap []byte, colorArray []byte) error {
	if !IsConnected() {
		return fmt.Errorf("not connected")
	}
	if uploadRunning {
		log.Warn("Mask upload is already running!")
		return fmt.Errorf("mask upload already running")
	}

	//prep struct
	currentUpload = uploadBuffer{}
	currentUpload.bitmap = bitmap
	currentUpload.colorArray = colorArray
	currentUpload.totalLen = uint16(len(currentUpload.bitmap) + len(currentUpload.colorArray))
	currentUpload.bytesSent = 0
	currentUpload.completeBuffer = make([]byte, 0)
	currentUpload.completeBuffer = append(currentUpload.completeBuffer, currentUpload.bitmap...)
	currentUpload.completeBuffer = append(currentUpload.completeBuffer, currentUpload.colorArray...)
	log.Debug("bitmap: ", currentUpload.bitmap)
	log.Debug("colorArray: ", currentUpload.colorArray)
	//log.Debug("completeBuffer: ", currentUpload.completeBuffer)

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

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Infof("Upload Init upload totlen %d, bit len %d out: %v", currentUpload.totalLen, len(currentUpload.bitmap), buf)

	err := SendDataToBtChar(genralBtChar, EncryptAes128(buf))
	if err == nil {
		uploadRunning = true
	}
	return err
}

func maskUploadCallback(encBuffer []byte) {
	buf := DecryptAes128Ecb(encBuffer)
	strLen := buf[0]
	resp := string(buf[1 : strLen+1])

	log.Infof("Callback data: %v, hex: %s, parsed resp %s", buf, hex.EncodeToString(buf), resp)

	if resp == "DATSOK" {
		//ok, we can start to send
		maskUploadPart()
	} else if resp == "REOK" {
		if currentUpload.bytesSent < currentUpload.totalLen {
			maskUploadPart()
		} else {
			maskFinishUpload()
		}
	} else if resp == "DATCPOK" {
		//yay we are done
		uploadRunning = false
	} else if resp == "PLAYOK" {
		//do nothing, resp to diy image
	} else {
		log.Warnf("unknown notify response: %s", resp)
	}

}

func maskUploadPart() {
	if currentUpload.bytesSent == currentUpload.totalLen {
		return
	}

	var maxSize uint16 = uint16(btMaxPacketSize) - 2
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
	SendDataToBtChar(uploadBtChar, buf)

	currentUpload.bytesSent += uint16(bytesToSend)
	currentUpload.packetCount += 1
}

func maskFinishUpload() {
	modeStr := []byte("DATCP")

	buf := []byte{}
	buf = append(buf, 5) //len
	buf = append(buf, modeStr...)

	buf = padByteArray(buf, btPaddedPacketSize)
	log.Debug("finish upload out: ", buf)

	SendDataToBtChar(genralBtChar, EncryptAes128(buf))
}

// always returns a byte array with specified len
func padByteArray(array []byte, len byte) []byte {
	//filling with zeros are a bad pratice, but then again the mask is using aes-ecb lol
	out := make([]byte, len)
	copy(out, array)
	return out
}

// SendRawData sends a byte buffer to  genralBtChar (UUID 0x9600)
func SendRawData(sendbuf []byte) error {
	return SendDataToBtChar(genralBtChar, sendbuf)
}

// SendRawData sends a byte buffer to the specifed char, breaking it up if the buffer is to big
func SendDataToBtChar(device *bluetooth.DeviceCharacteristic, sendbuf []byte) error {
	log.Debugf("sendStuff (len %d): %v\n", len(sendbuf), sendbuf)
	// Send the sendbuf after breaking it up in pieces.
	for len(sendbuf) != 0 {
		// Chop off up to 20 bytes from the sendbuf.
		partlen := btMaxPacketSize
		if len(sendbuf) < btMaxPacketSize {
			partlen = len(sendbuf)
		}
		part := sendbuf[:partlen]
		sendbuf = sendbuf[partlen:]
		// This performs a "write command" aka "write without response".
		_, err := device.WriteWithoutResponse(part)
		if err != nil {
			log.Error("could not send: ", err)
			return err
		}
	}

	return nil
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
