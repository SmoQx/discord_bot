package main

import (
	"database/sql"
	"discord_bot/crud"
	"fmt"
)

func main() {
	db, err := sql.Open("sqlite3", "./database/data.db")
	if err != nil {
		fmt.Println(err)
	}
	defer db.Close()

	crud.InitDatabase(db)

	// crud.Test()
	MainBOT(db)
}
