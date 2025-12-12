package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"


	"github.com/dosgo/grdp/client"
	"github.com/dosgo/grdp/core"
	"github.com/dosgo/grdp/protocol/pdu"
)

var (
	gc            Control
	fApp          fyne.App
	fWindow       fyne.Window
	width, height int
	screenWidget  *InteractiveScreen
)

// StartUI 入口保持不变
func StartUI(w, h int) {
	width, height = w, h
	fApp = app.New()
	fWindow = fApp.NewWindow("MSTSC")
	fWindow.Resize(fyne.NewSize(float32(width), float32(height)))

	// 初始化屏幕图像
	ScreenImage = image.NewRGBA(image.Rect(0, 0, width, height))
	//fWindow.SetFullScreen(true)

	// 构建 UI
	appMain()

	fWindow.SetOnClosed(func() {
		if gc != nil {
			gc.Close()
		}
	})

	// 启动更新协程
	update()
	go func() {
		time.Sleep(200 * time.Millisecond)
		forceMaximizeWindows(fWindow)
	}()

	fWindow.ShowAndRun()
}

// 封装一个支持交互的屏幕组件
type InteractiveScreen struct {
	widget.BaseWidget
	raster *canvas.Image
}

func NewInteractiveScreen() *InteractiveScreen {
	s := &InteractiveScreen{}
	s.ExtendBaseWidget(s)
	// 使用 RasterFromImage 引用全局 ScreenImage，这样修改 ScreenImage 后 Refresh 即可
	s.raster = canvas.NewImageFromImage(ScreenImage)
	s.raster.ScaleMode = canvas.ImageScalePixels
	s.raster.FillMode = canvas.ImageFillStretch
	return s
}

func (s *InteractiveScreen) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.raster)
}

// 实现鼠标按下接口
func (s *InteractiveScreen) MouseDown(ev *desktop.MouseEvent) {
	if gc != nil {
		btn := 0 // Left
		if ev.Button == desktop.MouseButtonSecondary {
			btn = 2
		} else if ev.Button == desktop.MouseButtonTertiary {
			btn = 1
		}
		widgetSize := s.Size() // 获取当前渲染的大小

		// 2. 定义远程屏幕的固定大小 (R_w, R_h)

		// 3. 计算远程坐标
		remoteX := int(ev.Position.X * (float32(width) / widgetSize.Width))
		remoteY := int(ev.Position.Y * (float32(height) / widgetSize.Height))
		gc.MouseDown(btn, int(remoteX), int(remoteY))
	}
}

// 实现鼠标抬起接口
func (s *InteractiveScreen) MouseUp(ev *desktop.MouseEvent) {
	if gc != nil {
		btn := 0
		if ev.Button == desktop.MouseButtonSecondary {
			btn = 2
		} else if ev.Button == desktop.MouseButtonTertiary {
			btn = 1
		}

		widgetSize := s.Size() // 获取当前渲染的大小

		// 3. 计算远程坐标
		remoteX := int(ev.Position.X * (float32(width) / widgetSize.Width))
		remoteY := int(ev.Position.Y * (float32(height) / widgetSize.Height))

		gc.MouseUp(btn, int(remoteX), int(remoteY))
	}
}

// 实现鼠标移动接口
func (s *InteractiveScreen) MouseMoved(ev *desktop.MouseEvent) {
	if gc != nil {
		gc.MouseMove(int(ev.Position.X), int(ev.Position.Y))
	}
}

// 实现鼠标滚动接口
func (s *InteractiveScreen) Scrolled(ev *fyne.ScrollEvent) {
	if gc != nil {
		// Fyne ScrollY upwards is positive, logic might need adjustment based on RDP expectation
		// Original: gc.MouseWheel(e.ScrollY, e.Point.X, e.Point.Y)
		// Fyne doesn't give X,Y in ScrollEvent easily without keeping track,
		// but usually scroll happens at current pointer.
		// For simplicity, passing 0,0 or we need to track last mouse pos.
		// Assuming generic scroll here.
		fmt.Printf("Scrolled PointEvent:%+v ev.Scrolled:%+v\r\n", ev.PointEvent, ev.Scrolled)

		var p = &pdu.PointerEvent{}
		if math.Abs(float64(ev.Scrolled.DX)) > 0 {
			p.PointerFlags |= pdu.PTRFLAGS_HWHEEL
		} else {
			p.PointerFlags |= pdu.PTRFLAGS_WHEEL
		}

		//if info.IsNegative {
		//p.PointerFlags |= pdu.PTRFLAGS_WHEEL_NEGATIVE
		//	}
		step := int(ev.Scrolled.DY)
		if (math.Abs(float64(ev.Scrolled.DX))) > 0 {
			step = int(ev.Scrolled.DX)
		}

		p.PointerFlags |= uint16(step) & pdu.WheelRotationMask
		p.XPos = uint16(ev.PointEvent.Position.X)
		p.YPos = uint16(ev.PointEvent.Position.Y)
		if gc != nil {

			if client, ok := gc.(*client.Client); ok {
				client.RdpSendInputEvents(pdu.INPUT_EVENT_MOUSE, []pdu.InputEventsInterface{p})
			}

		}
	}
}

func appMain() {
	// 登录表单部分
	label := widget.NewLabel("Welcome Mstsc")
	label.Alignment = fyne.TextAlignCenter
	// label.Color is controlled by theme in Fyne usually, but we can stick to default or custom theme

	ipEntry := widget.NewEntry()
	ipEntry.SetText("")
	ipEntry.PlaceHolder = "IP:Port"

	userEntry := widget.NewEntry()
	userEntry.SetText("")
	userEntry.PlaceHolder = "Username"

	passEntry := widget.NewPasswordEntry()
	passEntry.SetText("")
	passEntry.PlaceHolder = "Password"

	loginForm := widget.NewForm(
		widget.NewFormItem("IP:", ipEntry),
		widget.NewFormItem("User:", userEntry),
		widget.NewFormItem("Password:", passEntry),
	)

	currentConfig := core.LoadConfig()
	ipEntry.SetText(currentConfig.IP)
	userEntry.SetText(currentConfig.Username)
	passEntry.SetText(currentConfig.Password)

	formContainer := container.New(layout.NewGridWrapLayout(fyne.NewSize(300, 180)), loginForm)

	// 远程桌面显示部分
	screenWidget = NewInteractiveScreen()
	screenContainer := container.New(layout.NewMaxLayout(), screenWidget)

	// 容器切换逻辑
	var content *fyne.Container // 声明 content 变量

	btnOk := widget.NewButton("OK", func() {
		if ipEntry.Text == "" || userEntry.Text == "" || passEntry.Text == "" {
			dialog.NewInformation("提示", "请填写完整信息", fWindow).Show()
			return
		}

		core.SaveConfig(core.AppConfig{
			IP:       ipEntry.Text,
			Username: userEntry.Text,
			Password: passEntry.Text,
		})

		err, info := NewInfo(ipEntry.Text, userEntry.Text, passEntry.Text)
		info.Width, info.Height = width, height
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		err, gc = uiClient(info)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fWindow.SetContent(screenContainer)

		// 激活键盘监听
		setupKeyboard(fWindow)
	})

	btnClear := widget.NewButton("Clear", func() {
		ipEntry.SetText("")
		userEntry.SetText("")
		passEntry.SetText("")
	})

	buttonBox := container.NewHBox(layout.NewSpacer(), btnOk, btnClear, layout.NewSpacer())

	loginBox := container.NewVBox(
		layout.NewSpacer(),
		label,
		container.NewCenter(formContainer),
		buttonBox,
		layout.NewSpacer(),
	)

	// 主容器
	content = container.NewCenter(loginBox)
	fWindow.SetContent(content)

}

func setupKeyboard(w fyne.Window) {

	if deskCanvas, ok := w.Canvas().(desktop.Canvas); ok {
		deskCanvas.SetOnKeyDown(func(key *fyne.KeyEvent) {
			if gc == nil {
				return
			}
			key1 := transKey(key.Name)
			//key.Physical.ScanCode
			fmt.Printf("key.Name:%s\r\n", key.Name)
			if key1 > 0 {
				gc.KeyDown(key1, "")
			} else {
				gc.KeyDown(key.Physical.ScanCode, "")
			}
		})
		deskCanvas.SetOnKeyUp(func(key *fyne.KeyEvent) {
			if gc == nil {
				return
			}
			key1 := transKey(key.Name)
			if key1 > 0 {
				gc.KeyUp(key1, "")
			} else {
				//key1 := transKey(key.Name)
				gc.KeyUp(key.Physical.ScanCode, "")
			}
		})
	}

}

var (
	ScreenImage *image.RGBA
)

// BitmapCH 通道定义
var BitmapCH = make(chan []client.Bitmap, 100)

func update() {
	go func() {
		for {
			select {
			case bs := <-BitmapCH:
				paint_bitmap(bs)
			default:
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()
}

// ToRGBA 保持原逻辑不变
func ToRGBA(pixel int, i int, data []byte) (r, g, b, a uint8) {
	a = 255
	switch pixel {
	case 1:
		rgb555 := core.Uint16BE(data[i], data[i+1])
		r, g, b = core.RGB555ToRGB(rgb555)
	case 2:
		rgb565 := core.Uint16BE(data[i], data[i+1])
		r, g, b = core.RGB565ToRGB(rgb565)
	case 3, 4:
		fallthrough
	default:
		r, g, b = data[i+2], data[i+1], data[i]
	}
	return
}

func paint_bitmap(bs []client.Bitmap) {
	var (
		pixel      int
		i          int
		r, g, b, a uint8
	)

	for _, bm := range bs {
		i = 0
		pixel = bm.BitsPerPixel
		// 创建临时图像用于绘制该块
		m := image.NewRGBA(image.Rect(0, 0, bm.Width, bm.Height))
		for y := 0; y < bm.Height; y++ {
			for x := 0; x < bm.Width; x++ {
				if i+pixel > len(bm.Data) {
					break
				}
				r, g, b, a = ToRGBA(pixel, i, bm.Data)
				c := color.RGBA{r, g, b, a}
				i += pixel
				m.Set(x, y, c)
			}
		}
		// 绘制到主屏幕图像上
		draw.Draw(ScreenImage, ScreenImage.Bounds().Add(image.Pt(bm.DestLeft, bm.DestTop)), m, m.Bounds().Min, draw.Src)
	}

	// 通知 UI 刷新
	if screenWidget != nil {
		// Fyne 的 Refresh 必须在 UI 线程或者它是线程安全的（Refresh通常是安全的，但为了保险起见）
		screenWidget.raster.Image = ScreenImage // 重新指向，确保更新
		screenWidget.Refresh()
	}
}

func ui_paint_bitmap(bs []client.Bitmap) {
	BitmapCH <- bs
}

// uiClient 保持签名不变
func uiClient(info *Info) (error, Control) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var (
		err error
		g   Control
	)
	if true {
		err, g = uiRdp(info)
	} else {
		err, g = uiVnc(info)
	}
	return err, g
}

// Control 接口保持不变
type Control interface {
	Login() error
	KeyUp(sc int, name string)
	KeyDown(sc int, name string)
	MouseMove(x, y int)
	MouseWheel(scroll, x, y int)
	MouseUp(button int, x, y int)
	MouseDown(button int, x, y int)
	Close()
}


var KeyMap = map[fyne.KeyName]int{

	fyne.KeyEscape:       0x0001,
	fyne.Key1:            0x0002,
	fyne.Key2:            0x0003,
	fyne.Key3:            0x0004,
	fyne.Key4:            0x0005,
	fyne.Key5:            0x0006,
	fyne.Key6:            0x0007,
	fyne.Key7:            0x0008,
	fyne.Key8:            0x0009,
	fyne.Key9:            0x000A,
	fyne.Key0:            0x000B,
	fyne.KeyMinus:        0x000C,
	fyne.KeyEqual:        0x000D,
	fyne.KeyBackspace:    0x000E,
	fyne.KeyTab:          0x000F,
	fyne.KeyQ:            0x0010,
	fyne.KeyW:            0x0011,
	fyne.KeyE:            0x0012,
	fyne.KeyR:            0x0013,
	fyne.KeyT:            0x0014,
	fyne.KeyY:            0x0015,
	fyne.KeyU:            0x0016,
	fyne.KeyI:            0x0017,
	fyne.KeyO:            0x0018,
	fyne.KeyP:            0x0019,
	fyne.KeyLeftBracket:  0x001A,
	fyne.KeyRightBracket: 0x001B,
	fyne.KeyEnter:        0x001C,
	//	fyne.KeyLeftControl:  0x001D,
	fyne.KeyA:         0x001E,
	fyne.KeyS:         0x001F,
	fyne.KeyD:         0x0020,
	fyne.KeyF:         0x0021,
	fyne.KeyG:         0x0022,
	fyne.KeyH:         0x0023,
	fyne.KeyJ:         0x0024,
	fyne.KeyK:         0x0025,
	fyne.KeyL:         0x0026,
	fyne.KeySemicolon: 0x0027,
	//"Quote":              0x0028,
	//"Backquote":          0x0029,
	//fyne.KeyLeftShift:  0x002A,
	fyne.KeyBackslash: 0x002B,
	fyne.KeyZ:         0x002C,
	fyne.KeyX:         0x002D,
	fyne.KeyC:         0x002E,
	fyne.KeyV:         0x002F,
	fyne.KeyB:         0x0030,
	fyne.KeyN:         0x0031,
	fyne.KeyM:         0x0032,
	fyne.KeyComma:     0x0033,
	fyne.KeyPeriod:    0x0034,
	fyne.KeySlash:     0x0035,
	//fyne.KeyRightShift: 0x0036,
	//fyne.KeyKpMultiply: 0x0037,
	//fyne.KeyLeftAlt:    0x0038,
	fyne.KeySpace: 0x0039,
	//fyne.KeyCapsLock:   0x003A,
	fyne.KeyF1:  0x003B,
	fyne.KeyF2:  0x003C,
	fyne.KeyF3:  0x003D,
	fyne.KeyF4:  0x003E,
	fyne.KeyF5:  0x003F,
	fyne.KeyF6:  0x0040,
	fyne.KeyF7:  0x0041,
	fyne.KeyF8:  0x0042,
	fyne.KeyF9:  0x0043,
	fyne.KeyF10: 0x0044,
	//fyne.KeyPause:        0x0045,
	//fyne.KeyScrollLock:   0x0046,
	//fyne.KeyKp7:          0x0047,
	//fyne.KeyKp8:          0x0048,
	//fyne.KeyKp9:          0x0049,
	//	fyne.KeyKpSubtract:   0x004A,
	//	fyne.KeyKp4:          0x004B,
	//	fyne.KeyKp5:          0x004C,
	//	fyne.KeyKp6:          0x004D,
	//	fyne.KeyKpAdd:        0x004E,
	//	fyne.KeyKp1:          0x004F,
	//	fyne.KeyKp2:          0x0050,
	//fyne.KeyKp3:          0x0051,
	//	fyne.KeyKp0:          0x0052,
	//fyne.KeyKpDecimal:    0x0053,
	fyne.KeyF11: 0x0057,
	fyne.KeyF12: 0x0058,
	//fyne.KeyKpEqual:      0x0059,
	//fyne.KeyKpEnter:      0xE01C,
	//fyne.KeyRightControl: 0xE01D,
	//	fyne.KeyKpDivide:     0xE035,
	//fyne.KeyPrintScreen:  0xE037,
	//fyne.KeyRightAlt:     0xE038,
	//fyne.KeyNumLock:      0xE045,
	//fyne.KeyPause:        0xE046,
	fyne.KeyHome:     0xE047,
	fyne.KeyUp:       0xE048,
	fyne.KeyPageUp:   0xE049,
	fyne.KeyLeft:     0xE04B,
	fyne.KeyRight:    0xE04D,
	fyne.KeyEnd:      0xE04F,
	fyne.KeyDown:     0xE050,
	fyne.KeyPageDown: 0xE051,
	fyne.KeyInsert:   0xE052,
	fyne.KeyDelete:   0xE053,
	//fyne.KeyMenu:         0xE05D,
}

func transKey(in fyne.KeyName) int {
	if v, ok := KeyMap[in]; ok {
		return v
	}
	return 0
}
