package crud

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	User_id  string
	Username string
}

type Song_counter struct {
	Id             string
	Title          string
	Server         string
	Played_counter int
}

func InitDatabase(db *sql.DB) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT
	);`)

	if err != nil {
		fmt.Println("failed to create table:", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS songs (
		id TEXT PRIMARY KEY,
		title TEXT,
		server TEXT,
		played_counter INTEGER
	);`)

	if err != nil {
		fmt.Println("failed to create table:", err)
	}
}

func InsertUserIntoDatabase(username string, user_id string, db *sql.DB) error {
	if username == "" {
		return fmt.Errorf("Username is empyt")
	}

	_, err := db.Exec(`INSERT INTO users (id, username) VALUES (?, ?)`, user_id, username)

	return err
}

func InsertSongIntoDatabase(song_id string, song_title string, server string, db *sql.DB) error {
	if song_id == "" || song_title == "" {
		return fmt.Errorf("Username is empty")
	}
	_, err := db.Exec(`INSERT INTO songs (id, title, played_counter, server) VALUES (?, ?, 0, ?)`, song_id, song_title, server)

	return err
}

func UpdateSongsPlayCount(song_id string, server string, db *sql.DB) {
	_, err := db.Exec("UPDATE songs SET played_counter = played_counter + 1 WHERE id = ? and server = ?", song_id, server)
	if err != nil {
		fmt.Println("Update:", err)
	}
}
func ReadPlayedCountForSong(song_id string, db *sql.DB) ([]Song_counter, error) {
	rows, err := db.Query("SELECT title, played_counter FROM songs WHERE id = ? ", song_id)
	if err != nil {
		fmt.Println("Select:", err)
		return nil, err
	}
	defer rows.Close()

	var songs []Song_counter
	for rows.Next() {
		var song Song_counter
		err = rows.Scan(&song.Title, &song.Played_counter)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("song_title: %s, played_counter: %d\n", song.Title, song.Played_counter)
	}
	return songs, nil
}

func ReadAllPlayedCountForSong(db *sql.DB) ([]Song_counter, error) {
	rows, err := db.Query("select s.title, s.played_counter  from songs s order by s.played_counter desc")
	if err != nil {
		fmt.Println("Select:", err)
		return nil, err
	}
	defer rows.Close()

	var songs []Song_counter
	for rows.Next() {
		var song Song_counter
		err = rows.Scan(&song.Title, &song.Played_counter)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("song_title: %s, played_counter: %d\n", song.Title, song.Played_counter)
		songs = append(songs, song)
	}
	return songs, nil
}

func ReadAllPlayedCountForSongInServer(server_name string, db *sql.DB) ([]Song_counter, error) {
	rows, err := db.Query("SELECT title, played_counter FROM songs WHERE server = ?", server_name)
	if err != nil {
		fmt.Println("Select:", err)
		return nil, err
	}
	defer rows.Close()

	var songs []Song_counter
	for rows.Next() {
		var song Song_counter
		err = rows.Scan(&song.Title, &song.Played_counter)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("song_title: %s, played_counter: %d\n", song.Title, song.Played_counter)
		songs = append(songs, song)
	}
	return songs, nil
}

func ReadAllUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query("SELECT id, username FROM users")
	if err != nil {
		fmt.Println("Select:", err)
		return nil, err
	}
	defer rows.Close()

	var users []User

	for rows.Next() {
		var user User
		err = rows.Scan(&user.User_id, &user.Username)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("ID=%s, Name=%s\n", user.User_id, user.Username)
		users = append(users, user)
	}

	return users, nil
}

func Test() {
	// Open (or create) the SQLite database file
	db, err := sql.Open("sqlite3", "./example.db")
	if err != nil {
		fmt.Println(err)
	}
	defer db.Close()

	InitDatabase(db)

	InsertSongIntoDatabase("1", "first_song", "1", db)
	InsertSongIntoDatabase("1", "first_song", "1", db)
	InsertSongIntoDatabase("2", "second", "1", db)
	InsertSongIntoDatabase("3", "third", "1", db)
	InsertUserIntoDatabase("asdf", "1", db)
	InsertUserIntoDatabase("asdf", "1", db)
	InsertUserIntoDatabase("afdfds", "2", db)
	UpdateSongsPlayCount("1", "1", db)

	fmt.Println("Users:")
	_, err = ReadAllUsers(db)

	fmt.Println("Songs:")
	_, err = ReadAllPlayedCountForSong(db)

	fmt.Println("Count for Song:")
	_, err = ReadPlayedCountForSong("1", db)
}
