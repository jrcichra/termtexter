package termtexterdb

import (
	"database/sql"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"

	//because I need the driver
	_ "github.com/go-sql-driver/mysql"
)

//User object
type User struct {
	UserID      int
	Username    string
	Displayname string
	Password    string
}

//DB is an object that will abstract the db stuff into nice methods
type DB struct {
	dbh *sql.DB
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

//Connect gets the user record from the database
func (d *DB) Connect(hostname string, password string) error {
	// connect to the database
	var err error
	d.dbh, err = sql.Open("mysql", "termtexter:"+password+"@tcp("+hostname+")/termtexter")
	return err
}

//GetUserIDFromKey - given a login session key, get the userID associated with it
func (d DB) GetUserIDFromKey(key string) (string, error) {
	rows, err := d.dbh.Query("select u.user_id from users u join sessions s on u.user_id = s.user_id where `key` = ?", key)
	check(err)
	defer rows.Close()
	rows.Next()
	var u string
	err = rows.Scan(&u)
	return u, err
}

//DoesRoomExist - returns if this room exists
func (d DB) DoesRoomExist(rid string) (bool, error) {
	rows, err := d.dbh.Query("select 1 from rooms where name = ?", rid)
	check(err)
	defer rows.Close()
	c := 0
	for rows.Next() {
		c++
	}
	var b bool
	if c > 0 {
		b = true
	} else {
		b = false
	}
	return b, err
}

//GetRooms -
func (d DB) GetRooms(uid string) ([]string, error) {
	rows, err := d.dbh.Query("select name from rooms r join room_users ru on r.room_id = ru.room_id join users u on u.user_id = ru.user_id where u.user_id = ?", uid)
	check(err)
	defer rows.Close()
	rooms := make([]string, 0)
	for rows.Next() {
		var temp string
		rows.Scan(&temp)
		rooms = append(rooms, temp)
	}
	return rooms, err
}

//AddUserToRoom - given an id, add this id into the mapping table
func (d DB) AddUserToRoom(uid string, rid string) error {
	res, err := d.dbh.Exec("insert into room_users (room_id,user_id) values (?,?)", rid, uid)
	fmt.Println(res)
	return err
}

//CreateRoom - Create a room, set user as admin, and build a default first channel
func (d DB) CreateRoom(rid string, uid string, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	//Handle this as a transaction since we're doing a few changes here
	t, err := d.dbh.Begin()
	defer t.Rollback()
	//Start by creating the room
	res, err := t.Exec("insert into rooms (name,password) values (?,?)", rid, hash)
	check(err)
	dbRoomID, err := res.LastInsertId()
	check(err)
	//Add the requesting user to this room as an admin
	res, err = t.Exec("insert into room_users (room_id,user_id,admin) values (?,?,?)", dbRoomID, uid, 1)
	check(err)
	//Create a channel for this room, with the name of "general"
	res, err = t.Exec("insert into channels (room_id,name) values (?,?)", dbRoomID, "general")
	check(err)
	fmt.Println(res)
	t.Commit()
	return err
}

//GetUserID gets the user id from the database if it exists
func (d DB) GetUserID(username string) (string, error) {
	rows, err := d.dbh.Query("select user_id from users where username = ?", username)
	check(err)
	defer rows.Close()
	rows.Next()
	var u string
	err = rows.Scan(&u)
	return u, err
}

//GetUser gets the user record from the database
func (d DB) GetUser(username string) (User, error) {
	rows, err := d.dbh.Query("select user_id, username, displayname, password from users where username = ?", username)
	check(err)
	defer rows.Close()
	rows.Next()
	var u User
	err = rows.Scan(&u.UserID, &u.Username, &u.Displayname, &u.Password)
	return u, err
}

//Register - Register's a new user
func (d DB) Register(username string, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	res, err := d.dbh.Exec("insert into users (username,password) values (?,?)", username, string(hash))
	fmt.Println(res)
	return err
}

//UserExists will return a bool if the user is registered
func (d DB) UserExists(username string) (bool, error) {
	rows, err := d.dbh.Query("select 1 from users where username = ?", username)
	check(err)
	defer rows.Close()
	c := 0
	for rows.Next() {
		c++
	}
	var b bool
	if c > 0 {
		b = true
	} else {
		b = false
	}
	return b, err
}

//IsValidLogin will determine if the login was valid. Pass in a plain text password
func (d DB) IsValidLogin(uid string, password string) bool {
	rows, err := d.dbh.Query("select password from users where user_id = ?", uid)
	check(err)
	defer rows.Close()
	rows.Next()
	var epassword string
	err = rows.Scan(&epassword)
	check(err)
	res := bcrypt.CompareHashAndPassword([]byte(epassword), []byte(password))
	if res == nil {
		return true
	} else {
		log.Println(res)
	}
	return false
}

//AddSession inserts the uuid we're handing to this client over to the user
func (d *DB) AddSession(uid, uuid string) error {
	res, err := d.dbh.Exec("insert into sessions (user_id,`key`) values (?,?)", uid, uuid)
	fmt.Println(res)
	return err
}
