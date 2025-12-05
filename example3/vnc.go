package main

import (
	"errors"
	"fmt"

	"github.com/dosgo/grdp/client"
	"github.com/dosgo/grdp/glog"
	"github.com/dosgo/grdp/protocol/rfb"
)

func uiVnc(info *Info) (error, *client.Client) {
	BitmapCH = make(chan []client.Bitmap, 500)

	conf := &client.Setting{
		Width:    info.Width,
		Height:   info.Height,
		LogLevel: glog.INFO,
	}
	g := client.NewClient(fmt.Sprintf("%s:%s", info.Ip, info.Port), info.Username, info.Passwd, 1, conf)

	//g.info = info
	err := g.Login()
	if err != nil {
		glog.Error("Login:", err)
		return err, nil
	}

	g.OnError(func(e error) {
		glog.Info("on error")
		glog.Error(e)
	})
	g.OnClose(func() {
		err = errors.New("close")
		glog.Info("on close")
	})

	g.OnSuccess(func() {
		err = nil
		glog.Info("on success")
	})
	g.OnReady(func() {
		glog.Info("on ready")
	})
	g.VncBitmap(func(br *rfb.BitRect) {
		glog.Debug("on bitmap:", br)
		bs := make([]client.Bitmap, 0, 50)
		for _, v := range br.Rects {
			b := client.Bitmap{int(v.Rect.X), int(v.Rect.Y), int(v.Rect.X + v.Rect.Width), int(v.Rect.Y + v.Rect.Height),
				int(v.Rect.Width), int(v.Rect.Height),
				client.Bpp(uint16(br.Pf.BitsPerPixel)), false, v.Data}
			bs = append(bs, b)
		}
		ui_paint_bitmap(bs)
	})
	return nil, g
}
