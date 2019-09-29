package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	proto "termtexter/proto"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

const (
	HTTP_OK          = 200
	HTTP_FORBIDDEN   = 403
	HTTP_BADREQUEST  = 400
	HTTP_ERROR       = 500
	HTTP_UNAVAILABLE = 503
)

type channels struct {
	getMessagesResponse chan proto.GetMessagesResponse
	getRoomsResponse    chan proto.GetRoomsResponse
	joinRoomResponse    chan proto.JoinRoomResponse
	registerResponse    chan proto.RegisterResponse
	createRoomResponse  chan proto.CreateRoomResponse
	postMessageResponse chan proto.PostMessageResponse
	loginResponse       chan proto.LoginResponse
	dynamicMessage      chan proto.DynamicMessage
}

//Client - client struct
type Client struct {
	conn     net.Conn
	proto    proto.Proto
	rooms    map[int]*proto.Room
	curRoom  int
	curChan  int
	loggedIn bool
	channels channels
	app      *tview.Application
	pages    *tview.Pages
	chat     *tview.TextView
	users    *tview.List
	roomtree *tview.TreeView
	mainmenu *tview.Primitive
}

func (c Client) check(e error) {
	if e != nil {
		panic(e)
	}
}

//Init - get the client socket ready
func (c *Client) Init(host string, port int) {
	var err error
	a, err := net.Dial("tcp", host+":"+strconv.Itoa(port))
	c.conn = a
	c.check(err)
	c.proto = proto.Proto{Conn: c.conn}

	//allocate memory for the channels
	c.channels.createRoomResponse = make(chan proto.CreateRoomResponse)
	c.channels.getMessagesResponse = make(chan proto.GetMessagesResponse)
	c.channels.getRoomsResponse = make(chan proto.GetRoomsResponse)
	c.channels.joinRoomResponse = make(chan proto.JoinRoomResponse)
	c.channels.loginResponse = make(chan proto.LoginResponse)
	c.channels.postMessageResponse = make(chan proto.PostMessageResponse)
	c.channels.registerResponse = make(chan proto.RegisterResponse)
	c.channels.dynamicMessage = make(chan proto.DynamicMessage)
	//listens for incoming packets and sends to the proper channels
	go c.packetListener()
	// make the app and pages
	c.app = tview.NewApplication()
	c.pages = tview.NewPages()
}

func (c *Client) messageHandler(chat *tview.TextView) {
	for {
		//block waiting for new dynamic messages
		msg := <-c.channels.dynamicMessage

		m := proto.Message{}
		m.Created = msg.Created
		m.ID = msg.ID
		m.Message = msg.Message
		m.Received = msg.Received
		m.Timestamp = msg.Timestamp
		m.Type = msg.Type
		m.UserID = msg.UserID

		c.rooms[msg.Room].Channels[msg.Channel].Messages = append(c.rooms[msg.Room].Channels[msg.Channel].Messages, &m)
		//TODO: do this if we're viewing the current chat, otherwise bold the channel this message would be in
		if msg.Room == c.curRoom && msg.Channel == c.curChan {
			chat.SetText(chat.GetText(true) + c.buildMessage(msg.Created.String(), c.rooms[c.curRoom].Users[msg.UserID].DisplayName, msg.Message))
		} else {
			chat.SetText("You're in the wrong castle")
		}
	}
}

//GetRegistrationResponse - see what the response is
func (c Client) GetRegistrationResponse() proto.RegisterResponse {
	var ret proto.RegisterResponse
	msg := <-c.channels.registerResponse
	ret = msg
	return ret
}

//SendRegistration - calls the proto register
func (c Client) SendRegistration(username string, password string) {
	c.proto.SendRegistration(username, password)
}

//CreateRoom - creates a room
func (c *Client) CreateRoom(name string, password string) int {
	err := c.proto.SendCreateRoom(name, password)
	c.check(err)
	var res proto.CreateRoomResponse
	msg := <-c.channels.createRoomResponse

	res = msg
	if res.Code == HTTP_OK {
		//We joined the room
		log.Println("Successfully created the", name, "room.")
	} else if res.Code == HTTP_BADREQUEST {
		//The room already exists
		log.Println("This room already exists", name)
	} else {
		log.Println("Unknown return code from the server:", res.Code)
	}

	return res.Code
}

//JoinRoom - joins a room. This is a one time per account operation, just to link your account to a room (if you know the name and password). Returns success
func (c *Client) JoinRoom(name string, password string) bool {
	err := c.proto.SendJoinRoom(name, password)
	c.check(err)
	var res proto.JoinRoomResponse
	ret := false
	msg := <-c.channels.joinRoomResponse

	res = msg
	if res.Code == HTTP_OK {
		//We joined the room
		log.Println("Successfully joined the", name, "room.")
		ret = true
	} else if res.Code == HTTP_BADREQUEST {
		//The room doesn't exist
		log.Println("This room doesn't exist (yet):", name)
	} else if res.Code == HTTP_FORBIDDEN {
		//bad
		log.Println("The password you entered for this room is incorrect, or you were banned from this room")
	} else {
		log.Println("Unknown return code from the server:", res.Code)
	}

	return ret
}

//Login - Logs user in and returns http code
func (c *Client) Login(username string, password string) bool {
	err := c.proto.SendLogin(username, password)
	c.check(err)
	var ret proto.LoginResponse
	msg := <-c.channels.loginResponse
	ret = msg
	var resp bool
	if ret.Code == 200 {
		//Set our proto's session key
		c.proto.SetKey(ret.Key)
		c.loggedIn = true
		resp = true
	} else {
		resp = false
	}
	return resp
}

//UpdateRooms - updates the object's rooms struct value by using the return value of GetRooms
func (c *Client) UpdateRooms() {
	c.rooms = c.GetRooms()
}

//GetRooms - replaces the list of rooms in the object with what the database says
func (c Client) GetRooms() map[int]*proto.Room {
	err := c.proto.SendGetRoomsRequest()
	c.check(err)
	var ret proto.GetRoomsResponse
	msg := <-c.channels.getRoomsResponse
	ret = msg
	if ret.Code == 200 {
		//We got a good response...
		//c.rooms = msg.Rooms
	} else {
		log.Println("Not updating the rooms because we got a bad return code...")
	}

	return ret.Rooms
}

//PrintRooms - Nicely prints all the rooms provided
func (c Client) PrintRooms() {
	if !c.loggedIn {
		fmt.Println("You are not logged in")
	}
	for _, v := range c.rooms {
		fmt.Println("\tID:", v.ID) //could also use k
		fmt.Println("\tName:", v.Name)
		fmt.Println("\tDisplay Name:", v.DisplayName)
		fmt.Println("\tChannels:")
		for _, v2 := range v.Channels {
			fmt.Println("\t\tID:", v2.ID)
			fmt.Println("\t\tName:", v2.Name)
		}
	}
}

func (c *Client) sendMessage(msg string, room int, channel int) error {
	if room == -1 || channel == -1 {
		fmt.Println("Please set your channel and room before sending a message.")
		return nil
	}
	err := c.proto.SendPostMessageRequest(msg, room, channel)
	c.check(err)
	var ret proto.PostMessageResponse
	rmsg := <-c.channels.postMessageResponse

	ret = rmsg
	if ret.Code == 200 {
		//We got a good response...
	} else {
		log.Println("We got a bad return code...")
	}

	return err
}

//UpdateMessages - Queries the database and gets the last N messages from the DB for the channel we are currently on
func (c *Client) UpdateMessages() {
	msgs, empty := c.GetMessages(c.curRoom, c.curChan)
	if !empty {
		c.rooms[c.curRoom].Channels[c.curChan].Messages = msgs
	} else {
		//no messages, don't do anything :-)
	}
}

//GetMessages - Queries the database and gets the last N messages from the DB for the channel we are currently on
func (c *Client) GetMessages(room int, channel int) ([]*proto.Message, bool) {
	e := true
	if room == -1 || channel == -1 {
		fmt.Println("Please set your channel and room before requesting messages.")
		empty := make([]*proto.Message, 0)
		return empty, e
	}
	err := c.proto.SendGetMessagesRequest(room, channel)
	c.check(err)
	ret := <-c.channels.getMessagesResponse

	if ret.Code == 200 {
		//We got a good response...
	} else {
		log.Println("Not updating the rooms because we got a bad return code...")
	}

	//see how big our array is
	if len(ret.Messages) == 0 {
		e = true
	}

	return ret.Messages, e
}

func (c *Client) populateRoomTree() {
	root := c.roomtree.GetRoot()
	for _, v := range c.rooms {
		node := tview.NewTreeNode(v.DisplayName).SetColor(tcell.ColorGreen)
		for _, v2 := range v.Channels {
			node2 := tview.NewTreeNode(v2.Name).SetColor(tcell.ColorGray)
			node.AddChild(node2)
		}
		root.AddChild(node)
	}
}

func (c *Client) registerPage() *tview.Grid {
	form := tview.NewForm()
	form = form.AddDropDown("Type", []string{"Login", "Register"}, 1, func(option string, index int) {
		//runs when a selection is made
		if option == "Register" {
			//They want to register. Since we're already here, do nothing
		} else if option == "Login" {
			//They want to see all things related to logging in. The only way to get here is thru the login page, so lets send them back
			c.pages.SwitchToPage("login")
			form.GetFormItemByLabel("Type").(*tview.DropDown).SetCurrentOption(1)
		} else {
			//no idea what they want
		}
	}).AddInputField("Username", "", 20, nil, nil).
		AddPasswordField("Password", "", 10, '*', nil).
		AddPasswordField("Verify", "", 10, '*', nil)
	form = form.AddButton("Register", func() {
		//get the input elements
		ufield := form.GetFormItemByLabel("Username").(*tview.InputField)
		pfield := form.GetFormItemByLabel("Password").(*tview.InputField)
		verify := form.GetFormItemByLabel("Verify").(*tview.InputField)
		//make sure the passwords match
		if pfield.GetText() != verify.GetText() {
			//mismatch
			pfield.SetText("")
			verify.SetText("")
			c.app.SetFocus(ufield)
		} else {
			//passwords match
			//send a registration request
			c.SendRegistration(ufield.GetText(), pfield.GetText())
			//wait for a registration response
			resp := <-c.channels.registerResponse
			if resp.Code == 200 {
				//it worked! send them back to the login
				c.pages.SwitchToPage("login")
			} else {
				//something went wrong
				pfield.SetText("")
				verify.SetText("")
				c.app.SetFocus(ufield)
			}
		}
	})
	//make the ufield be the default focus
	form = form.SetFocus(1)
	grid := tview.NewGrid().SetColumns(0, 20, 0).SetRows(0, 0, 0).AddItem(form, 1, 1, 1, 1, 0, 0, true)
	grid.SetBorder(true).SetTitle("termtexter").SetTitleAlign(tview.AlignCenter).SetTitleColor(tcell.ColorLimeGreen)
	return grid
}

func (c *Client) loginPage() *tview.Grid {
	form := tview.NewForm()
	form = form.AddDropDown("Type", []string{"Login", "Register"}, 0, func(option string, index int) {
		//runs when a selection is made
		if option == "Register" {
			//They want to register. Let's make that a different "page"
			c.pages.SwitchToPage("register")
			form.GetFormItemByLabel("Type").(*tview.DropDown).SetCurrentOption(0)
		} else if option == "Login" {
			//They want to see all things related to logging in. Since we're already here, do nothing
		} else {
			//no idea what they want
		}
	}).
		AddInputField("Username", "", 20, nil, nil).
		AddPasswordField("Password", "", 10, '*', nil)
	form = form.AddButton("Login", func() {
		//parse out the input elements
		ufield := form.GetFormItemByLabel("Username").(*tview.InputField)
		pfield := form.GetFormItemByLabel("Password").(*tview.InputField)
		username := ufield.GetText()
		password := pfield.GetText()
		//check the login
		if c.Login(username, password) {
			c.UpdateRooms()
			c.pages.SwitchToPage("main")
			//c.getMessages()
			//c.getUsers()
			//c.populateRoomTree()
			c.app.SetFocus(c.chat)
		} else {
			//bad credentials, let the user know and blank out their password
			// ufield.SetText("")
			pfield.SetText("")
			//switch focus up to the ufield
			c.app.SetFocus(ufield)
		}
	})
	form = form.AddButton("Quit", func() {
		c.app.Stop()
	}).AddCheckbox("Remember", false, nil)
	//default focus
	form = form.SetFocus(4)

	//TESTING: setting to user
	form.GetFormItemByLabel("Username").(*tview.InputField).SetText("bill")
	form.GetFormItemByLabel("Password").(*tview.InputField).SetText("asdf")

	grid := tview.NewGrid().SetColumns(0, 20, 0).SetRows(0, 0, 0).AddItem(form, 1, 1, 1, 1, 0, 0, true)
	grid.SetBorder(true).SetTitle("termtexter").SetTitleAlign(tview.AlignCenter).SetTitleColor(tcell.ColorLimeGreen)
	return grid
}

func (c *Client) buildMessage(date string, dispname string, msg string) string {
	return date + " - " + dispname + " <" + msg + ">\n"
}

func (c *Client) getMessages() {
	//data for the chat window
	c.UpdateMessages()
	messages := ""
	for _, v := range c.rooms[c.curRoom].Channels[c.curChan].Messages {
		messages += c.buildMessage(v.Created.String(), c.rooms[c.curRoom].Users[v.UserID].DisplayName, v.Message)
	}
	c.chat.SetText(strings.Repeat("\n", 1000) + messages)
}

func (c *Client) getUsers() {
	for _, v := range c.rooms[c.curRoom].Users {
		c.users.AddItem(v.DisplayName, "", '+', nil)
	}
}

func (c *Client) mainMenu() {
	modal := func(p tview.Primitive, width, height int) tview.Primitive {
		return tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(tview.NewList().AddItem("Tanner", "isreal", tcell.RuneDiamond, nil), 0, 1, false).
				AddItem(p, height, 1, false).
				AddItem(nil, 0, 1, false), width, 1, false).
			AddItem(nil, 0, 1, false)
	}
	box := tview.NewBox().
		SetBorder(true).
		SetTitle("Main Menu").
		SetBorderColor(tcell.ColorRed)

	m := modal(box, 40, 10)
	c.pages.AddPage("mainmenu", m, true, false)
	c.mainmenu = &m
}

func (c *Client) checkIfMainMenu(event *tcell.EventKey) {
	if event.Key() == tcell.KeyEsc {
		//bring up the modal menu
		c.app.SetFocus(*c.mainmenu)
		c.pages.ShowPage("mainmenu")
	}
}

func (c *Client) mainPage() *tview.Flex {
	//data for the rooms
	root := tview.NewTreeNode("Rooms").SetColor(tcell.ColorRed)
	c.roomtree = tview.NewTreeView().SetRoot(root).SetCurrentNode(root)
	c.roomtree.SetBorder(true).SetTitle("Rooms")

	c.chat = tview.NewTextView().SetScrollable(true).ScrollToEnd()
	c.chat.SetBorder(true).SetTitle("Chat")
	//handles new messages that are dynamically sent in
	go c.messageHandler(c.chat)

	//chatbox
	chatbox := tview.NewInputField()
	chatbox.SetBorder(true).SetTitle("Chatbox")

	//users
	c.users = tview.NewList()
	c.users.SetBorder(true).SetTitle("Users")

	//flex for the page
	flex := tview.NewFlex().
		AddItem(c.roomtree, 20, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			//AddItem(tview.NewBox().SetBorder(true).SetTitle("Top"), 0, 1, false).
			AddItem(c.chat, 0, 3, false).
			AddItem(chatbox, 3, 1, false), 0, 2, false).
		AddItem(c.users, 20, 1, false)

	c.chat.SetTitleColor(tcell.ColorRed)

	//force a redraw when the textview is updated
	c.chat.SetChangedFunc(func() {
		c.app.Draw()
	})

	c.roomtree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var ret *tcell.EventKey
		ret = nil
		if event.Key() == tcell.KeyRight {
			c.roomtree.SetTitleColor(tcell.ColorWhite)
			c.chat.SetTitleColor(tcell.ColorRed)
			c.app.SetFocus(c.chat)
		} else {
			ret = event
		}
		c.checkIfMainMenu(event)
		return ret
	})

	c.chat.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var ret *tcell.EventKey
		ret = nil
		if event.Key() == tcell.KeyLeft {
			c.chat.SetTitleColor(tcell.ColorWhite)
			c.roomtree.SetTitleColor(tcell.ColorRed)
			c.app.SetFocus(c.roomtree)
			ret = event
		} else if event.Key() == tcell.KeyDown {
			c.chat.SetTitleColor(tcell.ColorWhite)
			chatbox.SetTitleColor(tcell.ColorRed)
			c.app.SetFocus(chatbox)
		} else if event.Key() == tcell.KeyRight {
			c.chat.SetTitleColor(tcell.ColorWhite)
			c.users.SetTitleColor(tcell.ColorRed)
			c.app.SetFocus(c.users)
			ret = event
		} else if event.Key() == tcell.KeyPgUp {
			ret = tcell.NewEventKey(tcell.KeyUp, event.Rune(), event.Modifiers())
		} else if event.Key() == tcell.KeyPgDn {
			ret = tcell.NewEventKey(tcell.KeyDown, event.Rune(), event.Modifiers())
		}
		c.checkIfMainMenu(event)
		return ret
	})

	c.users.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var ret *tcell.EventKey
		ret = nil
		if event.Key() == tcell.KeyLeft {
			c.users.SetTitleColor(tcell.ColorWhite)
			c.chat.SetTitleColor(tcell.ColorRed)
			c.app.SetFocus(c.chat)
		} else {
			ret = event
		}
		c.checkIfMainMenu(event)
		return ret
	})

	chatbox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var ret *tcell.EventKey
		ret = nil
		if event.Key() == tcell.KeyUp {
			chatbox.SetTitleColor(tcell.ColorWhite)
			c.chat.SetTitleColor(tcell.ColorRed)
			c.app.SetFocus(c.chat)
		} else if event.Key() == tcell.KeyEnter {
			//We want to send a message to the server on an enter
			err := c.sendMessage(chatbox.GetText(), c.curRoom, c.curChan)
			c.check(err)
			//chat.SetText(chat.GetText(true) + chatbox.GetText() + "\n")
			chatbox.SetText("")
		} else {
			ret = event
		}
		c.checkIfMainMenu(event)
		return ret
	})

	return flex
}

//packetListener - one go routine that listens on the socket and sends the appropriate object to the appropriate channel
func (c *Client) packetListener() {
	for {
		switch msg := c.proto.Decode().(type) {
		case proto.GetMessagesResponse:
			c.channels.getMessagesResponse <- msg
		case proto.GetRoomsResponse:
			c.channels.getRoomsResponse <- msg
		case proto.JoinRoomResponse:
			c.channels.joinRoomResponse <- msg
		case proto.RegisterResponse:
			c.channels.registerResponse <- msg
		case proto.CreateRoomResponse:
			c.channels.createRoomResponse <- msg
		case proto.PostMessageResponse:
			c.channels.postMessageResponse <- msg
		case proto.LoginResponse:
			c.channels.loginResponse <- msg
		case proto.DynamicMessage:
			c.channels.dynamicMessage <- msg
		default:
			log.Println("I don't know what I just got")
			log.Println(msg)
		}
	}
}

func main() {

	c := new(Client)
	//c.curRoom = 3
	//c.curChan = 1
	c.Init("localhost", 1200)

	register := c.registerPage()
	login := c.loginPage()
	main := c.mainPage()
	c.pages.AddPage("main", main, true, false)
	c.pages.AddPage("login", login, true, true)
	c.pages.AddPage("register", register, true, false)
	//create the main menu modal
	c.mainMenu()
	if err := c.app.SetRoot(c.pages, true).SetFocus(login).Run(); err != nil {
		panic(err)
	}
}
