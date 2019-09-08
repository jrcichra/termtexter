package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"syscall"
	proto "termtexter/proto"

	"golang.org/x/crypto/ssh/terminal"
)

//Client - client struct
type Client struct {
	conn  net.Conn
	proto proto.Proto
}

func (c Client) check(e error) {
	if e != nil {
		panic(e)
	}
}

//Init - get the client socket ready
func (c Client) Init(host string, port int) {
	var err error
	c.conn, err = net.Dial("tcp", host+":"+strconv.Itoa(port))
	c.check(err)
	c.proto = proto.Proto{}
	c.proto.Init(c.conn)

}

func (c Client) getLogin() proto.Login {
	var ret proto.Login
	switch msg := c.proto.Decode().(type) {
	case proto.Login:
		ret = msg
	default:
		r := reflect.TypeOf(msg)
		fmt.Printf("Other:%v\n", r)
		os.Exit(1)
	}
	return ret
}

//SendLogin - forwards to the Proto object
func (c Client) SendLogin(username string, password string) {
	c.proto.SendLogin(username, password)
}

//GetCredentials - Gets the login credentials from the user
func (c Client) GetCredentials() (string, string) {
	reader := bufio.NewReader(os.Stdin)
	username := ""
	for username == "" {
		fmt.Println("Enter your username:")
		var err error
		username, err = reader.ReadString('\n')
		c.check(err)
	}
	password := ""
	for password == "" {
		fmt.Println("Enter your password:")
		bytepwd, err := terminal.ReadPassword(int(syscall.Stdin))
		c.check(err)
		password = string(bytepwd)
	}
	return username, password
}

func main() {
	var c Client
	c.Init("localhost", 1200)
	username, password := c.GetCredentials()
	c.SendLogin(username, password)
	c.getLogin()
}
