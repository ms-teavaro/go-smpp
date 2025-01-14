package main

import (
	"github.com/ms-teavaro/go-smpp"
	"github.com/abiosoft/ishell"
)

var session *smpp.Session

var shell = ishell.New()

func init() {
	shell.AutoHelp(true)
	shell.SetHistoryPath(".smpp_repl_history")
	shell.AddCmd(&ishell.Cmd{Name: "connect", Help: "connect to server", Func: onConnectToServer})
}

func main() {
	shell.Println("Short Message Peer-to-Peer interactive shell")
	shell.Run()
}
