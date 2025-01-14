package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/ms-teavaro/go-smpp"
	"github.com/ms-teavaro/go-smpp/pdu"
	"github.com/imdario/mergo"
	. "github.com/xeipuuv/gojsonschema"
)

var configure Configuration
var mutex sync.Mutex

//go:embed schema.json
var schemaFile []byte

func init() {
	var confPath string
	flag.StringVar(&confPath, "c", "configure.json", "configure file-path")

	if data, err := os.ReadFile(confPath); err != nil {
		log.Fatalln(err)
	} else if result, _ := Validate(NewBytesLoader(schemaFile), NewBytesLoader(data)); !result.Valid() {
		for _, desc := range result.Errors() {
			log.Println(desc)
		}
		log.Fatalln("invalid configuration")
	} else {
		_ = json.Unmarshal(data, &configure)
	}
	_ = mergo.Merge(configure.Devices[0], &Device{
		Version:          pdu.SMPPVersion34,
		BindMode:         "receiver",
		KeepAliveTick:    time.Millisecond * 500,
		KeepAliveTimeout: time.Second,
	})
	for i := 1; i < len(configure.Devices); i++ {
		_ = mergo.Merge(configure.Devices[i], configure.Devices[i-1])
	}
}

func main() {
	hook := runProgramWithEvent
	if configure.HookMode == "ndjson" {
		hook = runProgramWithStream()
	}
	for _, device := range configure.Devices {
		go connect(device, hook)
	}
	select {}
}

//goland:noinspection GoUnhandledErrorResult
func connect(device *Device, hook func(*Payload)) {
	ctx := context.Background()
	parent, err := net.Dial("tcp", device.SMSC)
	if err != nil {
		log.Fatalln(err)
	}
	session := smpp.NewSession(ctx, parent)
	session.ReadTimeout = time.Second
	session.WriteTimeout = time.Second
	defer session.Close(ctx)
	if resp, err := session.Submit(ctx, device.Binder()); err != nil {
		log.Fatalln(device, err)
	} else if status := pdu.ReadCommandStatus(resp); status != 0 {
		log.Fatalln(device, status)
	} else {
		log.Println(device, "Connected")
		go session.EnquireLink(ctx, device.KeepAliveTick, device.KeepAliveTimeout)
	}
	addDeliverSM := makeCombineMultipartDeliverSM(device, hook)
	for {
		select {
		case <-ctx.Done():
			log.Println(device, "Disconnected")
			time.Sleep(time.Second)
			go connect(device, hook)
			return
		case packet := <-session.PDU():
			switch p := packet.(type) {
			case *pdu.DeliverSM:
				addDeliverSM(p)
				_ = session.Send(p.Resp())
			case pdu.Responsable:
				_ = session.Send(p.Resp())
			}
		}
	}
}

//goland:noinspection GoUnhandledErrorResult
func runProgramWithEvent(message *Payload) {
	mutex.Lock()
	defer mutex.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*15)
	defer cancel()
	cmd := exec.CommandContext(ctx, configure.Hook)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if stdin, err := cmd.StdinPipe(); err != nil {
		log.Fatalln(err)
	} else {
		go func() {
			defer stdin.Close()
			_ = json.NewEncoder(stdin).Encode(message)
		}()
	}
	if err := cmd.Run(); err != nil {
		log.Fatalln(err)
	}
}

//goland:noinspection GoUnhandledErrorResult
func runProgramWithStream() func(*Payload) {
	cmd := exec.Command(configure.Hook)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalln(err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		if err = cmd.Run(); err != nil {
			log.Fatalln(err)
		}
	}()
	return func(message *Payload) {
		mutex.Lock()
		defer mutex.Unlock()
		_ = json.NewEncoder(stdin).Encode(message)
		_, _ = fmt.Fprintln(stdin)
	}
}

func makeCombineMultipartDeliverSM(device *Device, hook func(*Payload)) func(*pdu.DeliverSM) {
	return pdu.CombineMultipartDeliverSM(func(delivers []*pdu.DeliverSM) {
		var mergedMessage string
		for _, sm := range delivers {
			if sm.Message.DataCoding == 0x00 && device.Workaround == "SMG4000" {
				mergedMessage += string(sm.Message.Message)
			} else if message, err := sm.Message.Parse(); err == nil {
				mergedMessage += message
			}
		}
		source := delivers[0].SourceAddr
		target := delivers[0].DestAddr
		log.Println(device, source, "->", target)
		go hook(&Payload{
			SMSC:        device.SMSC,
			SystemID:    device.SystemID,
			SystemType:  device.SystemType,
			Owner:       device.Owner,
			Phone:       device.Phone,
			Extra:       device.Extra,
			Source:      source.String(),
			Target:      target.String(),
			Message:     mergedMessage,
			DeliverTime: time.Now(),
		})
	})
}
