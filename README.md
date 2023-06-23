# mask-go
A shining mask implementation writen in Golang. It is intended to be used with masks controlled by the Shining mask app.

It seems that there are several different vendor models avaialable, but they share the same app/protocol. E.g. `Lumen Couture LED Face Changing Mask`.

"Normal android control app": https://play.google.com/store/apps/details?id=cn.com.heaton.shiningmask

Features: 
- Connection with mask over BLE (using tinygo.org/x/bluetooth)
- Controlling the mask remotely (Brightness, Static image, Anmination, Text speed, Text color) 
- Uploading/showing text on the mask 

## Usage
Installation with
`go get https://github.com/GoneUp/mask-go`


Simple usage example
```go
import "mask-go/mask"

mask.InitAndConnect(true)
mask.SetText("Hello world")
```

A demo application is included in the [main.go] file.


## Protocol 
The mask commuicates over Bluetooth LE with the mask. The protocol iself is fairly simply, however there is a AES ECB encryption. 

Please review this reddit post, it contains all basic protocol details and the AES key used: https://www.reddit.com/r/ReverseEngineering/comments/lr9xxr/comment/h14nm39/?utm_source=reddit&utm_medium=web2x&context=3

The most complicated bit of the mask-go is the text upload. The mask just accepts a bitmap/colors in a very custom format. 
So to upload text, we have to draw it to a bitmap, convert it accordingly to the protcol and send it in custom manner to the app.

The protocol is implemented in [mask.go] and [draw.go].

Braindumping protocol details:

```
Methods 

ccroll mode:
05MODEnn 
nn 01 = steady
nn 02 = blink
nn 03 = scroll left
nn 04 = scroll right
nn 05 = steady


custom text front color:
06FC<00/01> <RR> <BB> <GG>

custom text back color:
06BC<00/01> <RR> <BB> <GG>

speed:
06SPEEDnn

set text color mode:
03M<00/01><00-07>

00-03= text gradients 
05-07= background animations

set light
06LIGHTnn

set image
06IMAGnn

set anmiation
06ANIMnn

set diy image
06PLAY01nn



Text upload:

	UPLOAD PROCESS:
	DATS > Mask
	Mask > DATSOKP
	per packet
		Upload ...
		Mask > REOK
	DATCP > Mask
	Mask > DATCPOK

09DATS - 2 byte total len - 2 byte bitmap len 

Image data:
The display of the mask is 16 pixel high, the data is sent accordingly per pixel colum. 
For each pixel in a pixel colum a bit is set (on/off). For each colum there is also a RGB value. 

In more formal form:
  for each column:
    column encoded in 2byte 
      b1: line 0-7, bit 0-7
      b2: line 7-15, bit 0-7
  for each colum
    3 byte RGB



example: 8 colums, 12b pixels, 24b color = 36byte
FFFF0000 FFFF0000 FFFF0000 FFFF0000  FF0000 FF0000 00FF00 00FF00 FF0000 FF0000 00FF00 00FF00

Implemented in draw.go

sending in the format of max 100b packets:
  <len with count><pkt count><data, max 98b>
```




## Credits

I was able to learn a lot from other open-source code:

All peeps in this reddit thread https://www.reddit.com/r/ReverseEngineering/comments/lr9xxr/help_me_figure_out_how_to_reverse_engineer_the/

beclaminde with this project https://github.com/beclamide/mask-controller

shawnrancatore with this hacky project https://github.com/shawnrancatore/shining-mask
