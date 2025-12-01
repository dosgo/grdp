// main.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"text/template"
	"time"

	"github.com/dosgo/grdp/glog"
	"github.com/dosgo/grdp/protocol/pdu"
	"github.com/dosgo/grdp/static"
	socketio "github.com/googollee/go-socket.io"
	"github.com/gorilla/websocket"
)

func showPreview(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("static/html/index.html")
	if err != nil {
		w.Write([]byte(err.Error() + "\n"))
		return
	}
	w.Header().Add("Content-Type", "text/html")
	t.Execute(w, nil)

}

func socketIO() {
	server := socketio.NewServer(nil)
	server.OnConnect("/", func(so socketio.Conn) error {
		fmt.Println("OnConnect", so.ID())
		//so.Emit("rdp-connect", true)
		//fmt.Println("OnConnect111", so.ID())
		return nil
	})
	server.OnEvent("/", "infos", func(so socketio.Conn, data interface{}) {
		fmt.Println("infos", so.ID())
		var info Info
		v, _ := json.Marshal(data)
		json.Unmarshal(v, &info)
		fmt.Println(so.ID(), "logon infos:", info)

		g := NewRdpClient(fmt.Sprintf("%s:%s", info.Ip, info.Port), info.Width, info.Height, glog.INFO)
		g.info = &info
		err := g.Login()
		if err != nil {
			fmt.Println("Login:", err)
			so.Emit("rdp-error", "{\"code\":1,\"message\":\""+err.Error()+"\"}")
			return
		}
		so.SetContext(g)
		g.pdu.On("error", func(e error) {
			fmt.Println("on error:", e)
			so.Emit("rdp-error", "{\"code\":1,\"message\":\""+e.Error()+"\"}")
			//wg.Done()
		}).On("close", func() {
			err = errors.New("close")
			fmt.Println("on close")
		}).On("success", func() {
			fmt.Println("on success")
		}).On("ready", func() {
			fmt.Println("on ready")
		}).On("bitmap", func(rectangles []pdu.BitmapData) {
			glog.Info(time.Now(), "on update Bitmap:", len(rectangles))
			bs := make([]Bitmap, 0, len(rectangles))
			for _, v := range rectangles {
				IsCompress := v.IsCompress()
				data := v.BitmapDataStream
				glog.Debug("data:", data)
				if IsCompress {
					//data = decompress(&v)
					//IsCompress = false
				}

				glog.Debug(IsCompress, v.BitsPerPixel)
				b := Bitmap{int(v.DestLeft), int(v.DestTop), int(v.DestRight), int(v.DestBottom),
					int(v.Width), int(v.Height), int(v.BitsPerPixel), IsCompress, data}
				so.Emit("rdp-bitmap", []Bitmap{b})
				bs = append(bs, b)
			}
			so.Emit("rdp-bitmap", bs)
		})
	})

	server.OnEvent("/", "mouse", func(so socketio.Conn, x, y uint16, button int, isPressed bool) {
		glog.Info("mouse", x, ":", y, ":", button, ":", isPressed)
		p := &pdu.PointerEvent{}
		if isPressed {
			p.PointerFlags |= pdu.PTRFLAGS_DOWN
		}

		switch button {
		case 1:
			p.PointerFlags |= pdu.PTRFLAGS_BUTTON1
		case 2:
			p.PointerFlags |= pdu.PTRFLAGS_BUTTON2
		case 3:
			p.PointerFlags |= pdu.PTRFLAGS_BUTTON3
		default:
			p.PointerFlags |= pdu.PTRFLAGS_MOVE
		}

		p.XPos = x
		p.YPos = y
		g := so.Context().(*RdpClient)
		g.pdu.SendInputEvents(pdu.INPUT_EVENT_MOUSE, []pdu.InputEventsInterface{p})
	})

	//keyboard
	server.OnEvent("/", "scancode", func(so socketio.Conn, button uint16, isPressed bool) {
		glog.Info("scancode:", "button:", button, "isPressed:", isPressed)

		p := &pdu.ScancodeKeyEvent{}
		p.KeyCode = button
		if !isPressed {
			p.KeyboardFlags |= pdu.KBDFLAGS_RELEASE
		}
		g := so.Context().(*RdpClient)
		g.pdu.SendInputEvents(pdu.INPUT_EVENT_SCANCODE, []pdu.InputEventsInterface{p})

	})

	//wheel
	server.OnEvent("/", "wheel", func(so socketio.Conn, x, y, step uint16, isNegative, isHorizontal bool) {
		glog.Info("wheel", x, ":", y, ":", step, ":", isNegative, ":", isHorizontal)
		var p = &pdu.PointerEvent{}
		if isHorizontal {
			p.PointerFlags |= pdu.PTRFLAGS_HWHEEL
		} else {
			p.PointerFlags |= pdu.PTRFLAGS_WHEEL
		}

		if isNegative {
			p.PointerFlags |= pdu.PTRFLAGS_WHEEL_NEGATIVE
		}

		p.PointerFlags |= (step & pdu.WheelRotationMask)
		p.XPos = x
		p.YPos = y
		g := so.Context().(*RdpClient)
		g.pdu.SendInputEvents(pdu.INPUT_EVENT_MOUSE, []pdu.InputEventsInterface{p})
	})

	server.OnError("/", func(so socketio.Conn, err error) {
		if so == nil || so.Context() == nil {
			return
		}
		fmt.Println("error:", err)
		g := so.Context().(*RdpClient)
		if g != nil {
			g.tpkt.Close()
		}
		so.Close()
	})

	server.OnDisconnect("/", func(so socketio.Conn, s string) {
		if so.Context() == nil {
			return
		}
		fmt.Println("OnDisconnect:", s)
		so.Emit("rdp-error", "{code:1,message:"+s+"}")

		g := so.Context().(*RdpClient)
		if g != nil {
			g.tpkt.Close()
		}
		so.Close()
	})
	go server.Serve()
	defer server.Close()

	http.Handle("/socket.io/", server)

	http.Handle("/", http.FileServer(http.FS(static.StaticFiles)))

	log.Println("Serving at localhost:8088...")
	log.Fatal(http.ListenAndServe(":8088", nil))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func wsIO() {

	http.HandleFunc("/ws", handleConnections)

	http.Handle("/", http.FileServer(http.FS(static.StaticFiles)))

	log.Println("Serving at localhost:8088...")
	log.Fatal(http.ListenAndServe(":8088", nil))
}

func handleConnections(w http.ResponseWriter, r *http.Request) {

	type Message struct {
		Cmd  string `json:"cmd"`
		Data string `json:"data"`
	}

	type MouseInfo struct {
		X         uint16 `json:"x"`
		Y         uint16 `json:"y"`
		Button    uint16 `json:"button"`
		IsPressed bool   `json:"isPressed"`
	}

	type KeyboardInfo struct {
		Button    uint16 `json:"button"`
		IsPressed bool   `json:"isPressed"`
	}

	type WheelInfo struct {
		X            uint16 `json:"x"`
		Y            uint16 `json:"y"`
		Step         uint16 `json:"step"`
		IsNegative   bool   `json:"isNegative"`
		IsHorizontal bool   `json:"isHorizontal"`
	}

	// 升级HTTP连接到WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	defer conn.Close()

	// 发送欢迎消息
	connectMsg := Message{
		Cmd: "rdp-connect",
	}
	conn.WriteJSON(connectMsg)

	// 读取客户端消息
	var recvMsg Message
	var g *RdpClient
	for {

		err := conn.ReadJSON(&recvMsg)
		if err != nil {

			break
		}

		// 获取连接
		if recvMsg.Cmd == "infos" {

			fmt.Println("infos", r.RemoteAddr)
			var info Info
			json.Unmarshal([]byte(recvMsg.Data), &info)
			fmt.Println(r.RemoteAddr, "logon infos:", info)

			g = NewRdpClient(fmt.Sprintf("%s:%s", info.Ip, info.Port), info.Width, info.Height, glog.INFO)
			g.info = &info
			err := g.Login()
			if err != nil {
				fmt.Println("Login:", err)
				conn.WriteJSON(Message{
					Cmd:  "rdp-error",
					Data: "{\"code\":1,\"message\":\"" + err.Error() + "\"}",
				})
				return
			}
			g.pdu.On("error", func(e error) {
				fmt.Println("on error:", e)
				//so.Emit("rdp-error", "{\"code\":1,\"message\":\""+e.Error()+"\"}")
				conn.WriteJSON(Message{
					Cmd:  "rdp-error",
					Data: "{\"code\":1,\"message\":\"" + e.Error() + "\"}",
				})
				//wg.Done()
			}).On("close", func() {
				err = errors.New("close")
				fmt.Println("on close")
			}).On("success", func() {
				fmt.Println("on success")
			}).On("ready", func() {
				fmt.Println("on ready")
			}).On("color", func(fastPathColorPdu *pdu.FastPathColorPdu) {

				fmt.Printf("color v:%+v\r\n", fastPathColorPdu)

			}).On("orders", func(orderPdus []pdu.OrderPdu) {
				isSend := false
				for _, v := range orderPdus {
					if v.Type == 0 {
						isSend = true
					}
					if v.Primary != nil {

						//	fmt.Printf("orders  v Primary type:%d data:%+v\r\n", v.Primary.Data.Type(), v.Primary.Data)

					}

				}
				if isSend {
					data, err := json.Marshal(orderPdus)
					if err == nil {
						conn.WriteJSON(Message{
							Cmd:  "rdp-orders",
							Data: string(data),
						})
					}
				}

			}).On("bitmap", func(rectangles []pdu.BitmapData) {
				//glog.Info(time.Now(), "on update Bitmap:", len(rectangles))
				bs := make([]Bitmap, 0, len(rectangles))
				for _, v := range rectangles {
					IsCompress := v.IsCompress()
					data := v.BitmapDataStream

					glog.Debug(IsCompress, v.BitsPerPixel)
					b := Bitmap{int(v.DestLeft), int(v.DestTop), int(v.DestRight), int(v.DestBottom),
						int(v.Width), int(v.Height), int(v.BitsPerPixel), IsCompress, data}
					//so.Emit("rdp-bitmap", []Bitmap{b})
					/*
						data, err := json.Marshal([]Bitmap{b})
						if err == nil {
							conn.WriteJSON(Message{
								Cmd:  "rdp-bitmap",
								Data: string(data),
							})
						}*/
					bs = append(bs, b)
				}
				//so.Emit("rdp-bitmap", bs)
				data, err := json.Marshal(bs)
				if err == nil {
					conn.WriteJSON(Message{
						Cmd:  "rdp-bitmap",
						Data: string(data),
					})
				}
			})
		}

		// 获取连接
		if recvMsg.Cmd == "mouse" {

			var info MouseInfo

			json.Unmarshal([]byte(recvMsg.Data), &info)

			glog.Info("mouse", info.X, ":", info.Y, ":", info.Button, ":", info.IsPressed)
			p := &pdu.PointerEvent{}
			if info.IsPressed {
				p.PointerFlags |= pdu.PTRFLAGS_DOWN
			}

			switch info.Button {
			case 1:
				p.PointerFlags |= pdu.PTRFLAGS_BUTTON1
			case 2:
				p.PointerFlags |= pdu.PTRFLAGS_BUTTON2
			case 3:
				p.PointerFlags |= pdu.PTRFLAGS_BUTTON3
			default:
				p.PointerFlags |= pdu.PTRFLAGS_MOVE
			}

			p.XPos = info.X
			p.YPos = info.Y
			if g != nil {
				g.pdu.SendInputEvents(pdu.INPUT_EVENT_MOUSE, []pdu.InputEventsInterface{p})
			}
		}
		//keyboard
		if recvMsg.Cmd == "scancode" {
			var info KeyboardInfo
			json.Unmarshal([]byte(recvMsg.Data), &info)
			if info.Button == 0 {
				continue
			}
			glog.Info("scancode:", "button:", info.Button, "isPressed:", info.IsPressed)

			p := &pdu.ScancodeKeyEvent{}
			p.KeyCode = info.Button
			if !info.IsPressed {
				p.KeyboardFlags |= pdu.KBDFLAGS_RELEASE
			}
			if g != nil {
				g.pdu.SendInputEvents(pdu.INPUT_EVENT_SCANCODE, []pdu.InputEventsInterface{p})
			}
		}
		//wheel
		if recvMsg.Cmd == "wheel" {
			var info WheelInfo
			json.Unmarshal([]byte(recvMsg.Data), &info)
			glog.Info("wheel", info.X, ":", info.Y, ":", info.Step, ":", info.IsNegative, ":", info.IsHorizontal)
			var p = &pdu.PointerEvent{}
			if info.IsHorizontal {
				p.PointerFlags |= pdu.PTRFLAGS_HWHEEL
			} else {
				p.PointerFlags |= pdu.PTRFLAGS_WHEEL
			}

			if info.IsNegative {
				p.PointerFlags |= pdu.PTRFLAGS_WHEEL_NEGATIVE
			}

			p.PointerFlags |= info.Step & pdu.WheelRotationMask
			p.XPos = info.X
			p.YPos = info.Y
			if g != nil {
				g.pdu.SendInputEvents(pdu.INPUT_EVENT_MOUSE, []pdu.InputEventsInterface{p})
			}

		}

	}
	if g != nil {

		g.tpkt.Close()

	}
}
