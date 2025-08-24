package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

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

	_, err := db.Exec(`INSERT INTO users (id, name) VALUES (?, ?)`, user_id, username)

	return err
}

func InsertSongIntoDatabase(song_id string, song_title string, db *sql.DB) error {
	if song_id == "" || song_title == "" {
		return fmt.Errorf("Username is empyt")
	}
	_, err := db.Exec(`INSERT INTO songs (id, title, played_counter) VALUES (?, ?, 0)`, song_id, song_title)

	return err
}

func UpdateSongsPlayCount(song_id string, db *sql.DB) {
	_, err := db.Exec("UPDATE songs SET played_counter = played_counter + 1 WHERE id = ?", song_id)
	if err != nil {
		fmt.Println("Update:", err)
	}
}
func ReadPlayedCountForSong(song_id string, db *sql.DB) *sql.Rows {
	rows, err := db.Query("SELECT song_title, played_counter FROM songs WHERE id = ?", song_id)
	if err != nil {
		fmt.Println("Select:", err)
	}
	defer rows.Close()

	return rows
}

func ReadAllUsers(db *sql.DB) *sql.Rows {
	rows, err := db.Query("SELECT id, name FROM users")
	if err != nil {
		fmt.Println("Select:", err)
	}
	defer rows.Close()

	return rows
}

func main() {
	// Open (or create) the SQLite database file
	db, err := sql.Open("sqlite3", "./example.db")
	if err != nil {
		fmt.Println(err)
	}
	defer db.Close()

	InitDatabase(db)

	InsertSongIntoDatabase("1", "first_song", db)
	InsertSongIntoDatabase("1", "first_song", db)
	InsertUserIntoDatabase("asdf", "1", db)
	InsertUserIntoDatabase("asdf", "1", db)

	rows := ReadAllUsers(db)

	fmt.Println("Users:")
	for rows.Next() {
		var id string
		var name string
		err = rows.Scan(&id, &name)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("ID=%s, Name=%s\n", id, name)
	}

	rows = ReadPlayedCountForSong("1", db)
	fmt.Println("Count for Song:")
	for rows.Next() {
		var song_title string
		var played_counter int
		err = rows.Scan(&song_title, &played_counter)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("song_title: %s, played_counter: %d\n", song_title, played_counter)
	}
}
