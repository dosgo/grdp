// ui.go
package main

import (
	"image"
	"image/color"
	"image/draw"
	"runtime"
	"time"

	"cogentcore.org/core/colors"
	"cogentcore.org/core/core"
	"cogentcore.org/core/events"
	"cogentcore.org/core/icons"
	"cogentcore.org/core/styles"
	"cogentcore.org/core/styles/units"
	"cogentcore.org/core/tree"

	"github.com/dosgo/grdp/client"
)

var (
	gc      Control
	width   = 1024
	height  = 768
	mainWin *core.Body
)

func StartUI(w, h int) {
	width, height = w, h
	core.TheApp.AppName = "MSTSC"
	core.TheApp.AppAbout = "Remote Desktop Client"
	b := core.NewBody("MSTSC")
	core.TheApp.SetQuitCleanFunc(func() {
		if gc != nil {
			gc.Close()
		}
	})

	core.TheApp.RunMainWindow(func() {
		mainWin = core.NewBody("MSTSC")
		appMain(mainWin)
	})
}

func appMain(b *core.Body) {
	// 创建主布局
	mainLayout := core.NewFrame(b)
	mainLayout.Styler(func(s *styles.Style) {
		s.Grow.Set(1, 1)
	})

	// 创建连接表单
	createConnectionForm(mainLayout)

	// 创建远程桌面显示区域
	createRemoteDisplay(mainLayout)

	b.AddLayout(mainLayout)
}

func createConnectionForm(parent tree.Node) {
	form := core.NewFrame(parent)
	form.Styler(func(s *styles.Style) {
		s.Grow.Set(1, 1)
		s.Padding.Set(units.Dp(20))
	})

	// 标题
	title := core.NewText(form)
	title.SetText("Welcome to MSTSC")
	title.Styler(func(s *styles.Style) {
		s.Text.Align = styles.Center
		s.Text.Color = colors.Scheme.Error.Base
		s.Font.Size.Dp(24)
		s.Font.Weight = styles.WeightBold
	})

	// 表单字段
	formLayout := core.NewLayout(form)
	formLayout.Styler(func(s *styles.Style) {
		s.Gap.Set(units.Dp(10))
	})

	ipField := core.NewTextField(formLayout)
	ipField.SetLabel("Server Address:")
	ipField.SetPlaceholder("192.168.0.132:3389")
	ipField.SetText("192.168.0.132:3389")

	userField := core.NewTextField(formLayout)
	userField.SetLabel("Username:")
	userField.SetPlaceholder("administrator")
	userField.SetText("administrator")

	passField := core.NewTextField(formLayout)
	passField.SetLabel("Password:")
	passField.SetPlaceholder("Password")
	passField.SetType(core.FieldPassword)

	// 按钮布局
	buttonLayout := core.NewLayout(formLayout)
	buttonLayout.Styler(func(s *styles.Style) {
		s.Gap.Set(units.Dp(10))
		s.Justify.Content = styles.Center
	})

	okBtn := core.NewButton(buttonLayout)
	okBtn.SetText("Connect")
	okBtn.SetIcon(icons.Link)
	okBtn.OnClick(func(e events.Event) {
		connectToServer(ipField.Text(), userField.Text(), passField.Text())
	})

	clearBtn := core.NewButton(buttonLayout)
	clearBtn.SetText("Clear")
	clearBtn.SetIcon(styles.Icon(0xF00D)) // clear icon
	clearBtn.OnClick(func(e events.Event) {
		ipField.SetText("")
		userField.SetText("")
		passField.SetText("")
	})
}

func createRemoteDisplay(parent tree.Node) *core.Image {
	// 创建图像显示区域
	imgFrame := core.NewFrame(parent)
	imgFrame.Styler(func(s *styles.Style) {
		s.Grow.Set(1, 1)
		s.Display = styles.Stacked
	})
	imgFrame.SetVisible(false)

	img := core.NewImage(imgFrame)
	img.Styler(func(s *styles.Style) {
		s.Grow.Set(1, 1)
		s.Min.Set(units.Dp(width), units.Dp(height))
	})

	// 设置事件处理
	img.On(events.MouseDown, func(e events.Event) {
		if gc == nil {
			return
		}
		pos := e.Pos()
		gc.MouseDown(int(e.MouseButton()), int(pos.X), int(pos.Y))
	})

	img.On(events.MouseUp, func(e events.Event) {
		if gc == nil {
			return
		}
		pos := e.Pos()
		gc.MouseUp(int(e.MouseButton()), int(pos.X), int(pos.Y))
	})

	img.On(events.MouseMove, func(e events.Event) {
		if gc == nil {
			return
		}
		pos := e.Pos()
		gc.MouseMove(int(pos.X), int(pos.Y))
	})

	img.On(events.Scroll, func(e events.Event) {
		if gc == nil {
			return
		}
		pos := e.Pos()
		se := e.(*events.MouseScroll)
		gc.MouseWheel(int(se.Delta.Y), int(pos.X), int(pos.Y))
	})

	img.OnKeyChord(func(e events.Event) {
		if gc == nil {
			return
		}
		ke := e.(*events.Key)
		if ke.Type == events.KeyDown {
			key := transKey(ke.Key)
			gc.KeyDown(key, ke.String())
		} else if ke.Type == events.KeyUp {
			key := transKey(ke.Key)
			gc.KeyUp(key, ke.String())
		}
	})

	return img
}

var (
	ScreenImage *image.RGBA
	remoteImg   *core.Image
)

func update() {
	go func() {
		for {
			select {
			case bs := <-BitmapCH:
				paint_bitmap(bs)
			default:
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
}

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
		m := image.NewRGBA(image.Rect(0, 0, bm.Width, bm.Height))
		for y := 0; y < bm.Height; y++ {
			for x := 0; x < bm.Width; x++ {
				r, g, b, a = ToRGBA(pixel, i, bm.Data)
				c := color.RGBA{r, g, b, a}
				i += pixel
				m.Set(x, y, c)
			}
		}

		draw.Draw(ScreenImage, ScreenImage.Bounds().Add(image.Pt(bm.DestLeft, bm.DestTop)), m, m.Bounds().Min, draw.Src)
	}

	// 更新UI线程中的图像
	core.TheApp.RunOnMain(func() {
		if remoteImg != nil {
			remoteImg.SetImage(ScreenImage)
			remoteImg.NeedsRender()
		}
	})
}

var BitmapCH chan []client.Bitmap

func ui_paint_bitmap(bs []client.Bitmap) {
	BitmapCH <- bs
}

func connectToServer(ip, user, pass string) {
	info, err := NewInfo(ip, user, pass)
	if err != nil {
		core.ErrorDialog(mainWin, "Connection Error", err.Error())
		return
	}
	info.Width, info.Height = width, height

	err, control := uiClient(info)
	if err != nil {
		core.ErrorDialog(mainWin, "Connection Error", err.Error())
		return
	}

	gc = control
	// 隐藏连接表单，显示远程桌面
	mainWin.Widget.FirstChild().(*core.Frame).SetVisible(false)
	mainWin.Widget.Child(1).(*core.Frame).SetVisible(true)
}

func uiClient(info *Info) (error, Control) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var (
		err error
		g   Control
	)

	// 初始化位图通道
	BitmapCH = make(chan []client.Bitmap, 100)

	if true {
		err, g = uiRdp(info)
	} else {
		err, g = uiVnc(info)
	}

	if err == nil {
		update()
	}

	return err, g
}

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

// 键盘映射表
var KeyMap = map[events.KeyRunes]int{
	events.KeyEscape: 0x0001,
	events.Key1:      0x0002,
	events.Key2:      0x0003,
	// ... 其他键映射保持不变
	// 注意：需要根据Cogent Core的events.Key类型调整
}

func transKey(key events.KeyRunes) int {
	if v, ok := KeyMap[key]; ok {
		return v
	}
	return 0
}

// 初始化函数
func init() {
	ScreenImage = image.NewRGBA(image.Rect(0, 0, width, height))
	BitmapCH = make(chan []client.Bitmap, 100)
}
