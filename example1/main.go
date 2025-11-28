// main.go
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/dosgo/grdp/glog"
	"github.com/dosgo/grdp/protocol/pdu"
	"github.com/gorilla/websocket"
)

var (
	server bool
)

func init() {
	glog.SetLevel(glog.INFO)
	logger := log.New(os.Stdout, "", 0)
	glog.SetLogger(logger)
}

func main() {

	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())

	wsIO()
}

type Screen struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type Info struct {
	Domain   string `json:"domain"`
	Ip       string `json:"ip"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Passwd   string `json:"password"`
	Screen   `json:"screen"`
}

func NewInfo(ip, user, passwd string) (error, *Info) {
	var i Info
	if ip == "" || user == "" || passwd == "" {
		return fmt.Errorf("Must ip/user/passwd"), nil
	}
	t := strings.Split(ip, ":")
	i.Ip = t[0]
	i.Port = "3389"
	if len(t) > 1 {
		i.Port = t[1]
	}
	if strings.Index(user, "\\") != -1 {
		t = strings.Split(user, "\\")
		i.Domain = t[0]
		i.Username = t[len(t)-1]
	} else if strings.Index(user, "/") != -1 {
		t = strings.Split(user, "/")
		i.Domain = t[0]
		i.Username = t[len(t)-1]
	} else {
		i.Username = user
	}

	i.Passwd = passwd

	return nil, &i
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func showPreview(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("static/html/index.html")
	if err != nil {
		w.Write([]byte(err.Error() + "\n"))
		return
	}
	w.Header().Add("Content-Type", "text/html")
	t.Execute(w, nil)

}

func wsIO() {

	http.HandleFunc("/ws", handleConnections)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/css/", http.FileServer(http.Dir("static")))
	http.Handle("/js/", http.FileServer(http.Dir("static")))
	http.Handle("/img/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/", showPreview)

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
