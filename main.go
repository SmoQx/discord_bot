package main

import (
	"database/sql"
	"discord_bot/crud"
	"flag"
	"fmt"
)

func main() {
	runFlag := flag.Bool("bot", false, "Use this flag if you want bot only")
	runFlag2 := flag.Bool("server", false, "Use this if you want only the server for frontend")

	flag.Parse()

	db, err := sql.Open("sqlite3", "./database/data.db")
	if err != nil {
		fmt.Println(err)
	}
	defer db.Close()

	crud.InitDatabase(db)
	// crud.Test()

	if *runFlag {
		go RunServer(db)

		go MainBOT(db)
	}

	if *runFlag2 {
		RunFrontServer(db)
	}
}
