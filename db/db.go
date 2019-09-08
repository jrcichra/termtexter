package termtexterdb

import (
	"database/sql"
	"fmt"

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

func getHashedPassword(in string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(in), bcrypt.MinCost)
	return string(hash), err
}

//Connect gets the user record from the database
func (d *DB) Connect(hostname string, password string) error {
	// connect to the database
	var err error
	d.dbh, err = sql.Open("mysql", "termtexter:"+password+"@tcp("+hostname+")/termtexter")
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

//UserExists will return a bool if the user is registered
func (d DB) UserExists(username string) (bool, error) {
	rows, err := d.dbh.Query("select user_id, username, displayname, password from users where username = ?", username)
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
	res := bcrypt.CompareHashAndPassword([]byte(password), []byte(password))
	if res == nil {
		return true
	}
	return false
}

//AddClient inserts the uuid we're handing to this client over to the user
func (d *DB) AddClient(uid, uuid string) error {
	res, err := d.dbh.Exec("insert into clients (user_id,key) values (?,?)", uid, uuid)
	fmt.Println(res)
	return err
}
