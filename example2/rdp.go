package main

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/dosgo/grdp/client"
	"github.com/dosgo/grdp/core"
	"github.com/dosgo/grdp/glog"
	"github.com/dosgo/grdp/plugin/cliprdr"
	"github.com/dosgo/grdp/protocol/pdu"
)

func uiRdp(info *Info) (error, *client.Client) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	BitmapCH = make(chan []client.Bitmap, 500)
	conf := &client.Setting{
		Width:    info.Width,
		Height:   info.Height,
		LogLevel: glog.DEBUG,
	}
	g := client.NewClient(fmt.Sprintf("%s:%s", info.Ip, info.Port), info.Username, info.Passwd, 0, conf)
	g.SetLoginParam(fmt.Sprintf("%s:%s", info.Ip, info.Port), info.Username, info.Passwd)

	err := g.Login()
	if err != nil {
		glog.Error("Login:", err)
		return err, nil
	}
	cc := cliprdr.NewCliprdrClient()
	g.RdpChannelsRegister(cc)

	g.OnError(func(e error) {
		glog.Info("on error:", e)
	})

	g.OnClose(func() {
		err = errors.New("close")
		glog.Info("on close")
	})

	g.OnSuccess(func() {
		glog.Info("on success")
	})
	g.OnReady(func() {
		glog.Info("on ready")
	})
	g.RdpOnBitmap(func(rectangles []pdu.BitmapData) {
		glog.Info("Update Bitmap:", len(rectangles))
		bs := make([]client.Bitmap, 0, 50)
		for _, v := range rectangles {
			IsCompress := v.IsCompress()
			data := v.BitmapDataStream
			if IsCompress {
				data = core.Decompress(v.BitmapDataStream, int(v.Width), int(v.Height), client.Bpp(v.BitsPerPixel))
				IsCompress = false
			}

			b := client.Bitmap{int(v.DestLeft), int(v.DestTop), int(v.DestRight), int(v.DestBottom),
				int(v.Width), int(v.Height), client.Bpp(v.BitsPerPixel), IsCompress, data}
			bs = append(bs, b)
		}
		ui_paint_bitmap(bs)
	})

	return nil, g
}
